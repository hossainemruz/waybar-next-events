package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/pkg/auth"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/pkg/calendars"
)

func TestRunAccountAddCreatesConfigOnFirstRun(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "nested", "waybar-next-events", "config.json")
	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountAddPrompter{
		accountInput: accountAddInput{
			Name:         "Work",
			ClientID:     "client-id",
			ClientSecret: "client-secret",
		},
		selectedCalendars: []appconfig.Calendar{{Name: "Primary", ID: "primary-id"}},
	}
	backingStore := tokenstore.NewInMemoryTokenStore()

	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountAddPrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return backingStore },
		discoverCalendars: func(ctx context.Context, account *appconfig.GoogleAccount, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{{
				Calendar: appconfig.Calendar{Name: "Primary", ID: "primary-id"},
				Primary:  true,
			}}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountAdd() error = %v", err)
	}

	loaded, err := loader.Load()
	if err != nil {
		t.Fatalf("loader.Load() error = %v", err)
	}

	googleCfg, err := loaded.GetGoogleConfig()
	if err != nil {
		t.Fatalf("loaded.GetGoogleConfig() error = %v", err)
	}

	if len(googleCfg.Accounts) != 1 {
		t.Fatalf("accounts length = %d, want 1", len(googleCfg.Accounts))
	}

	account := googleCfg.Accounts[0]
	if account.Name != "Work" {
		t.Fatalf("account name = %q, want %q", account.Name, "Work")
	}
	if account.ClientID != "client-id" {
		t.Fatalf("account client ID = %q, want %q", account.ClientID, "client-id")
	}
	if len(account.Calendars) != 1 || account.Calendars[0].ID != "primary-id" {
		t.Fatalf("account calendars = %+v, want primary-id", account.Calendars)
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file stat error = %v", err)
	}

	if _, found, err := backingStore.Get(context.Background(), "client-id"); err != nil {
		t.Fatalf("backingStore.Get() error = %v", err)
	} else if found {
		t.Fatal("token store should remain unchanged when no token was staged")
	}
}

func TestRunAccountAddDoesNotSaveConfigWhenDiscoveryFails(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountAddPrompter{
		accountInput: accountAddInput{
			Name:         "Work",
			ClientID:     "client-id",
			ClientSecret: "client-secret",
		},
	}
	backingStore := tokenstore.NewInMemoryTokenStore()

	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountAddPrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return backingStore },
		discoverCalendars: func(ctx context.Context, account *appconfig.GoogleAccount, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return nil, errors.New("oauth login failed")
		},
	})
	if err == nil {
		t.Fatal("runAccountAdd() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "oauth login failed") {
		t.Fatalf("error = %q, want oauth failure", err.Error())
	}

	if _, statErr := os.Stat(configPath); !os.IsNotExist(statErr) {
		t.Fatalf("config file should not exist after failure, stat error = %v", statErr)
	}

	if _, found, err := backingStore.Get(context.Background(), "client-id"); err != nil {
		t.Fatalf("backingStore.Get() error = %v", err)
	} else if found {
		t.Fatal("token store should remain unchanged on failure")
	}
}

func TestRunAccountAddPreservesConfigOnUserAbort(t *testing.T) {
	configPath := writeTestConfigFile(t, `{
		"google": {
			"name": "Google Calendar",
			"accounts": [
				{
					"name": "Existing",
					"clientId": "existing-client",
					"clientSecret": "existing-secret",
					"calendars": []
				}
			]
		}
	}`)
	original, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountAddPrompter{
		accountInput: accountAddInput{
			Name:         "New",
			ClientID:     "new-client",
			ClientSecret: "new-secret",
		},
		calendarSelectionErr: huh.ErrUserAborted,
	}

	err = runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountAddPrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		discoverCalendars: func(ctx context.Context, account *appconfig.GoogleAccount, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{{
				Calendar: appconfig.Calendar{Name: "Primary", ID: "primary-id"},
				Primary:  true,
			}}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountAdd() error = %v, want nil", err)
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() after abort error = %v", err)
	}
	if string(after) != string(original) {
		t.Fatalf("config changed after user abort\n got: %s\nwant: %s", string(after), string(original))
	}
}

