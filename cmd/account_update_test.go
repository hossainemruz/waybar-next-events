package cmd

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/auth"
	"github.com/hossainemruz/waybar-next-events/internal/auth/tokenstore"
	appcalendar "github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"github.com/hossainemruz/waybar-next-events/pkg/calendars"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

func TestRunAccountUpdateUpdatesSelectedAccount(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{
		{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}, Calendars: []appconfig.CalendarRef{{ID: "primary-id", Name: "Primary"}}},
		{ID: "personal-id", Service: appcalendar.ServiceTypeGoogle, Name: "Personal", Settings: map[string]string{"client_id": "personal-client"}, Calendars: []appconfig.CalendarRef{{ID: "home-id", Name: "Home"}}},
	})
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "work-secret")
	_ = secretStore.Set(context.Background(), "personal-id", googleClientSecretKey, "personal-secret")
	prompter := &stubAccountUpdatePrompter{
		selectedAccountID: "personal-id",
		updatedInput:      accountUpdateInput{Name: "Personal Updated", ClientID: "personal-client", ClientSecret: "personal-secret"},
		selectedCalendars: []appconfig.CalendarRef{{Name: "Travel", ID: "travel-id"}},
	}

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountUpdatePrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken:     func(context.Context, *auth.Authenticator, secrets.Store, *appconfig.Account) error { return nil },
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
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
	if _, ok := updated.Settings[googleClientSecretKey]; ok {
		t.Fatalf("updated settings unexpectedly contained %q: %+v", googleClientSecretKey, updated.Settings)
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
			"client_id": "work-client",
			"tenant_id": "tenant-123",
		},
	}})
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "work-secret")
	prompter := &stubAccountUpdatePrompter{selectedAccountID: "work-id", updatedInput: accountUpdateInput{Name: "Work", ClientID: "updated-client", ClientSecret: "updated-secret"}}

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountUpdatePrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken:     func(context.Context, *auth.Authenticator, secrets.Store, *appconfig.Account) error { return nil },
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
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
	storedSecret, err := secretStore.Get(context.Background(), "work-id", googleClientSecretKey)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if storedSecret != "updated-secret" {
		t.Fatalf("stored secret = %q, want updated-secret", storedSecret)
	}
}

func TestRunAccountUpdateClearsOldTokenWhenCredentialsChange(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "old-client"}}})
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "old-secret")
	prompter := &stubAccountUpdatePrompter{selectedAccountID: "work-id", updatedInput: accountUpdateInput{Name: "Work", ClientID: "new-client", ClientSecret: "new-secret"}}
	clearCalled := false

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountUpdatePrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken: func(context.Context, *auth.Authenticator, secrets.Store, *appconfig.Account) error {
			clearCalled = true
			return nil
		},
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
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

func TestRunAccountUpdateReusesExistingTokenWhenCredentialsUnchanged(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}}})
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "work-secret")
	backingStore := tokenstore.NewInMemoryTokenStore()
	existingToken := &oauth2.Token{AccessToken: "existing-token", RefreshToken: "refresh-token", Expiry: time.Now().Add(time.Hour)}
	if err := backingStore.Set(context.Background(), tokenstore.TokenKey(string(appcalendar.ServiceTypeGoogle), "work-id"), existingToken); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	prompter := &stubAccountUpdatePrompter{selectedAccountID: "work-id", updatedInput: accountUpdateInput{Name: "Work", ClientID: "work-client", ClientSecret: "work-secret"}}

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountUpdatePrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return backingStore },
		clearToken: func(context.Context, *auth.Authenticator, secrets.Store, *appconfig.Account) error {
			t.Fatal("clearToken should not be called")
			return nil
		},
		discoverCalendars: func(ctx context.Context, account *appconfig.Account, secretStore secrets.Store, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			provider, err := calendars.GoogleProviderForAccount(ctx, account, secretStore)
			if err != nil {
				return nil, err
			}
			token, err := authenticator.Authenticate(ctx, provider)
			if err != nil {
				return nil, err
			}
			if token.AccessToken != "existing-token" {
				t.Fatalf("Authenticate() token = %q, want existing-token", token.AccessToken)
			}
			return []calendars.DiscoveredCalendar{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountUpdate() error = %v", err)
	}
}

