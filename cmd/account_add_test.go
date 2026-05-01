package cmd

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/cli/forms"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/spf13/cobra"
)

func TestRunAccountAddDelegatesToAppService(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeGenericConfig(t, nil))
	registry := newAppRegistry()
	prompter := &stubAccountAddPrompter{
		selectedService: &stubService{},
		accountResult: forms.AccountFieldsResult{
			Name:     "Work",
			Settings: map[string]string{"client_id": "client-id"},
			Secrets:  map[string]string{"client_secret": "client-secret"},
		},
	}
	stdout := &strings.Builder{}
	cmd := newTestCommand()
	cmd.SetOut(stdout)

	called := false
	err := runAccountAdd(cmd, accountAddDependencies{
		newLoader:   func() *appconfig.Loader { return loader },
		newRegistry: func() *calendar.Registry { return registry },
		newPrompter: func(*cobra.Command) accountAddPrompter { return prompter },
		addAccount: func(ctx context.Context, input app.AddAccountInput) (calendar.Account, error) {
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
	registry := newAppRegistry()
	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:   func() *appconfig.Loader { return loader },
		newRegistry: func() *calendar.Registry { return registry },
		newPrompter: func(*cobra.Command) accountAddPrompter {
			return &stubAccountAddPrompter{selectServiceErr: huh.ErrUserAborted}
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
	fields := []calendar.AccountField{
		{Key: "client_id", Label: "OAuth Client ID", Required: true},
		{Key: "client_secret", Label: "OAuth Client Secret", Required: true, Secret: true},
	}

	var result forms.AccountFieldsResult
	out := &strings.Builder{}
	form, commit := forms.NewAccountFieldsForm(fields, forms.AccountFieldsInput{}, &result, func(name string) error {
		return validateNewAccountName(cfg, name)
	})
	form = form.WithAccessible(true).WithInput(strings.NewReader("Work\nPersonal\nclient-id\nclient-secret\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	commit()
	if result.Name != "Personal" {
		t.Fatalf("result.Name = %q, want Personal", result.Name)
	}
}

type stubAccountAddPrompter struct {
	selectedService             calendar.Service
	selectServiceErr            error
	accountResult               forms.AccountFieldsResult
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

func (s *stubAccountAddPrompter) PromptAccountFields(context.Context, []calendar.AccountField, forms.AccountFieldsInput, func(string) error) (forms.AccountFieldsResult, error) {
	if s.accountErr != nil {
		return forms.AccountFieldsResult{}, s.accountErr
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

type stubService struct{}

func (s *stubService) Type() calendar.ServiceType { return calendar.ServiceTypeGoogle }
func (s *stubService) DisplayName() string        { return "Google" }
func (s *stubService) AccountFields() []calendar.AccountField {
	return []calendar.AccountField{
		{Key: "client_id", Label: "OAuth Client ID", Required: true},
		{Key: "client_secret", Label: "OAuth Client Secret", Required: true, Secret: true},
	}
}
func (s *stubService) DiscoverCalendars(context.Context, calendar.Account, *http.Client) ([]calendar.Calendar, error) {
	return nil, nil
}
func (s *stubService) FetchEvents(context.Context, calendar.Account, calendar.EventQuery, *http.Client) ([]calendar.Event, error) {
	return nil, nil
}
