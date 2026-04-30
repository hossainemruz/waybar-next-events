package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/hossainemruz/waybar-next-events/internal/auth/providers"
	"github.com/hossainemruz/waybar-next-events/internal/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"golang.org/x/oauth2"
)

func TestAccountManagerAddAccountPersistsConfigSecretsAndToken(t *testing.T) {
	loader := newMemoryConfigLoader()
	secretStore := secrets.NewInMemoryStore()
	tokenStore := tokenstore.NewInMemoryTokenStore()
	service := &stubAppService{
		serviceType:         calendar.ServiceTypeGoogle,
		fields:              []calendar.AccountField{{Key: "client_secret", Secret: true}},
		discoveredCalendars: []calendar.Calendar{{ID: "primary", Name: "Primary", Primary: true}},
	}
	manager := newTestAccountManager(loader, secretStore, tokenStore, service)
	selector := &stubCalendarSelector{selected: []calendar.CalendarRef{{ID: "primary", Name: "Primary"}}}

	account, err := manager.AddAccount(context.Background(), AddAccountInput{
		Service:          calendar.ServiceTypeGoogle,
		Name:             "Work",
		Settings:         map[string]string{"client_id": "client-id"},
		Secrets:          map[string]string{"client_secret": "secret-value"},
		CalendarSelector: selector,
	})
	if err != nil {
		t.Fatalf("AddAccount() error = %v", err)
	}
	if account.ID != "generated-id" {
		t.Fatalf("account.ID = %q, want generated-id", account.ID)
	}
	if len(loader.cfg.Accounts) != 1 || loader.cfg.Accounts[0].Name != "Work" {
		t.Fatalf("saved accounts = %+v", loader.cfg.Accounts)
	}
	secretValue, err := secretStore.Get(context.Background(), account.ID, "client_secret")
	if err != nil {
		t.Fatalf("Get(secret) error = %v", err)
	}
	if secretValue != "secret-value" {
		t.Fatalf("secretValue = %q, want secret-value", secretValue)
	}
	token, found, err := tokenStore.Get(context.Background(), tokenKey(account))
	if err != nil {
		t.Fatalf("Get(token) error = %v", err)
	}
	if !found || token.AccessToken != "token-generated-id" {
		t.Fatalf("stored token = %+v, found=%v", token, found)
	}
	if selector.confirmedEmpty {
		t.Fatal("unexpected empty-calendar confirmation")
	}
}

