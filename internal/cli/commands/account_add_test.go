package commands

import (
	"context"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/cli/forms"
)

func TestRunAccountAddDelegatesToAppService(t *testing.T) {
	registry := newTestRegistry()
	prompter := &stubAccountAddPrompter{
		selectedService: &stubService{},
		accountResult: forms.AccountFieldsData{
			Name:     "Work",
			Settings: map[string]string{"client_id": "client-id"},
			Secrets:  map[string]string{"client_secret": "client-secret"},
		},
	}
	stdout := &strings.Builder{}
	cmd := newTestCommand()
	cmd.SetOut(stdout)

	called := false
	err := runAccountAdd(cmd, accountAddDeps{
		registry: registry,
		manager: &fakeAccountAddManager{
			listAccounts: []calendar.Account{},
			addAccountFunc: func(ctx context.Context, input app.AddAccountInput) (calendar.Account, error) {
				called = true
				if input.Name != "Work" || input.Settings["client_id"] != "client-id" || input.Secrets["client_secret"] != "client-secret" {
					t.Fatalf("unexpected input: %+v", input)
				}
				_, err := input.CalendarSelector.SelectCalendars(ctx, calendar.Account{Name: "Work"}, []calendar.Calendar{{ID: "primary", Name: "Primary", Primary: true}})
				if err != nil {
					t.Fatalf("SelectCalendars() error = %v", err)
				}
				return calendar.Account{Name: "Work"}, nil
			},
		},
		prompter: prompter,
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
	registry := newTestRegistry()
	err := runAccountAdd(newTestCommand(), accountAddDeps{
		registry: registry,
		manager: &fakeAccountAddManager{
			listAccounts: []calendar.Account{},
		},
		prompter: &stubAccountAddPrompter{selectServiceErr: huh.ErrUserAborted},
	})
	if err != nil {
		t.Fatalf("runAccountAdd() error = %v, want nil", err)
	}
}

func TestAccountAddFormRejectsDuplicateAccountName(t *testing.T) {
	accounts := []calendar.Account{{ID: "a", Service: calendar.ServiceTypeGoogle, Name: "Work"}}
	fields := []calendar.AccountField{
		{Key: "client_id", Label: "OAuth Client ID", Required: true},
		{Key: "client_secret", Label: "OAuth Client Secret", Required: true, Secret: true},
	}

	out := &strings.Builder{}
	form, output := forms.NewAccountFieldsForm(fields, forms.AccountFieldsData{}, func(name string) error {
		return validateNewAccountName(accounts, name)
	})
	form = form.WithAccessible(true).WithInput(strings.NewReader("Work\nPersonal\nclient-id\nclient-secret\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	result := output()
	if result.Name != "Personal" {
		t.Fatalf("result.Name = %q, want Personal", result.Name)
	}
}

type stubAccountAddPrompter struct {
	selectedService             calendar.Service
	selectServiceErr            error
	accountResult               forms.AccountFieldsData
	accountErr                  error
	selectedCalendars           []calendar.CalendarRef
	calendarSelectionErr        error
	showNoCalendarsErr          error
	noCalendarsShown            bool
	promptedCalendarAccountName string
}

func (s *stubAccountAddPrompter) SelectService(context.Context, []calendar.Service) (calendar.Service, error) {
	if s.selectServiceErr != nil {
		return nil, s.selectServiceErr
	}
	return s.selectedService, nil
}

func (s *stubAccountAddPrompter) PromptAccountFields(context.Context, []calendar.AccountField, forms.AccountFieldsData, func(string) error) (forms.AccountFieldsData, error) {
	if s.accountErr != nil {
		return forms.AccountFieldsData{}, s.accountErr
	}
	return s.accountResult, nil
}

func (s *stubAccountAddPrompter) SelectCalendars(_ context.Context, accountName string, _ []calendar.Calendar, _ []string) ([]calendar.CalendarRef, error) {
	s.promptedCalendarAccountName = accountName
	if s.calendarSelectionErr != nil {
		return nil, s.calendarSelectionErr
	}
	return s.selectedCalendars, nil
}

func (s *stubAccountAddPrompter) ConfirmEmptyCalendars(context.Context, string) error {
	s.noCalendarsShown = true
	return s.showNoCalendarsErr
}

type fakeAccountAddManager struct {
	listAccounts   []calendar.Account
	listErr        error
	addAccountFunc func(context.Context, app.AddAccountInput) (calendar.Account, error)
}

func (f *fakeAccountAddManager) ListAccounts() ([]calendar.Account, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listAccounts, nil
}

func (f *fakeAccountAddManager) AddAccount(ctx context.Context, input app.AddAccountInput) (calendar.Account, error) {
	if f.addAccountFunc != nil {
		return f.addAccountFunc(ctx, input)
	}
	return calendar.Account{}, nil
}
