package cmd

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	appcalendar "github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/pkg/auth"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/pkg/calendars"
)

func TestRunAccountUpdateUpdatesSelectedAccount(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{
		{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client", "client_secret": "work-secret"}, Calendars: []appconfig.CalendarRef{{ID: "primary-id", Name: "Primary"}}},
		{ID: "personal-id", Service: appcalendar.ServiceTypeGoogle, Name: "Personal", Settings: map[string]string{"client_id": "personal-client", "client_secret": "personal-secret"}, Calendars: []appconfig.CalendarRef{{ID: "home-id", Name: "Home"}}},
	})
	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountUpdatePrompter{
		selectedAccountID: "personal-id",
		updatedInput:      accountUpdateInput{Name: "Personal Updated", ClientID: "personal-client", ClientSecret: "personal-secret"},
		selectedCalendars: []appconfig.CalendarRef{{Name: "Travel", ID: "travel-id"}},
	}

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountUpdatePrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken:    func(context.Context, *auth.Authenticator, *appconfig.Account) error { return nil },
		discoverCalendars: func(context.Context, *appconfig.Account, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{{Calendar: appconfig.CalendarRef{Name: "Travel", ID: "travel-id"}}}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountUpdate() error = %v", err)
	}

	loaded, _ := loader.Load()
	updated := loaded.FindAccountByID("personal-id")
	if updated == nil || updated.Name != "Personal Updated" {
		t.Fatalf("updated account = %+v, want renamed account", updated)
	}
	if len(updated.Calendars) != 1 || updated.Calendars[0].ID != "travel-id" {
		t.Fatalf("updated calendars = %+v, want travel-id", updated.Calendars)
	}
}

func TestRunAccountUpdatePreservesUnknownSettings(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{
		ID:      "work-id",
		Service: appcalendar.ServiceTypeGoogle,
		Name:    "Work",
		Settings: map[string]string{
			"client_id":     "work-client",
			"client_secret": "work-secret",
			"tenant_id":     "tenant-123",
		},
	}})
	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountUpdatePrompter{selectedAccountID: "work-id", updatedInput: accountUpdateInput{Name: "Work", ClientID: "updated-client", ClientSecret: "updated-secret"}}

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountUpdatePrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken:    func(context.Context, *auth.Authenticator, *appconfig.Account) error { return nil },
		discoverCalendars: func(context.Context, *appconfig.Account, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountUpdate() error = %v", err)
	}
	loaded, _ := loader.Load()
	updated := loaded.FindAccountByID("work-id")
	if updated.Setting("tenant_id") != "tenant-123" {
		t.Fatalf("tenant_id = %q, want preserved setting", updated.Setting("tenant_id"))
	}
}

func TestRunAccountUpdateClearsOldTokenWhenCredentialsChange(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "old-client", "client_secret": "old-secret"}}})
	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountUpdatePrompter{selectedAccountID: "work-id", updatedInput: accountUpdateInput{Name: "Work", ClientID: "new-client", ClientSecret: "new-secret"}}
	clearCalled := false

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountUpdatePrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken: func(context.Context, *auth.Authenticator, *appconfig.Account) error {
			clearCalled = true
			return nil
		},
		discoverCalendars: func(context.Context, *appconfig.Account, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountUpdate() error = %v", err)
	}
	if !clearCalled {
		t.Fatal("expected clearToken to be called")
	}
}

func TestRunAccountUpdateReturnsNoAccountsErrorWhenConfigMissing(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(filepath.Join(t.TempDir(), "missing.json"))
	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountUpdatePrompter { return &stubAccountUpdatePrompter{} },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken:    func(context.Context, *auth.Authenticator, *appconfig.Account) error { return nil },
		discoverCalendars: func(context.Context, *appconfig.Account, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return nil, nil
		},
	})
	if !errors.Is(err, appconfig.ErrNoAccounts) {
		t.Fatalf("runAccountUpdate() error = %v, want ErrNoAccounts", err)
	}
}

func TestRunAccountUpdatePreservesConfigOnAbort(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client", "client_secret": "work-secret"}}})
	original := readFile(t, configPath)
	loader := appconfig.NewLoaderWithPath(configPath)

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader: func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountUpdatePrompter {
			return &stubAccountUpdatePrompter{selectionErr: huh.ErrUserAborted}
		},
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken:    func(context.Context, *auth.Authenticator, *appconfig.Account) error { return nil },
		discoverCalendars: func(context.Context, *appconfig.Account, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountUpdate() error = %v, want nil", err)
	}
	assertConfigUnchanged(t, configPath, original)
}

func TestRunAccountUpdateRollsBackConfigWhenTokenCommitFails(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "old-client", "client_secret": "old-secret"}}})
	original := readFile(t, configPath)
	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountUpdatePrompter{selectedAccountID: "work-id", updatedInput: accountUpdateInput{Name: "Work Updated", ClientID: "new-client", ClientSecret: "new-secret"}}
	backingStore := &failingTokenStore{clearErr: errors.New("keyring unavailable")}

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountUpdatePrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return backingStore },
		clearToken:    clearAccountToken,
		discoverCalendars: func(context.Context, *appconfig.Account, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{{Calendar: appconfig.CalendarRef{Name: "Primary", ID: "primary-id"}}}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to persist OAuth token") {
		t.Fatalf("error = %v, want token persistence error", err)
	}
	assertConfigUnchanged(t, configPath, original)
}

func TestUpdateAccountFormRejectsDuplicateRenameButAllowsCurrentName(t *testing.T) {
	cfg := &appconfig.Config{Accounts: []appconfig.Account{{ID: "a", Service: appcalendar.ServiceTypeGoogle, Name: "Work"}, {ID: "b", Service: appcalendar.ServiceTypeGoogle, Name: "Personal"}}}
	var input = accountUpdateInput{Name: "Work", ClientID: "work-client", ClientSecret: "work-secret"}
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
