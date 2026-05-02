package commands

import (
	"context"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/cli/forms"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
)

func TestRunAccountUpdateDelegatesToAppService(t *testing.T) {
	registry := newTestRegistry()
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", "client_secret", "old-secret")
	prompter := &stubAccountUpdatePrompter{
		selectedAccountID: "work-id",
		accountResult: forms.AccountFieldsData{
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
	err := runAccountUpdate(cmd, accountUpdateDeps{
		registry:    registry,
		secretStore: secretStore,
		manager: &fakeAccountUpdateManager{
			fakeBaseManager: fakeBaseManager{listAccounts: []calendar.Account{
				{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "old-client"}, Calendars: []calendar.CalendarRef{{ID: "old", Name: "Old"}}},
			}},
			updateAccountFunc: func(ctx context.Context, input app.UpdateAccountInput) (calendar.Account, error) {
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
		},
		prompter: prompter,
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

func TestRunAccountUpdatePreservesUnknownSettings(t *testing.T) {
	registry := newTestRegistry()
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", "client_secret", "old-secret")
	prompter := &stubAccountUpdatePrompter{
		selectedAccountID: "work-id",
		accountResult: forms.AccountFieldsData{
			Name:     "Work",
			Settings: map[string]string{"client_id": "new-client"},
			Secrets:  map[string]string{"client_secret": "new-secret"},
		},
		selectedCalendars: []calendar.CalendarRef{{ID: "old", Name: "Old"}},
	}

	var receivedSettings map[string]string
	err := runAccountUpdate(newTestCommand(), accountUpdateDeps{
		registry:    registry,
		secretStore: secretStore,
		manager: &fakeAccountUpdateManager{
			fakeBaseManager: fakeBaseManager{listAccounts: []calendar.Account{{
				ID:        "work-id",
				Service:   calendar.ServiceTypeGoogle,
				Name:      "Work",
				Settings:  map[string]string{"client_id": "old-client", "region": "us-east"},
				Calendars: []calendar.CalendarRef{{ID: "old", Name: "Old"}},
			}}},
			updateAccountFunc: func(_ context.Context, input app.UpdateAccountInput) (calendar.Account, error) {
				receivedSettings = input.Settings
				return calendar.Account{Name: "Work"}, nil
			},
		},
		prompter: prompter,
	})
	if err != nil {
		t.Fatalf("runAccountUpdate() error = %v", err)
	}
	if receivedSettings == nil {
		t.Fatal("expected updateAccount to be called")
	}
	if receivedSettings["client_id"] != "new-client" {
		t.Fatalf("Settings[client_id] = %q, want new-client", receivedSettings["client_id"])
	}
	if receivedSettings["region"] != "us-east" {
		t.Fatalf("Settings[region] = %q, want us-east; unknown settings were dropped", receivedSettings["region"])
	}
}

func TestRunAccountUpdateReturnsNilOnSelectionAbort(t *testing.T) {
	registry := newTestRegistry()
	err := runAccountUpdate(newTestCommand(), accountUpdateDeps{
		registry: registry,
		manager: &fakeAccountUpdateManager{
			fakeBaseManager: fakeBaseManager{listAccounts: []calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}},
		},
		prompter: &stubAccountUpdatePrompter{selectionErr: huh.ErrUserAborted},
	})
	if err != nil {
		t.Fatalf("runAccountUpdate() error = %v, want nil", err)
	}
}

func TestUpdateAccountFormRejectsDuplicateRenameButAllowsCurrentName(t *testing.T) {
	accounts := []calendar.Account{
		{ID: "a", Service: calendar.ServiceTypeGoogle, Name: "Work"},
		{ID: "b", Service: calendar.ServiceTypeGoogle, Name: "Personal"},
	}
	fields := []calendar.AccountField{
		{Key: "client_id", Label: "OAuth Client ID", Required: true},
		{Key: "client_secret", Label: "OAuth Client Secret", Required: true, Secret: true},
	}
	defaults := forms.AccountFieldsData{
		Name:     "Work",
		Settings: map[string]string{"client_id": "work-client"},
		Secrets:  map[string]string{"client_secret": "work-secret"},
	}

	out := &strings.Builder{}
	form, output := forms.NewAccountFieldsForm(fields, defaults, func(name string) error {
		return validateUpdatedAccountName(accounts, "Work", name)
	})
	form = form.WithAccessible(true).WithInput(strings.NewReader("Personal\nWork Updated\nwork-client\nwork-secret\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	result := output()
	if result.Name != "Work Updated" {
		t.Fatalf("result.Name = %q, want Work Updated", result.Name)
	}
	if err := validateUpdatedAccountName(accounts, "Work", "Work"); err != nil {
		t.Fatalf("validateUpdatedAccountName() error = %v", err)
	}
}

type stubAccountUpdatePrompter struct {
	selectedAccountID      string
	selectionErr           error
	accountResult          forms.AccountFieldsData
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

func (s *stubAccountUpdatePrompter) PromptAccountFields(context.Context, []calendar.AccountField, forms.AccountFieldsData, func(string) error) (forms.AccountFieldsData, error) {
	if s.accountErr != nil {
		return forms.AccountFieldsData{}, s.accountErr
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

type fakeAccountUpdateManager struct {
	fakeBaseManager
	updateAccountFunc func(context.Context, app.UpdateAccountInput) (calendar.Account, error)
}

func (f *fakeAccountUpdateManager) UpdateAccount(ctx context.Context, input app.UpdateAccountInput) (calendar.Account, error) {
	if f.updateAccountFunc != nil {
		return f.updateAccountFunc(ctx, input)
	}
	return calendar.Account{}, nil
}
