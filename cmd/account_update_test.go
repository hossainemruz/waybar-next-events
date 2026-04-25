package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/pkg/auth"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/pkg/calendars"
)

func TestRunAccountUpdateUpdatesSelectedAccountAndPersistsCalendars(t *testing.T) {
	configPath := writeTestConfigFile(t, `{
		"google": {
			"name": "Google Calendar",
			"accounts": [
				{
					"name": "Work",
					"clientId": "work-client",
					"clientSecret": "work-secret",
					"calendars": [
						{"name": "Primary", "id": "primary-id"},
						{"name": "Team", "id": "team-id"}
					]
				},
				{
					"name": "Personal",
					"clientId": "personal-client",
					"clientSecret": "personal-secret",
					"calendars": [
						{"name": "Home", "id": "home-id"}
					]
				}
			]
		}
	}`)

	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountUpdatePrompter{
		selectedAccountName: "Work",
		updatedInput: accountUpdateInput{
			Name:         "Work Updated",
			ClientID:     "work-client",
			ClientSecret: "work-secret",
		},
		selectedCalendars: []appconfig.Calendar{{Name: "Team", ID: "team-id"}},
	}

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountUpdatePrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken: func(context.Context, *auth.Authenticator, *appconfig.GoogleAccount) error {
			return nil
		},
		discoverCalendars: func(ctx context.Context, account *appconfig.GoogleAccount, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{
				{Calendar: appconfig.Calendar{Name: "Primary", ID: "primary-id"}, Primary: true},
				{Calendar: appconfig.Calendar{Name: "Team", ID: "team-id"}},
				{Calendar: appconfig.Calendar{Name: "Focus", ID: "focus-id"}},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountUpdate() error = %v", err)
	}

	if prompter.detailsInput.Name != "Work" || prompter.detailsInput.ClientID != "work-client" || prompter.detailsInput.ClientSecret != "work-secret" {
		t.Fatalf("details input = %+v, want original account values", prompter.detailsInput)
	}
	if len(prompter.preselectedCalendarIDs) != 2 || prompter.preselectedCalendarIDs[0] != "primary-id" || prompter.preselectedCalendarIDs[1] != "team-id" {
		t.Fatalf("preselected calendar IDs = %v, want [primary-id team-id]", prompter.preselectedCalendarIDs)
	}

	loaded, err := loader.Load()
	if err != nil {
		t.Fatalf("loader.Load() error = %v", err)
	}

	googleCfg, err := loaded.GetGoogleConfig()
	if err != nil {
		t.Fatalf("loaded.GetGoogleConfig() error = %v", err)
	}

	if len(googleCfg.Accounts) != 2 {
		t.Fatalf("accounts length = %d, want 2", len(googleCfg.Accounts))
	}

	updated := googleCfg.Accounts[0]
	if updated.Name != "Work Updated" {
		t.Fatalf("updated.Name = %q, want %q", updated.Name, "Work Updated")
	}
	if updated.ClientID != "work-client" || updated.ClientSecret != "work-secret" {
		t.Fatalf("updated credentials = (%q, %q), want (%q, %q)", updated.ClientID, updated.ClientSecret, "work-client", "work-secret")
	}
	if len(updated.Calendars) != 1 || updated.Calendars[0].ID != "team-id" {
		t.Fatalf("updated calendars = %+v, want only team-id", updated.Calendars)
	}

	unchanged := googleCfg.Accounts[1]
	if unchanged.Name != "Personal" || unchanged.ClientID != "personal-client" || unchanged.ClientSecret != "personal-secret" {
		t.Fatalf("unchanged account = %+v, want original personal account", unchanged)
	}
}

func TestRunAccountUpdateClearsOldTokenBeforeReauthWhenCredentialsChange(t *testing.T) {
	configPath := writeTestConfigFile(t, `{
		"google": {
			"name": "Google Calendar",
			"accounts": [
				{
					"name": "Work",
					"clientId": "old-client",
					"clientSecret": "old-secret",
					"calendars": []
				}
			]
		}
	}`)

	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountUpdatePrompter{
		selectedAccountName: "Work",
		updatedInput: accountUpdateInput{
			Name:         "Work",
			ClientID:     "new-client",
			ClientSecret: "new-secret",
		},
		selectedCalendars: []appconfig.Calendar{},
	}

	clearCalled := false
	clearProviderName := ""
	discoverSawClear := false

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountUpdatePrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken: func(ctx context.Context, authenticator *auth.Authenticator, account *appconfig.GoogleAccount) error {
			clearCalled = true
			clearProviderName = account.ClientID
			return nil
		},
		discoverCalendars: func(ctx context.Context, account *appconfig.GoogleAccount, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			discoverSawClear = clearCalled
			return []calendars.DiscoveredCalendar{{Calendar: appconfig.Calendar{Name: "Primary", ID: "primary-id"}, Primary: true}}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountUpdate() error = %v", err)
	}

	if !clearCalled {
		t.Fatal("expected clearToken to be called when credentials change")
	}
	if !discoverSawClear {
		t.Fatal("expected clearToken to run before calendar discovery")
	}
	if clearProviderName != "old-client" {
		t.Fatalf("cleared provider = %q, want %q", clearProviderName, "old-client")
	}
}

func TestRunAccountUpdateRollsBackOnOAuthFailure(t *testing.T) {
	configPath := writeTestConfigFile(t, `{
		"google": {
			"name": "Google Calendar",
			"accounts": [
				{
					"name": "Work",
					"clientId": "old-client",
					"clientSecret": "old-secret",
					"calendars": [
						{"name": "Primary", "id": "primary-id"}
					]
				}
			]
		}
	}`)
	original, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountUpdatePrompter{
		selectedAccountName: "Work",
		updatedInput: accountUpdateInput{
			Name:         "Work Updated",
			ClientID:     "new-client",
			ClientSecret: "new-secret",
		},
	}
	backingStore := tokenstore.NewInMemoryTokenStore()
	if err := backingStore.Set(context.Background(), "old-client", &oauth2.Token{AccessToken: "old-token"}); err != nil {
		t.Fatalf("backingStore.Set() error = %v", err)
	}

	err = runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountUpdatePrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return backingStore },
		clearToken:    clearGoogleAccountToken,
		discoverCalendars: func(ctx context.Context, account *appconfig.GoogleAccount, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return nil, errors.New("oauth login failed")
		},
	})
	if err == nil {
		t.Fatal("runAccountUpdate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "oauth login failed") {
		t.Fatalf("error = %q, want oauth failure", err.Error())
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() after failure error = %v", err)
	}
	if string(after) != string(original) {
		t.Fatalf("config changed after OAuth failure\n got: %s\nwant: %s", string(after), string(original))
	}

	token, found, err := backingStore.Get(context.Background(), "old-client")
	if err != nil {
		t.Fatalf("backingStore.Get() error = %v", err)
	}
	if !found || token == nil || token.AccessToken != "old-token" {
		t.Fatalf("stored token after failure = %+v, found=%v, want original token", token, found)
	}
}