func TestRunAccountAddPreservesConfigWhenNoCalendarsPromptIsAborted(t *testing.T) {
	configPath := writeTestConfigFile(t, `{
		"google": {
			"name": "Google Calendar",
			"accounts": [
				{
					"name": "Existing",
					"clientId": "existing-client",
					"clientSecret": "existing-secret",
					"calendars": []
				}
			]
		}
	}`)
	original, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountAddPrompter{
		accountInput: accountAddInput{
			Name:         "New",
			ClientID:     "new-client",
			ClientSecret: "new-secret",
		},
		showNoCalendarsErr: huh.ErrUserAborted,
	}
	backingStore := tokenstore.NewInMemoryTokenStore()

	err = runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountAddPrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return backingStore },
		discoverCalendars: func(ctx context.Context, account *appconfig.GoogleAccount, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountAdd() error = %v, want nil", err)
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() after abort error = %v", err)
	}
	if string(after) != string(original) {
		t.Fatalf("config changed after user abort\n got: %s\nwant: %s", string(after), string(original))
	}
	if _, found, err := backingStore.Get(context.Background(), "new-client"); err != nil {
		t.Fatalf("backingStore.Get() error = %v", err)
	} else if found {
		t.Fatal("token store should remain unchanged on user abort")
	}
	if !prompter.noCalendarsShown {
		t.Fatal("expected no-calendars notice to be shown")
	}
}

func TestRunAccountAddTreatsContextCancellationAsUserAbort(t *testing.T) {
	configPath := writeTestConfigFile(t, `{
		"google": {
			"name": "Google Calendar",
			"accounts": [
				{
					"name": "Existing",
					"clientId": "existing-client",
					"clientSecret": "existing-secret",
					"calendars": []
				}
			]
		}
	}`)
	original, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountAddPrompter{
		accountInput: accountAddInput{
			Name:         "New",
			ClientID:     "new-client",
			ClientSecret: "new-secret",
		},
		calendarSelectionErr: context.Canceled,
	}
	backingStore := tokenstore.NewInMemoryTokenStore()

	err = runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountAddPrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return backingStore },
		discoverCalendars: func(ctx context.Context, account *appconfig.GoogleAccount, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{{
				Calendar: appconfig.Calendar{Name: "Primary", ID: "primary-id"},
				Primary:  true,
			}}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountAdd() error = %v, want nil", err)
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() after abort error = %v", err)
	}
	if string(after) != string(original) {
		t.Fatalf("config changed after user abort\n got: %s\nwant: %s", string(after), string(original))
	}
	if _, found, err := backingStore.Get(context.Background(), "new-client"); err != nil {
		t.Fatalf("backingStore.Get() error = %v", err)
	} else if found {
		t.Fatal("token store should remain unchanged on user abort")
	}
}

func TestRunAccountAddReturnsMalformedConfigError(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeTestConfigFile(t, `{invalid json}`))

	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountAddPrompter { return &stubAccountAddPrompter{} },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		discoverCalendars: func(ctx context.Context, account *appconfig.GoogleAccount, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("runAccountAdd() error = nil, want error")
	}

	want := "failed to load config: failed to parse config file: invalid character 'i' looking for beginning of object key string"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestRunAccountAddSavesEmptyCalendarsWhenNoneDiscovered(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountAddPrompter{
		accountInput: accountAddInput{
			Name:         "Work",
			ClientID:     "client-id",
			ClientSecret: "client-secret",
		},
	}

	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountAddPrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		discoverCalendars: func(ctx context.Context, account *appconfig.GoogleAccount, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountAdd() error = %v", err)
	}

	loaded, err := loader.Load()
	if err != nil {
		t.Fatalf("loader.Load() error = %v", err)
	}

	googleCfg, err := loaded.GetGoogleConfig()
	if err != nil {
		t.Fatalf("loaded.GetGoogleConfig() error = %v", err)
	}

	if len(googleCfg.Accounts) != 1 {
		t.Fatalf("accounts length = %d, want 1", len(googleCfg.Accounts))
	}
	if googleCfg.Accounts[0].Calendars == nil {
		t.Fatal("calendars = nil, want empty slice")
	}
	if len(googleCfg.Accounts[0].Calendars) != 0 {
		t.Fatalf("calendars length = %d, want 0", len(googleCfg.Accounts[0].Calendars))
	}
	if !prompter.noCalendarsShown {
		t.Fatal("expected no-calendars notice to be shown")
	}
}

