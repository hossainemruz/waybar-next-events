package cmd

import (
	"context"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/pkg/calendars"
	"github.com/spf13/cobra"
)

func TestRunAccountAddDelegatesToAppService(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeGenericConfig(t, nil))
	prompter := &stubAccountAddPrompter{accountInput: accountAddInput{Name: "Work", ClientID: "client-id", ClientSecret: "client-secret"}}
	stdout := &strings.Builder{}
	cmd := newTestCommand()
	cmd.SetOut(stdout)

	called := false
	err := runAccountAdd(cmd, accountAddDependencies{
		newLoader:   func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountAddPrompter { return prompter },
		addAccount: func(ctx context.Context, input app.AddAccountInput) (calendar.Account, error) {
			called = true
			if input.Name != "Work" || input.Settings["client_id"] != "client-id" || input.Secrets[googleClientSecretKey] != "client-secret" {
				t.Fatalf("unexpected input: %+v", input)
			}
			_, err := input.CalendarSelector.SelectCalendars(ctx, calendar.Account{Name: "Work"}, []calendar.Calendar{{ID: "primary", Name: "Primary", Primary: true}})
			if err != nil {
				t.Fatalf("SelectCalendars() error = %v", err)
			}
			return calendar.Account{Name: "Work"}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountAdd() error = %v", err)
	}
	if !called {
		t.Fatal("expected addAccount to be called")
	}
	if !strings.Contains(stdout.String(), `Added account "Work".`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if prompter.promptedCalendarAccountName != "Work" {
		t.Fatalf("promptedCalendarAccountName = %q, want Work", prompter.promptedCalendarAccountName)
	}
}

func TestRunAccountAddReturnsNilOnUserAbort(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeGenericConfig(t, nil))
	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader: func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountAddPrompter {
			return &stubAccountAddPrompter{accountErr: huh.ErrUserAborted}
		},
		addAccount: func(context.Context, app.AddAccountInput) (calendar.Account, error) {
			t.Fatal("addAccount should not be called")
			return calendar.Account{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountAdd() error = %v, want nil", err)
	}
}

func TestAccountAddFormRejectsDuplicateAccountName(t *testing.T) {
	cfg := &appconfig.Config{Accounts: []appconfig.Account{{ID: "a", Service: calendar.ServiceTypeGoogle, Name: "Work"}}}
	var input accountAddInput
	out := &strings.Builder{}
	form := newAccountDetailsForm(&input, cfg).WithAccessible(true).WithInput(strings.NewReader("Work\nPersonal\nclient-id\nclient-secret\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	if input.Name != "Personal" {
		t.Fatalf("input.Name = %q, want Personal", input.Name)
	}
}

type stubAccountAddPrompter struct {
	accountInput                accountAddInput
	accountErr                  error
	selectedCalendars           []appconfig.CalendarRef
	calendarSelectionErr        error
	showNoCalendarsErr          error
	noCalendarsShown            bool
	promptedCalendarAccountName string
}

func (s *stubAccountAddPrompter) PromptAccountDetails(context.Context, *appconfig.Config) (accountAddInput, error) {
	if s.accountErr != nil {
		return accountAddInput{}, s.accountErr
	}
	return s.accountInput, nil
}

func (s *stubAccountAddPrompter) PromptCalendarSelection(_ context.Context, accountName string, _ []calendars.DiscoveredCalendar) ([]appconfig.CalendarRef, error) {
	s.promptedCalendarAccountName = accountName
	if s.calendarSelectionErr != nil {
		return nil, s.calendarSelectionErr
	}
	return s.selectedCalendars, nil
}

func (s *stubAccountAddPrompter) ShowNoCalendarsFound(context.Context, string) error {
	s.noCalendarsShown = true
	return s.showNoCalendarsErr
}