func TestRunAccountUpdateReturnsNoAccountsErrorWhenConfigMissing(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(filepath.Join(t.TempDir(), "missing-config.json"))

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountUpdatePrompter { return &stubAccountUpdatePrompter{} },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken: func(context.Context, *auth.Authenticator, *appconfig.GoogleAccount) error {
			return nil
		},
		discoverCalendars: func(context.Context, *appconfig.GoogleAccount, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return nil, nil
		},
	})
	if !errors.Is(err, appconfig.ErrNoAccounts) {
		t.Fatalf("runAccountUpdate() error = %v, want ErrNoAccounts", err)
	}
	if err.Error() != "no accounts configured: add an account first" {
		t.Fatalf("error = %q, want no-accounts hint", err.Error())
	}
}

func TestRunAccountUpdateReturnsMalformedConfigError(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeTestConfigFile(t, `{invalid json}`))

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountUpdatePrompter { return &stubAccountUpdatePrompter{} },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken: func(context.Context, *auth.Authenticator, *appconfig.GoogleAccount) error {
			return nil
		},
		discoverCalendars: func(context.Context, *appconfig.GoogleAccount, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("runAccountUpdate() error = nil, want error")
	}

	want := "failed to load config: failed to parse config file: invalid character 'i' looking for beginning of object key string"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestUpdateAccountFormRejectsDuplicateRenameButAllowsCurrentName(t *testing.T) {
	googleCfg := &appconfig.GoogleCalendar{
		Name: "Google Calendar",
		Accounts: []appconfig.GoogleAccount{
			{Name: "Work", ClientID: "work-client"},
			{Name: "Personal", ClientID: "personal-client"},
		},
	}

	var input = accountUpdateInput{
		Name:         "Work",
		ClientID:     "work-client",
		ClientSecret: "work-secret",
	}
	out := &strings.Builder{}
	form := newUpdateAccountDetailsForm(&input, googleCfg).
		WithAccessible(true).
		WithInput(strings.NewReader("Personal\nWork Updated\nwork-client\nwork-secret\n")).
		WithOutput(out)

	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}

	if input.Name != "Work Updated" {
		t.Fatalf("input.Name = %q, want %q", input.Name, "Work Updated")
	}
	if !strings.Contains(out.String(), `account name already exists: "Personal"`) {
		t.Fatalf("form output = %q, want duplicate-name validation message", out.String())
	}

	if err := validateUpdatedAccountName(googleCfg, "Work", "Work"); err != nil {
		t.Fatalf("validateUpdatedAccountName() error = %v, want nil for unchanged name", err)
	}
}

type stubAccountUpdatePrompter struct {
	selectedAccountName    string
	selectionErr           error
	updatedInput           accountUpdateInput
	detailsInput           accountUpdateInput
	detailsErr             error
	preselectedCalendarIDs []string
	selectedCalendars      []appconfig.Calendar
	calendarSelectionErr   error
	showNoCalendarsErr     error
	noCalendarsShown       bool
}

func (s *stubAccountUpdatePrompter) PromptAccountSelection(context.Context, *appconfig.GoogleCalendar) (string, error) {
	if s.selectionErr != nil {
		return "", s.selectionErr
	}
	return s.selectedAccountName, nil
}

func (s *stubAccountUpdatePrompter) PromptAccountDetails(_ context.Context, _ *appconfig.GoogleCalendar, input accountUpdateInput) (accountUpdateInput, error) {
	s.recordPromptAccountDetails(input)
	return s.promptAccountDetails()
}

func (s *stubAccountUpdatePrompter) promptAccountDetails() (accountUpdateInput, error) {
	if s.detailsErr != nil {
		return accountUpdateInput{}, s.detailsErr
	}
	return s.updatedInput, nil
}

func (s *stubAccountUpdatePrompter) recordPromptAccountDetails(input accountUpdateInput) {
	s.detailsInput = input
}

func (s *stubAccountUpdatePrompter) PromptCalendarSelection(_ context.Context, _ string, _ []calendars.DiscoveredCalendar, selectedCalendarIDs []string) ([]appconfig.Calendar, error) {
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

var _ accountUpdatePrompter = (*stubAccountUpdatePrompter)(nil)