func TestRunAccountAddRollsBackConfigWhenTokenCommitFails(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	initial := `{
		"google": {
			"name": "Google Calendar",
			"accounts": [
				{
					"name": "Existing",
					"clientId": "existing-client",
					"clientSecret": "existing-secret",
					"calendars": []
				}
			]
		}
	}`
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountAddPrompter{
		accountInput: accountAddInput{
			Name:         "Work",
			ClientID:     "client-id",
			ClientSecret: "client-secret",
		},
		selectedCalendars: []appconfig.Calendar{{Name: "Primary", ID: "primary-id"}},
	}
	backingStore := &failingTokenStore{clearErr: errors.New("keyring unavailable")}

	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountAddPrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return backingStore },
		discoverCalendars: func(ctx context.Context, account *appconfig.GoogleAccount, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			providerName := account.ClientID
			if err := authenticator.ClearToken(ctx, &stubProvider{name: providerName, clientID: providerName}); err != nil {
				return nil, err
			}
			return []calendars.DiscoveredCalendar{{
				Calendar: appconfig.Calendar{Name: "Primary", ID: "primary-id"},
				Primary:  true,
			}}, nil
		},
	})
	if err == nil {
		t.Fatal("runAccountAdd() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to persist OAuth token") {
		t.Fatalf("error = %q, want token persistence error", err.Error())
	}

	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("os.ReadFile() error = %v", readErr)
	}
	if string(data) != initial {
		t.Fatalf("config was not restored after token commit failure\n got: %s\nwant: %s", string(data), initial)
	}
}

func TestAccountAddFormRejectsDuplicateAccountName(t *testing.T) {
	googleCfg := &appconfig.GoogleCalendar{
		Name: "Google Calendar",
		Accounts: []appconfig.GoogleAccount{
			{Name: "Work", ClientID: "existing-client"},
		},
	}

	var input accountAddInput
	out := &strings.Builder{}
	form := newAccountDetailsForm(&input, googleCfg).
		WithAccessible(true).
		WithInput(strings.NewReader("Work\nPersonal\nclient-id\nclient-secret\n")).
		WithOutput(out)

	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}

	if input.Name != "Personal" {
		t.Fatalf("input.Name = %q, want %q", input.Name, "Personal")
	}
	if !strings.Contains(out.String(), `account name already exists: "Work"`) {
		t.Fatalf("form output = %q, want duplicate-name validation message", out.String())
	}
}

func newTestCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.SetOut(ioDiscard{})
	cmd.SetErr(ioDiscard{})
	return cmd
}

type stubAccountAddPrompter struct {
	accountInput         accountAddInput
	accountErr           error
	selectedCalendars    []appconfig.Calendar
	calendarSelectionErr error
	showNoCalendarsErr   error
	noCalendarsShown     bool
}

func (s *stubAccountAddPrompter) PromptAccountDetails(context.Context, *appconfig.GoogleCalendar) (accountAddInput, error) {
	if s.accountErr != nil {
		return accountAddInput{}, s.accountErr
	}
	return s.accountInput, nil
}

func (s *stubAccountAddPrompter) PromptCalendarSelection(context.Context, string, []calendars.DiscoveredCalendar) ([]appconfig.Calendar, error) {
	if s.calendarSelectionErr != nil {
		return nil, s.calendarSelectionErr
	}
	return s.selectedCalendars, nil
}

func (s *stubAccountAddPrompter) ShowNoCalendarsFound(context.Context, string) error {
	s.noCalendarsShown = true
	return s.showNoCalendarsErr
}

type failingTokenStore struct {
	setErr   error
	clearErr error
}

func (s *failingTokenStore) Set(context.Context, string, *oauth2.Token) error {
	return s.setErr
}

func (s *failingTokenStore) Get(context.Context, string) (*oauth2.Token, bool, error) {
	return nil, false, nil
}

func (s *failingTokenStore) Clear(context.Context, string) error {
	return s.clearErr
}

type stubProvider struct {
	name     string
	clientID string
}

func (p *stubProvider) Name() string                             { return p.name }
func (p *stubProvider) ClientID() string                         { return p.clientID }
func (p *stubProvider) ClientSecret() string                     { return "secret" }
func (p *stubProvider) AuthURL() string                          { return "https://example.com/auth" }
func (p *stubProvider) TokenURL() string                         { return "https://example.com/token" }
func (p *stubProvider) RedirectURL() string                      { return appconfig.DefaultCallbackURL }
func (p *stubProvider) Scopes() []string                         { return []string{"scope"} }
func (p *stubProvider) AuthCodeOptions() []oauth2.AuthCodeOption { return nil }
func (p *stubProvider) ExchangeOptions() []oauth2.AuthCodeOption { return nil }

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