func TestRunAccountUpdateRenamePreservesExistingToken(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}}})
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "work-secret")
	backingStore := tokenstore.NewInMemoryTokenStore()
	tokenKey := tokenstore.TokenKey(string(appcalendar.ServiceTypeGoogle), "work-id")
	existingToken := &oauth2.Token{AccessToken: "existing-token", RefreshToken: "refresh-token", Expiry: time.Now().Add(time.Hour)}
	if err := backingStore.Set(context.Background(), tokenKey, existingToken); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	prompter := &stubAccountUpdatePrompter{selectedAccountID: "work-id", updatedInput: accountUpdateInput{Name: "Work Renamed", ClientID: "work-client", ClientSecret: "work-secret"}}

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountUpdatePrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return backingStore },
		clearToken: func(context.Context, *auth.Authenticator, secrets.Store, *appconfig.Account) error {
			t.Fatal("clearToken should not be called")
			return nil
		},
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountUpdate() error = %v", err)
	}

	loaded, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	renamed := loaded.FindAccountByID("work-id")
	if renamed == nil || renamed.Name != "Work Renamed" {
		t.Fatalf("renamed account = %+v, want renamed account", renamed)
	}

	storedToken, found, err := backingStore.Get(context.Background(), tokenKey)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found || storedToken == nil || storedToken.AccessToken != "existing-token" {
		t.Fatalf("stored token = %+v, found=%v, want preserved token", storedToken, found)
	}
}

func TestRunAccountUpdateReturnsNoAccountsErrorWhenConfigMissing(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(filepath.Join(t.TempDir(), "missing.json"))
	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountUpdatePrompter { return &stubAccountUpdatePrompter{} },
		newSecretStore: func() secrets.Store { return secrets.NewInMemoryStore() },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken:     func(context.Context, *auth.Authenticator, secrets.Store, *appconfig.Account) error { return nil },
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return nil, nil
		},
	})
	if !errors.Is(err, appconfig.ErrNoAccounts) {
		t.Fatalf("runAccountUpdate() error = %v, want ErrNoAccounts", err)
	}
}

func TestRunAccountUpdatePreservesConfigOnAbort(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}}})
	original := readFile(t, configPath)
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "work-secret")

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader: func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountUpdatePrompter {
			return &stubAccountUpdatePrompter{selectionErr: huh.ErrUserAborted}
		},
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken:     func(context.Context, *auth.Authenticator, secrets.Store, *appconfig.Account) error { return nil },
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountUpdate() error = %v, want nil", err)
	}
	assertConfigUnchanged(t, configPath, original)
}

func TestRunAccountUpdateRollsBackConfigWhenTokenCommitFails(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "old-client"}}})
	original := readFile(t, configPath)
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "old-secret")
	prompter := &stubAccountUpdatePrompter{selectedAccountID: "work-id", updatedInput: accountUpdateInput{Name: "Work Updated", ClientID: "new-client", ClientSecret: "new-secret"}}
	backingStore := &failingTokenStore{clearErr: errors.New("keyring unavailable")}

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountUpdatePrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return backingStore },
		clearToken:     clearAccountToken,
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return []calendars.DiscoveredCalendar{{Calendar: appconfig.CalendarRef{Name: "Primary", ID: "primary-id"}}}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to persist OAuth token") {
		t.Fatalf("error = %v, want token persistence error", err)
	}
	assertConfigUnchanged(t, configPath, original)
	storedSecret, err := secretStore.Get(context.Background(), "work-id", googleClientSecretKey)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if storedSecret != "old-secret" {
		t.Fatalf("stored secret = %q, want old-secret after rollback", storedSecret)
	}
}

func TestRunAccountUpdateReturnsMissingSecretError(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}}})
	loader := appconfig.NewLoaderWithPath(configPath)

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader: func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountUpdatePrompter {
			return &stubAccountUpdatePrompter{selectedAccountID: "work-id"}
		},
		newSecretStore: func() secrets.Store { return secrets.NewInMemoryStore() },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken:     clearAccountToken,
		discoverCalendars: func(context.Context, *appconfig.Account, secrets.Store, *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			return nil, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "missing stored secret") {
		t.Fatalf("error = %v, want missing stored secret error", err)
	}
}