func TestAccountManagerAddAccountRollsBackOnTokenCommitFailure(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "existing", Service: calendar.ServiceTypeGoogle, Name: "Existing"}})
	secretStore := secrets.NewInMemoryStore()
	tokenStore := &failingTokenStore{setErr: errors.New("token store down")}
	service := &stubAppService{
		serviceType:         calendar.ServiceTypeGoogle,
		fields:              []calendar.AccountField{{Key: "client_secret", Secret: true}},
		discoveredCalendars: []calendar.Calendar{{ID: "primary", Name: "Primary"}},
	}
	manager := newTestAccountManager(loader, secretStore, tokenStore, service)

	_, err := manager.AddAccount(context.Background(), AddAccountInput{
		Service:          calendar.ServiceTypeGoogle,
		Name:             "Work",
		Settings:         map[string]string{"client_id": "client-id"},
		Secrets:          map[string]string{"client_secret": "secret-value"},
		CalendarSelector: &stubCalendarSelector{selected: []calendar.CalendarRef{{ID: "primary", Name: "Primary"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "persist OAuth token") {
		t.Fatalf("error = %v, want token persistence error", err)
	}
	if len(loader.cfg.Accounts) != 1 || loader.cfg.Accounts[0].Name != "Existing" {
		t.Fatalf("accounts after rollback = %+v", loader.cfg.Accounts)
	}
	if _, err := secretStore.Get(context.Background(), "generated-id", "client_secret"); !errors.Is(err, secrets.ErrSecretNotFound) {
		t.Fatalf("secret after rollback error = %v, want ErrSecretNotFound", err)
	}
}

func TestAccountManagerAddAccountReturnsSaveErrorWithoutPersistingSecrets(t *testing.T) {
	loader := newMemoryConfigLoader()
	loader.saveErr = errors.New("save failed")
	secretStore := secrets.NewInMemoryStore()
	tokenStore := tokenstore.NewInMemoryTokenStore()
	service := &stubAppService{
		serviceType:         calendar.ServiceTypeGoogle,
		fields:              []calendar.AccountField{{Key: "client_secret", Secret: true}},
		discoveredCalendars: []calendar.Calendar{{ID: "primary", Name: "Primary"}},
	}
	manager := newTestAccountManager(loader, secretStore, tokenStore, service)

	_, err := manager.AddAccount(context.Background(), AddAccountInput{
		Service:          calendar.ServiceTypeGoogle,
		Name:             "Work",
		Settings:         map[string]string{"client_id": "client-id"},
		Secrets:          map[string]string{"client_secret": "secret-value"},
		CalendarSelector: &stubCalendarSelector{selected: []calendar.CalendarRef{{ID: "primary", Name: "Primary"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "save config") {
		t.Fatalf("error = %v, want save config error", err)
	}
	if _, err := secretStore.Get(context.Background(), "generated-id", "client_secret"); !errors.Is(err, secrets.ErrSecretNotFound) {
		t.Fatalf("secret after save failure error = %v, want ErrSecretNotFound", err)
	}
}

func TestAccountManagerUpdateAccountPreservesExistingTokenWhenCredentialsUnchanged(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "client-id"}, Calendars: []calendar.CalendarRef{{ID: "old", Name: "Old"}}}})
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", "client_secret", "secret-value")
	tokenStore := tokenstore.NewInMemoryTokenStore()
	_ = tokenStore.Set(context.Background(), tokenstore.TokenKey("google", "work-id"), &oauth2.Token{AccessToken: "existing-token"})
	service := &stubAppService{
		serviceType:         calendar.ServiceTypeGoogle,
		fields:              []calendar.AccountField{{Key: "client_secret", Secret: true}},
		discoveredCalendars: []calendar.Calendar{{ID: "new", Name: "New"}},
	}
	manager := newTestAccountManager(loader, secretStore, tokenStore, service)

	updated, err := manager.UpdateAccount(context.Background(), UpdateAccountInput{
		AccountID:        "work-id",
		Name:             "Work Renamed",
		Settings:         map[string]string{"client_id": "client-id"},
		Secrets:          map[string]string{"client_secret": "secret-value"},
		CalendarSelector: &stubCalendarSelector{selected: []calendar.CalendarRef{{ID: "new", Name: "New"}}},
	})
	if err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}
	storedToken, found, err := tokenStore.Get(context.Background(), tokenKey(updated))
	if err != nil {
		t.Fatalf("Get(token) error = %v", err)
	}
	if !found || storedToken.AccessToken != "existing-token" {
		t.Fatalf("storedToken = %+v, found=%v", storedToken, found)
	}
}

func TestAccountManagerDeleteAccountRemovesConfigSecretsAndToken(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}})
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", "client_secret", "secret-value")
	tokenStore := tokenstore.NewInMemoryTokenStore()
	_ = tokenStore.Set(context.Background(), tokenstore.TokenKey("google", "work-id"), &oauth2.Token{AccessToken: "existing-token"})
	service := &stubAppService{serviceType: calendar.ServiceTypeGoogle, fields: []calendar.AccountField{{Key: "client_secret", Secret: true}}}
	manager := newTestAccountManager(loader, secretStore, tokenStore, service)

	deleted, err := manager.DeleteAccount(context.Background(), DeleteAccountInput{AccountID: "work-id"})
	if err != nil {
		t.Fatalf("DeleteAccount() error = %v", err)
	}
	if deleted.Name != "Work" {
		t.Fatalf("deleted.Name = %q, want Work", deleted.Name)
	}
	if len(loader.cfg.Accounts) != 0 {
		t.Fatalf("remaining accounts = %+v, want none", loader.cfg.Accounts)
	}
	if _, err := secretStore.Get(context.Background(), "work-id", "client_secret"); !errors.Is(err, secrets.ErrSecretNotFound) {
		t.Fatalf("secret lookup error = %v, want ErrSecretNotFound", err)
	}
	if token, found, err := tokenStore.Get(context.Background(), tokenstore.TokenKey("google", "work-id")); err != nil || found || token != nil {
		t.Fatalf("token after delete = %+v, found=%v, err=%v", token, found, err)
	}
}

