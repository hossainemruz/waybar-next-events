package cmd

import (
	"context"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"github.com/hossainemruz/waybar-next-events/pkg/calendars"
	"github.com/spf13/cobra"
)

func TestRunAccountUpdateDelegatesToAppService(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "old-client"}, Calendars: []appconfig.CalendarRef{{ID: "old", Name: "Old"}}}})
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "old-secret")
	prompter := &stubAccountUpdatePrompter{selectedAccountID: "work-id", updatedInput: accountUpdateInput{Name: "Work Updated", ClientID: "new-client", ClientSecret: "new-secret"}, selectedCalendars: []appconfig.CalendarRef{{ID: "new", Name: "New"}}}
	stdout := &strings.Builder{}
	cmd := newTestCommand()
	cmd.SetOut(stdout)

	called := false
	err := runAccountUpdate(cmd, accountUpdateDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountUpdatePrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		updateAccount: func(ctx context.Context, input app.UpdateAccountInput) (calendar.Account, error) {
			called = true
			if input.AccountID != "work-id" || input.Name != "Work Updated" || input.Settings["client_id"] != "new-client" || input.Secrets[googleClientSecretKey] != "new-secret" {
				t.Fatalf("unexpected input: %+v", input)
			}
			_, err := input.CalendarSelector.SelectCalendars(ctx, calendar.Account{Name: "Work Updated"}, []calendar.Calendar{{ID: "new", Name: "New"}})
			if err != nil {
				t.Fatalf("SelectCalendars() error = %v", err)
			}
			return calendar.Account{Name: "Work Updated"}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountUpdate() error = %v", err)
	}
	if !called {
		t.Fatal("expected updateAccount to be called")
	}
	if !strings.Contains(stdout.String(), `Updated account "Work Updated".`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if prompter.detailsInput.ClientSecret != "old-secret" {
		t.Fatalf("detailsInput.ClientSecret = %q, want old-secret", prompter.detailsInput.ClientSecret)
	}
	if len(prompter.preselectedCalendarIDs) != 1 || prompter.preselectedCalendarIDs[0] != "old" {
		t.Fatalf("preselectedCalendarIDs = %+v, want [old]", prompter.preselectedCalendarIDs)
	}
}

func TestRunAccountUpdateReturnsNilOnSelectionAbort(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}))
	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader: func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountUpdatePrompter {
			return &stubAccountUpdatePrompter{selectionErr: huh.ErrUserAborted}
		},
		newSecretStore: func() secrets.Store { return secrets.NewInMemoryStore() },
		updateAccount: func(context.Context, app.UpdateAccountInput) (calendar.Account, error) {
			t.Fatal("updateAccount should not be called")
			return calendar.Account{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountUpdate() error = %v, want nil", err)
	}
}

func TestUpdateAccountFormRejectsDuplicateRenameButAllowsCurrentName(t *testing.T) {
	cfg := &appconfig.Config{Accounts: []appconfig.Account{{ID: "a", Service: calendar.ServiceTypeGoogle, Name: "Work"}, {ID: "b", Service: calendar.ServiceTypeGoogle, Name: "Personal"}}}
	input := accountUpdateInput{Name: "Work", ClientID: "work-client", ClientSecret: "work-secret"}
	out := &strings.Builder{}
	form := newUpdateAccountDetailsForm(&input, cfg).WithAccessible(true).WithInput(strings.NewReader("Personal\nWork Updated\nwork-client\nwork-secret\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	if input.Name != "Work Updated" {
		t.Fatalf("input.Name = %q, want Work Updated", input.Name)
	}
	if err := validateUpdatedAccountName(cfg, "Work", "Work"); err != nil {
		t.Fatalf("validateUpdatedAccountName() error = %v", err)
	}
}

func TestCloneCalendarsReturnsEmptySlice(t *testing.T) {
	cloned := cloneCalendars(nil)
	if cloned == nil || len(cloned) != 0 {
		t.Fatalf("cloneCalendars(nil) = %+v, want empty slice", cloned)
	}
}

type stubAccountUpdatePrompter struct {
	selectedAccountID      string
	selectionErr           error
	updatedInput           accountUpdateInput
	detailsInput           accountUpdateInput
	detailsErr             error
	preselectedCalendarIDs []string
	selectedCalendars      []appconfig.CalendarRef
	calendarSelectionErr   error
	showNoCalendarsErr     error
	noCalendarsShown       bool
}

func (s *stubAccountUpdatePrompter) PromptAccountSelection(context.Context, *appconfig.Config) (string, error) {
	if s.selectionErr != nil {
		return "", s.selectionErr
	}
	return s.selectedAccountID, nil
}

func (s *stubAccountUpdatePrompter) PromptAccountDetails(_ context.Context, _ *appconfig.Config, input accountUpdateInput) (accountUpdateInput, error) {
	s.detailsInput = input
	if s.detailsErr != nil {
		return accountUpdateInput{}, s.detailsErr
	}
	return s.updatedInput, nil
}

func (s *stubAccountUpdatePrompter) PromptCalendarSelection(_ context.Context, _ string, _ []calendars.DiscoveredCalendar, selectedCalendarIDs []string) ([]appconfig.CalendarRef, error) {
	s.preselectedCalendarIDs = append([]string(nil), selectedCalendarIDs...)
	if s.calendarSelectionErr != nil {
		return nil, s.calendarSelectionErr
	}
	return s.selectedCalendars, nil
}

func (s *stubAccountUpdatePrompter) ShowNoCalendarsFound(context.Context, string) error {
	s.noCalendarsShown = true
	return s.showNoCalendarsErr
}