func TestGoogleTokenKeyUsesStableAccountIdentity(t *testing.T) {
	account := &appconfig.Account{ID: "account-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "shared-client"}}

	got, err := googleTokenKey(account)
	if err != nil {
		t.Fatalf("googleTokenKey() error = %v", err)
	}

	if got != "google/account-id" {
		t.Fatalf("googleTokenKey() = %q, want %q", got, "google/account-id")
	}
}

func TestSeedStagedTokenStoreUsesStableAccountIdentity(t *testing.T) {
	ctx := context.Background()
	stagedStore := tokenstore.NewStagedTokenStore()
	backingStore := tokenstore.NewInMemoryTokenStore()
	account := &appconfig.Account{ID: "account-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "shared-client"}}
	existingToken := &oauth2.Token{AccessToken: "existing-token", Expiry: time.Now().Add(time.Hour)}
	if err := backingStore.Set(ctx, tokenstore.TokenKey(string(appcalendar.ServiceTypeGoogle), account.ID), existingToken); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if err := seedStagedTokenStore(ctx, stagedStore, backingStore, account); err != nil {
		t.Fatalf("seedStagedTokenStore() error = %v", err)
	}

	stagedToken, found, err := stagedStore.Get(ctx, tokenstore.TokenKey(string(appcalendar.ServiceTypeGoogle), account.ID))
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found || stagedToken == nil || stagedToken.AccessToken != "existing-token" {
		t.Fatalf("staged token = %+v, found=%v, want existing token", stagedToken, found)
	}
}

func TestRunAccountUpdateKeepsSeparateTokensForSharedClientID(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{
		{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "shared-client"}},
		{ID: "personal-id", Service: appcalendar.ServiceTypeGoogle, Name: "Personal", Settings: map[string]string{"client_id": "shared-client"}},
	})
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "work-secret")
	_ = secretStore.Set(context.Background(), "personal-id", googleClientSecretKey, "personal-secret")
	backingStore := tokenstore.NewInMemoryTokenStore()
	workKey := tokenstore.TokenKey(string(appcalendar.ServiceTypeGoogle), "work-id")
	personalKey := tokenstore.TokenKey(string(appcalendar.ServiceTypeGoogle), "personal-id")
	if err := backingStore.Set(context.Background(), workKey, &oauth2.Token{AccessToken: "work-token", Expiry: time.Now().Add(time.Hour)}); err != nil {
		t.Fatalf("Set(work) error = %v", err)
	}
	if err := backingStore.Set(context.Background(), personalKey, &oauth2.Token{AccessToken: "personal-token", Expiry: time.Now().Add(time.Hour)}); err != nil {
		t.Fatalf("Set(personal) error = %v", err)
	}
	prompter := &stubAccountUpdatePrompter{selectedAccountID: "personal-id", updatedInput: accountUpdateInput{Name: "Personal Updated", ClientID: "shared-client", ClientSecret: "personal-secret"}}

	err := runAccountUpdate(newTestCommand(), accountUpdateDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountUpdatePrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return backingStore },
		clearToken: func(context.Context, *auth.Authenticator, secrets.Store, *appconfig.Account) error {
			t.Fatal("clearToken should not be called")
			return nil
		},
		discoverCalendars: func(ctx context.Context, account *appconfig.Account, secretStore secrets.Store, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error) {
			provider, err := calendars.GoogleProviderForAccount(ctx, account, secretStore)
			if err != nil {
				return nil, err
			}
			token, err := authenticator.Authenticate(ctx, provider)
			if err != nil {
				return nil, err
			}
			if token.AccessToken != "personal-token" {
				t.Fatalf("Authenticate() token = %q, want %q", token.AccessToken, "personal-token")
			}
			return []calendars.DiscoveredCalendar{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountUpdate() error = %v", err)
	}

	workToken, found, err := backingStore.Get(context.Background(), workKey)
	if err != nil {
		t.Fatalf("Get(work) error = %v", err)
	}
	if !found || workToken == nil || workToken.AccessToken != "work-token" {
		t.Fatalf("work token = %+v, found=%v, want preserved work token", workToken, found)
	}
	personalToken, found, err := backingStore.Get(context.Background(), personalKey)
	if err != nil {
		t.Fatalf("Get(personal) error = %v", err)
	}
	if !found || personalToken == nil || personalToken.AccessToken != "personal-token" {
		t.Fatalf("personal token = %+v, found=%v, want preserved personal token", personalToken, found)
	}
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