func TestAccountManagerDeleteAccountRollsBackConfigWhenSecretCommitFails(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}})
	secretStore := &failingSecretStore{deleteErr: errors.New("secret store down"), values: map[string]string{"work-id/client_secret": "secret-value"}}
	tokenStore := tokenstore.NewInMemoryTokenStore()
	service := &stubAppService{serviceType: calendar.ServiceTypeGoogle, fields: []calendar.AccountField{{Key: "client_secret", Secret: true}}}
	manager := newTestAccountManager(loader, secretStore, tokenStore, service)

	_, err := manager.DeleteAccount(context.Background(), DeleteAccountInput{AccountID: "work-id"})
	if err == nil || !strings.Contains(err.Error(), "persist secrets") {
		t.Fatalf("error = %v, want persist secrets error", err)
	}
	if len(loader.cfg.Accounts) != 1 || loader.cfg.Accounts[0].Name != "Work" {
		t.Fatalf("accounts after rollback = %+v", loader.cfg.Accounts)
	}
}

func TestAccountManagerLoginAccountCommitsReplacementToken(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}})
	secretStore := secrets.NewInMemoryStore()
	tokenStore := tokenstore.NewInMemoryTokenStore()
	service := &stubAppService{serviceType: calendar.ServiceTypeGoogle}
	manager := newTestAccountManager(loader, secretStore, tokenStore, service)

	account, err := manager.LoginAccount(context.Background(), LoginAccountInput{AccountID: "work-id"})
	if err != nil {
		t.Fatalf("LoginAccount() error = %v", err)
	}
	if account.Name != "Work" {
		t.Fatalf("account.Name = %q, want Work", account.Name)
	}
	storedToken, found, err := tokenStore.Get(context.Background(), tokenstore.TokenKey("google", "work-id"))
	if err != nil {
		t.Fatalf("Get(token) error = %v", err)
	}
	if !found || storedToken.AccessToken != "forced-work-id" {
		t.Fatalf("storedToken = %+v, found=%v", storedToken, found)
	}
}

type stubAppService struct {
	serviceType         calendar.ServiceType
	fields              []calendar.AccountField
	discoveredCalendars []calendar.Calendar
	events              []calendar.Event
	providerErr         error
	discoverErr         error
	fetchErr            error
}

func (s *stubAppService) Type() calendar.ServiceType             { return s.serviceType }
func (s *stubAppService) DisplayName() string                    { return string(s.serviceType) }
func (s *stubAppService) AccountFields() []calendar.AccountField { return s.fields }
func (s *stubAppService) AuthProvider(account calendar.Account) (calendar.AuthProvider, error) {
	return &stubProvider{name: tokenstore.TokenKey(string(s.serviceType), account.ID)}, nil
}
func (s *stubAppService) Provider(_ context.Context, account calendar.Account, _ secrets.Store) (providers.Provider, error) {
	if s.providerErr != nil {
		return nil, s.providerErr
	}
	return &stubProvider{name: tokenstore.TokenKey(string(s.serviceType), account.ID)}, nil
}
func (s *stubAppService) DiscoverCalendars(context.Context, calendar.Account, *http.Client) ([]calendar.Calendar, error) {
	if s.discoverErr != nil {
		return nil, s.discoverErr
	}
	return s.discoveredCalendars, nil
}
func (s *stubAppService) FetchEvents(context.Context, calendar.Account, calendar.EventQuery, *http.Client) ([]calendar.Event, error) {
	if s.fetchErr != nil {
		return nil, s.fetchErr
	}
	return s.events, nil
}

type stubCalendarSelector struct {
	selected       []calendar.CalendarRef
	selectErr      error
	confirmedEmpty bool
	confirmErr     error
}

func (s *stubCalendarSelector) SelectCalendars(context.Context, calendar.Account, []calendar.Calendar) ([]calendar.CalendarRef, error) {
	return s.selected, s.selectErr
}
func (s *stubCalendarSelector) ConfirmEmptyCalendars(context.Context, calendar.Account) error {
	s.confirmedEmpty = true
	return s.confirmErr
}

type memoryConfigLoader struct {
	cfg      *config.Config
	lastSnap *config.Config
	loadErr  error
	saveErr  error
}

