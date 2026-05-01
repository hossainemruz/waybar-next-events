package cmd

import (
	"context"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/cli/forms"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"github.com/spf13/cobra"
)

func TestRunAccountUpdateDelegatesToAppService(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "old-client"}, Calendars: []appconfig.CalendarRef{{ID: "old", Name: "Old"}}}})
	loader := appconfig.NewLoaderWithPath(configPath)
	registry := newAppRegistry()
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", "client_secret", "old-secret")
	prompter := &stubAccountUpdatePrompter{
		selectedAccountID: "work-id",
		accountResult: forms.AccountFieldsResult{
			Name:     "Work Updated",
			Settings: map[string]string{"client_id": "new-client"},
			Secrets:  map[string]string{"client_secret": "new-secret"},
		},
		selectedCalendars: []calendar.CalendarRef{{ID: "new", Name: "New"}},
	}
	stdout := &strings.Builder{}
	cmd := newTestCommand()
	cmd.SetOut(stdout)

	called := false
	err := runAccountUpdate(cmd, accountUpdateDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newRegistry:    func() *calendar.Registry { return registry },
		newPrompter:    func(*cobra.Command) accountUpdatePrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		updateAccount: func(ctx context.Context, input app.UpdateAccountInput) (calendar.Account, error) {
			called = true
			if input.AccountID != "work-id" || input.Name != "Work Updated" || input.Settings["client_id"] != "new-client" || input.Secrets["client_secret"] != "new-secret" {
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
	if len(prompter.preselectedCalendarIDs) != 1 || prompter.preselectedCalendarIDs[0] != "old" {
		t.Fatalf("preselectedCalendarIDs = %+v, want [old]", prompter.preselectedCalendarIDs)
	}
}

func TestRunAccountUpdateReturnsNilOnSelectionAbort(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}))
	registry := newAppRegistry()
	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:   func() *appconfig.Loader { return loader },
		newRegistry: func() *calendar.Registry { return registry },
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
	fields := []calendar.AccountField{
		{Key: "client_id", Label: "OAuth Client ID", Required: true},
		{Key: "client_secret", Label: "OAuth Client Secret", Required: true, Secret: true},
	}
	defaults := forms.AccountFieldsInput{
		Name:     "Work",
		Settings: map[string]string{"client_id": "work-client"},
		Secrets:  map[string]string{"client_secret": "work-secret"},
	}

	var result forms.AccountFieldsResult
	out := &strings.Builder{}
	form, commit := forms.NewAccountFieldsForm(fields, defaults, &result, func(name string) error {
		return validateUpdatedAccountName(cfg, "Work", name)
	})
	form = form.WithAccessible(true).WithInput(strings.NewReader("Personal\nWork Updated\nwork-client\nwork-secret\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	commit()
	if result.Name != "Work Updated" {
		t.Fatalf("result.Name = %q, want Work Updated", result.Name)
	}
	if err := validateUpdatedAccountName(cfg, "Work", "Work"); err != nil {
		t.Fatalf("validateUpdatedAccountName() error = %v", err)
	}
}

type stubAccountUpdatePrompter struct {
	selectedAccountID      string
	selectionErr           error
	accountResult          forms.AccountFieldsResult
	accountErr             error
	preselectedCalendarIDs []string
	selectedCalendars      []calendar.CalendarRef
	calendarSelectionErr   error
	showNoCalendarsErr     error
	noCalendarsShown       bool
}

func (s *stubAccountUpdatePrompter) SelectAccount(context.Context, []calendar.Account, string) (string, error) {
	if s.selectionErr != nil {
		return "", s.selectionErr
	}
	return s.selectedAccountID, nil
}

func (s *stubAccountUpdatePrompter) PromptAccountFields(context.Context, []calendar.AccountField, forms.AccountFieldsInput, func(string) error) (forms.AccountFieldsResult, error) {
	if s.accountErr != nil {
		return forms.AccountFieldsResult{}, s.accountErr
	}
	return s.accountResult, nil
}

func (s *stubAccountUpdatePrompter) SelectCalendars(_ context.Context, _ string, _ []calendar.Calendar, preselected []string) ([]calendar.CalendarRef, error) {
	s.preselectedCalendarIDs = append([]string(nil), preselected...)
	if s.calendarSelectionErr != nil {
		return nil, s.calendarSelectionErr
	}
	return s.selectedCalendars, nil
}

func (s *stubAccountUpdatePrompter) ConfirmEmptyCalendars(context.Context, string) error {
	s.noCalendarsShown = true
	return s.showNoCalendarsErr
}
