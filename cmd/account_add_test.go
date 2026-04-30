package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	appcalendar "github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/pkg/auth"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/pkg/calendars"
)

func TestRunAccountAddCreatesConfigOnFirstRun(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	prompter := &stubAccountAddPrompter{
		accountInput:      accountAddInput{Name: "Work", ClientID: "client-id", ClientSecret: "client-secret"},
		selectedCalendars: []appconfig.CalendarRef{{Name: "Primary", ID: "primary-id"}},
	}

	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountAddPrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{{Calendar: appconfig.CalendarRef{Name: "Primary", ID: "primary-id"}, Primary: true}}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountAdd() error = %v", err)
	}

	loaded, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	googleAccounts := loaded.AccountsByService(appcalendar.ServiceTypeGoogle)
	if len(googleAccounts) != 1 {
		t.Fatalf("len(google accounts) = %d, want 1", len(googleAccounts))
	}
	account := googleAccounts[0]
	if account.ID == "" {
		t.Fatal("account.ID = empty, want generated stable ID")
	}
	if account.Name != "Work" {
		t.Fatalf("account.Name = %q, want Work", account.Name)
	}
	if account.Setting("client_id") != "client-id" {
		t.Fatalf("account settings = %+v", account.Settings)
	}
	if _, ok := account.Settings[googleClientSecretKey]; ok {
		t.Fatalf("account settings unexpectedly contained %q: %+v", googleClientSecretKey, account.Settings)
	}
	secretValue, err := secretStore.Get(context.Background(), account.ID, googleClientSecretKey)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if secretValue != "client-secret" {
		t.Fatalf("stored secret = %q, want client-secret", secretValue)
	}
	if len(account.Calendars) != 1 || account.Calendars[0].ID != "primary-id" {
		t.Fatalf("account.Calendars = %+v, want primary-id", account.Calendars)
	}
}

func TestRunAccountAddSavesEmptyCalendarsWhenNoneDiscovered(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(filepath.Join(t.TempDir(), "config.json"))
	secretStore := secrets.NewInMemoryStore()
	prompter := &stubAccountAddPrompter{accountInput: accountAddInput{Name: "Work", ClientID: "client-id", ClientSecret: "client-secret"}}

	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountAddPrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountAdd() error = %v", err)
	}

	loaded, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	account := loaded.AccountsByService(appcalendar.ServiceTypeGoogle)[0]
	if account.Calendars == nil || len(account.Calendars) != 0 {
		t.Fatalf("account.Calendars = %+v, want empty slice", account.Calendars)
	}
	if !prompter.noCalendarsShown {
		t.Fatal("expected no calendars prompt to be shown")
	}
}

func TestRunAccountAddDoesNotSaveConfigWhenDiscoveryFails(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	prompter := &stubAccountAddPrompter{accountInput: accountAddInput{Name: "Work", ClientID: "client-id", ClientSecret: "client-secret"}}

	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountAddPrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return nil, errors.New("oauth login failed")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "oauth login failed") {
		t.Fatalf("error = %v, want discovery failure", err)
	}
	if _, statErr := os.Stat(configPath); !os.IsNotExist(statErr) {
		t.Fatalf("config file should not exist, stat error = %v", statErr)
	}
}

func TestRunAccountAddPreservesConfigOnUserAbort(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "existing", Service: appcalendar.ServiceTypeGoogle, Name: "Existing"}})
	original, _ := os.ReadFile(configPath)
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	prompter := &stubAccountAddPrompter{
		accountInput:         accountAddInput{Name: "New", ClientID: "new-client", ClientSecret: "new-secret"},
		calendarSelectionErr: huh.ErrUserAborted,
	}

	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountAddPrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{{Calendar: appconfig.CalendarRef{Name: "Primary", ID: "primary-id"}}}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountAdd() error = %v, want nil", err)
	}

	after, _ := os.ReadFile(configPath)
	if string(after) != string(original) {
		t.Fatalf("config changed after abort\n got: %s\nwant: %s", after, original)
	}
}

func TestRunAccountAddTreatsContextCancellationAsUserAbort(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "existing", Service: appcalendar.ServiceTypeGoogle, Name: "Existing"}})
	original, _ := os.ReadFile(configPath)
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	prompter := &stubAccountAddPrompter{
		accountInput:         accountAddInput{Name: "New", ClientID: "new-client", ClientSecret: "new-secret"},
		calendarSelectionErr: context.Canceled,
	}

	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountAddPrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{{Calendar: appconfig.CalendarRef{Name: "Primary", ID: "primary-id"}}}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountAdd() error = %v, want nil", err)
	}
	assertConfigUnchanged(t, configPath, original)
}