func newMemoryConfigLoader() *memoryConfigLoader { return newMemoryConfigLoaderWithAccounts(nil) }
func newMemoryConfigLoaderWithAccounts(accounts []calendar.Account) *memoryConfigLoader {
	cfg := &config.Config{Accounts: accounts}
	cfg.Normalize()
	return &memoryConfigLoader{cfg: cfg}
}
func (l *memoryConfigLoader) Load() (*config.Config, error) {
	if l.loadErr != nil {
		return nil, l.loadErr
	}
	return cloneConfig(l.cfg), nil
}
func (l *memoryConfigLoader) LoadOrEmpty() (*config.Config, error) { return l.Load() }
func (l *memoryConfigLoader) Save(cfg *config.Config) error {
	if l.saveErr != nil {
		return l.saveErr
	}
	l.cfg = cloneConfig(cfg)
	return nil
}
func (l *memoryConfigLoader) Snapshot() (config.Snapshot, error) {
	l.lastSnap = cloneConfig(l.cfg)
	return config.Snapshot{}, nil
}
func (l *memoryConfigLoader) RestoreSnapshot(config.Snapshot) error {
	l.cfg = cloneConfig(l.lastSnap)
	return nil
}

func cloneConfig(cfg *config.Config) *config.Config {
	if cfg == nil {
		return nil
	}
	cloned := &config.Config{Accounts: make([]calendar.Account, len(cfg.Accounts))}
	copy(cloned.Accounts, cfg.Accounts)
	for i := range cloned.Accounts {
		cloned.Accounts[i].Settings = cloneMap(cloned.Accounts[i].Settings)
		cloned.Accounts[i].Calendars = append([]calendar.CalendarRef(nil), cloned.Accounts[i].Calendars...)
	}
	return cloned
}

func cloneMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func newTestAccountManager(loader ConfigLoader, secretStore secrets.Store, tokenStore tokenstore.TokenStore, service calendar.Service) *AccountManager {
	registry := calendar.NewRegistry()
	if err := registry.Register(service); err != nil {
		panic(err)
	}
	manager := NewAccountManager(loader, registry, secretStore, tokenStore)
	manager.newAccountID = func() (string, error) { return "generated-id", nil }
	manager.newAuthenticator = func(store tokenstore.TokenStore) Authenticator { return &stubAuthenticator{store: store} }
	return manager
}

type stubAuthenticator struct{ store tokenstore.TokenStore }

func (a *stubAuthenticator) Authenticate(context.Context, providers.Provider) (*oauth2.Token, error) {
	return nil, nil
}
func (a *stubAuthenticator) ForceAuthenticate(ctx context.Context, provider providers.Provider) (*oauth2.Token, error) {
	token := &oauth2.Token{AccessToken: "forced-work-id"}
	if err := a.store.Set(ctx, provider.Name(), token); err != nil {
		return nil, err
	}
	return token, nil
}
func (a *stubAuthenticator) HTTPClient(ctx context.Context, provider providers.Provider) (*http.Client, error) {
	if _, found, err := a.store.Get(ctx, provider.Name()); err != nil {
		return nil, err
	} else if !found {
		if err := a.store.Set(ctx, provider.Name(), &oauth2.Token{AccessToken: "token-" + strings.TrimPrefix(provider.Name(), "google/")}); err != nil {
			return nil, err
		}
	}
	return &http.Client{}, nil
}

type failingTokenStore struct{ setErr error }

func (s *failingTokenStore) Set(context.Context, string, *oauth2.Token) error { return s.setErr }
func (s *failingTokenStore) Get(context.Context, string) (*oauth2.Token, bool, error) {
	return nil, false, nil
}
func (s *failingTokenStore) Clear(context.Context, string) error { return nil }

type failingSecretStore struct {
	deleteErr error
	values    map[string]string
}

func (s *failingSecretStore) Get(_ context.Context, accountID, key string) (string, error) {
	value, ok := s.values[accountID+"/"+key]
	if ok {
		return value, nil
	}
	return "", secrets.ErrSecretNotFound
}
func (s *failingSecretStore) Set(context.Context, string, string, string) error { return nil }
func (s *failingSecretStore) Delete(_ context.Context, accountID, key string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.values, accountID+"/"+key)
	return nil
}

type stubProvider struct{ name string }

func (p *stubProvider) Name() string                             { return p.name }
func (p *stubProvider) ClientID() string                         { return "client" }
func (p *stubProvider) ClientSecret() string                     { return "secret" }
func (p *stubProvider) AuthURL() string                          { return "https://example.com/auth" }
func (p *stubProvider) TokenURL() string                         { return "https://example.com/token" }
func (p *stubProvider) RedirectURL() string                      { return config.DefaultCallbackURL }
func (p *stubProvider) Scopes() []string                         { return []string{"scope"} }
func (p *stubProvider) AuthCodeOptions() []oauth2.AuthCodeOption { return nil }
func (p *stubProvider) ExchangeOptions() []oauth2.AuthCodeOption { return nil }