func TestRunAccountAddPreservesConfigWhenNoCalendarsPromptIsAborted(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "existing", Service: appcalendar.ServiceTypeGoogle, Name: "Existing"}})
	original, _ := os.ReadFile(configPath)
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	prompter := &stubAccountAddPrompter{
		accountInput:       accountAddInput{Name: "New", ClientID: "new-client", ClientSecret: "new-secret"},
		showNoCalendarsErr: huh.ErrUserAborted,
	}

	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountAddPrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountAdd() error = %v, want nil", err)
	}
	assertConfigUnchanged(t, configPath, original)
	if !prompter.noCalendarsShown {
		t.Fatal("expected no calendars prompt to be shown")
	}
}

func TestRunAccountAddReturnsMalformedConfigError(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeTestConfigFile(t, `{invalid json}`))
	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountAddPrompter { return &stubAccountAddPrompter{} },
		newSecretStore: func() secrets.Store { return secrets.NewInMemoryStore() },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return nil, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to parse config file") {
		t.Fatalf("error = %v, want malformed config error", err)
	}
}

func TestRunAccountAddRollsBackConfigWhenTokenCommitFails(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "existing", Service: appcalendar.ServiceTypeGoogle, Name: "Existing", Settings: map[string]string{"client_id": "existing-client"}}})
	original, _ := os.ReadFile(configPath)
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	prompter := &stubAccountAddPrompter{
		accountInput:      accountAddInput{Name: "Work", ClientID: "client-id", ClientSecret: "client-secret"},
		selectedCalendars: []appconfig.CalendarRef{{Name: "Primary", ID: "primary-id"}},
	}
	backingStore := &failingTokenStore{clearErr: errors.New("keyring unavailable")}

	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountAddPrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return backingStore },
		discoverCalendars: func(ctx context.Context, account *appconfig.Account, secretStore secrets.Store, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			providerName := account.Setting("client_id")
			if err := authenticator.ClearToken(ctx, &stubProvider{name: providerName, clientID: providerName}); err != nil {
				return nil, err
			}
			if _, err := secretStore.Get(ctx, account.ID, googleClientSecretKey); err != nil {
				return nil, err
			}
			return []calendars.DiscoveredCalendar{{Calendar: appconfig.CalendarRef{Name: "Primary", ID: "primary-id"}}}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to persist OAuth token") {
		t.Fatalf("error = %v, want token persistence error", err)
	}

	after, _ := os.ReadFile(configPath)
	if string(after) != string(original) {
		t.Fatalf("config was not restored\n got: %s\nwant: %s", after, original)
	}
	if len(secretStore.Snapshot()) != 0 {
		t.Fatalf("stored secrets = %+v, want empty after rollback", secretStore.Snapshot())
	}
}

func TestRunAccountAddReturnsSecretStoreError(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(filepath.Join(t.TempDir(), "config.json"))
	prompter := &stubAccountAddPrompter{accountInput: accountAddInput{Name: "Work", ClientID: "client-id", ClientSecret: "client-secret"}}
	secretStore := &failingSecretStore{setErr: errors.New("keyring unavailable")}

	err := runAccountAdd(newTestCommand(), accountAddDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountAddPrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to persist account secrets") {
		t.Fatalf("error = %v, want secret persistence error", err)
	}
}

func TestAccountAddFormRejectsDuplicateAccountName(t *testing.T) {
	cfg := &appconfig.Config{Accounts: []appconfig.Account{{ID: "a", Service: appcalendar.ServiceTypeGoogle, Name: "Work"}}}
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
	selectedCalendars    []appconfig.CalendarRef
	calendarSelectionErr error
	showNoCalendarsErr   error
	noCalendarsShown     bool
}

func (s *stubAccountAddPrompter) PromptAccountDetails(context.Context, *appconfig.Config) (accountAddInput, error) {
	if s.accountErr != nil {
		return accountAddInput{}, s.accountErr
	}
	return s.accountInput, nil
}

func (s *stubAccountAddPrompter) PromptCalendarSelection(context.Context, string, []calendars.DiscoveredCalendar) ([]appconfig.CalendarRef, error) {
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

func (s *failingTokenStore) Set(context.Context, string, *oauth2.Token) error { return s.setErr }
func (s *failingTokenStore) Get(context.Context, string) (*oauth2.Token, bool, error) {
	return nil, false, nil
}
func (s *failingTokenStore) Clear(context.Context, string) error { return s.clearErr }

type failingSecretStore struct {
	getErr    error
	setErr    error
	deleteErr error
	values    map[string]string
}

func (s *failingSecretStore) Get(_ context.Context, accountID, key string) (string, error) {
	if s.getErr != nil {
		return "", s.getErr
	}
	if s.values != nil {
		value, ok := s.values[accountID+"/"+key]
		if ok {
			return value, nil
		}
	}
	return "", secrets.ErrSecretNotFound
}

func (s *failingSecretStore) Set(context.Context, string, string, string) error { return s.setErr }

func (s *failingSecretStore) Delete(_ context.Context, accountID, key string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	if s.values != nil {
		delete(s.values, accountID+"/"+key)
	}
	return nil
}

type stubProvider struct{ name, clientID string }

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

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
