package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
	loader := newMemoryConfigLoader(t)
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
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "existing", Service: calendar.ServiceTypeGoogle, Name: "Existing"}}, t)
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
	loader := newMemoryConfigLoader(t)
	loader.saveError = errors.New("save failed")
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

func TestAccountManagerAddAccountRollsBackOnSecretCommitFailure(t *testing.T) {
	loader := newMemoryConfigLoader(t)
	secretStore := &failingSecretStore{setErr: errors.New("secret store down"), values: map[string]string{}}
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
	if err == nil || !strings.Contains(err.Error(), "persist account secrets") {
		t.Fatalf("error = %v, want secret persistence error", err)
	}
	if len(loader.cfg.Accounts) != 0 {
		t.Fatalf("accounts after rollback = %+v, want none", loader.cfg.Accounts)
	}
}

func TestAccountManagerAddAccountReturnsWithoutMutatingStateOnUserAbort(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "existing", Service: calendar.ServiceTypeGoogle, Name: "Existing"}}, t)
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
		CalendarSelector: &stubCalendarSelector{selectErr: context.Canceled},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if len(loader.cfg.Accounts) != 1 || loader.cfg.Accounts[0].Name != "Existing" {
		t.Fatalf("accounts changed after abort = %+v", loader.cfg.Accounts)
	}
	if len(secretStore.Snapshot()) != 0 {
		t.Fatalf("secrets changed after abort = %+v", secretStore.Snapshot())
	}
	if token, found, err := tokenStore.Get(context.Background(), tokenstore.TokenKey("google", "generated-id")); err != nil || found || token != nil {
		t.Fatalf("token after abort = %+v, found=%v, err=%v", token, found, err)
	}
}

func TestAccountManagerUpdateAccountPreservesExistingTokenWhenCredentialsUnchanged(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "client-id"}, Calendars: []calendar.CalendarRef{{ID: "old", Name: "Old"}}}}, t)
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

func TestAccountManagerUpdateAccountRollsBackOnSaveFailure(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "old-client"}}}, t)
	loader.saveError = errors.New("save failed")
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", "client_secret", "old-secret")
	tokenStore := tokenstore.NewInMemoryTokenStore()
	service := &stubAppService{serviceType: calendar.ServiceTypeGoogle, fields: []calendar.AccountField{{Key: "client_secret", Secret: true}}, discoveredCalendars: []calendar.Calendar{{ID: "primary", Name: "Primary"}}}
	manager := newTestAccountManager(loader, secretStore, tokenStore, service)

	_, err := manager.UpdateAccount(context.Background(), UpdateAccountInput{
		AccountID:        "work-id",
		Name:             "Work Updated",
		Settings:         map[string]string{"client_id": "new-client"},
		Secrets:          map[string]string{"client_secret": "new-secret"},
		CalendarSelector: &stubCalendarSelector{selected: []calendar.CalendarRef{{ID: "primary", Name: "Primary"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "save config") {
		t.Fatalf("error = %v, want save config error", err)
	}
	if loader.cfg.Accounts[0].Name != "Work" {
		t.Fatalf("account changed after save failure = %+v", loader.cfg.Accounts[0])
	}
	secretValue, _ := secretStore.Get(context.Background(), "work-id", "client_secret")
	if secretValue != "old-secret" {
		t.Fatalf("secret after save failure = %q, want old-secret", secretValue)
	}
}

func TestAccountManagerUpdateAccountRollsBackOnSecretCommitFailure(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "old-client"}}}, t)
	secretStore := &failingSecretStore{setErr: errors.New("secret store down"), values: map[string]string{"work-id/client_secret": "old-secret"}}
	tokenStore := tokenstore.NewInMemoryTokenStore()
	service := &stubAppService{serviceType: calendar.ServiceTypeGoogle, fields: []calendar.AccountField{{Key: "client_secret", Secret: true}}, discoveredCalendars: []calendar.Calendar{{ID: "primary", Name: "Primary"}}}
	manager := newTestAccountManager(loader, secretStore, tokenStore, service)

	_, err := manager.UpdateAccount(context.Background(), UpdateAccountInput{
		AccountID:        "work-id",
		Name:             "Work Updated",
		Settings:         map[string]string{"client_id": "new-client"},
		Secrets:          map[string]string{"client_secret": "new-secret"},
		CalendarSelector: &stubCalendarSelector{selected: []calendar.CalendarRef{{ID: "primary", Name: "Primary"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "persist account secrets") {
		t.Fatalf("error = %v, want secret persistence error", err)
	}
	if loader.cfg.Accounts[0].Name != "Work" {
		t.Fatalf("account changed after rollback = %+v", loader.cfg.Accounts[0])
	}
}

func TestAccountManagerUpdateAccountRollsBackOnTokenCommitFailure(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "old-client"}}}, t)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", "client_secret", "old-secret")
	tokenStore := &failingTokenStore{setErr: errors.New("token store down")}
	service := &stubAppService{serviceType: calendar.ServiceTypeGoogle, fields: []calendar.AccountField{{Key: "client_secret", Secret: true}}, discoveredCalendars: []calendar.Calendar{{ID: "primary", Name: "Primary"}}}
	manager := newTestAccountManager(loader, secretStore, tokenStore, service)

	_, err := manager.UpdateAccount(context.Background(), UpdateAccountInput{
		AccountID:        "work-id",
		Name:             "Work Updated",
		Settings:         map[string]string{"client_id": "new-client"},
		Secrets:          map[string]string{"client_secret": "new-secret"},
		CalendarSelector: &stubCalendarSelector{selected: []calendar.CalendarRef{{ID: "primary", Name: "Primary"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "persist OAuth token") {
		t.Fatalf("error = %v, want token persistence error", err)
	}
	if loader.cfg.Accounts[0].Name != "Work" {
		t.Fatalf("account changed after rollback = %+v", loader.cfg.Accounts[0])
	}
	secretValue, _ := secretStore.Get(context.Background(), "work-id", "client_secret")
	if secretValue != "old-secret" {
		t.Fatalf("secret after token rollback = %q, want old-secret", secretValue)
	}
}

func TestAccountManagerUpdateAccountReturnsWithoutMutatingStateOnUserAbort(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "old-client"}}}, t)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", "client_secret", "old-secret")
	tokenStore := tokenstore.NewInMemoryTokenStore()
	service := &stubAppService{serviceType: calendar.ServiceTypeGoogle, fields: []calendar.AccountField{{Key: "client_secret", Secret: true}}, discoveredCalendars: []calendar.Calendar{{ID: "primary", Name: "Primary"}}}
	manager := newTestAccountManager(loader, secretStore, tokenStore, service)

	_, err := manager.UpdateAccount(context.Background(), UpdateAccountInput{
		AccountID:        "work-id",
		Name:             "Work Updated",
		Settings:         map[string]string{"client_id": "new-client"},
		Secrets:          map[string]string{"client_secret": "new-secret"},
		CalendarSelector: &stubCalendarSelector{selectErr: context.Canceled},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if loader.cfg.Accounts[0].Name != "Work" {
		t.Fatalf("account changed after abort = %+v", loader.cfg.Accounts[0])
	}
	secretValue, _ := secretStore.Get(context.Background(), "work-id", "client_secret")
	if secretValue != "old-secret" {
		t.Fatalf("secret after abort = %q, want old-secret", secretValue)
	}
}

func TestAccountManagerUpdateAccountMergesExistingSecretsForPartialUpdate(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "client-id"}}}, t)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", "client_secret", "old-client-secret")
	_ = secretStore.Set(context.Background(), "work-id", "api_key", "existing-api-key")
	tokenStore := tokenstore.NewInMemoryTokenStore()
	service := &stubAppService{serviceType: calendar.ServiceTypeGoogle, fields: []calendar.AccountField{{Key: "client_secret", Secret: true}, {Key: "api_key", Secret: true}}, discoveredCalendars: []calendar.Calendar{{ID: "primary", Name: "Primary"}}}
	manager := newTestAccountManager(loader, secretStore, tokenStore, service)

	_, err := manager.UpdateAccount(context.Background(), UpdateAccountInput{
		AccountID:        "work-id",
		Name:             "Work",
		Settings:         map[string]string{"client_id": "client-id"},
		Secrets:          map[string]string{"client_secret": "new-client-secret"},
		CalendarSelector: &stubCalendarSelector{selected: []calendar.CalendarRef{{ID: "primary", Name: "Primary"}}},
	})
	if err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}
	clientSecret, _ := secretStore.Get(context.Background(), "work-id", "client_secret")
	apiKey, _ := secretStore.Get(context.Background(), "work-id", "api_key")
	if clientSecret != "new-client-secret" || apiKey != "existing-api-key" {
		t.Fatalf("stored secrets = (%q, %q), want updated client secret and preserved api key", clientSecret, apiKey)
	}
}

func TestStageSecretsAllowsMissingOptionalSecret(t *testing.T) {
	staged := secrets.NewStagedStore()
	fields := []calendar.AccountField{
		{Key: "required_secret", Secret: true, Required: true},
		{Key: "optional_secret", Secret: true, Required: false},
	}

	err := stageSecrets(context.Background(), staged, "work-id", fields, map[string]string{"required_secret": "required-value"})
	if err != nil {
		t.Fatalf("stageSecrets() error = %v", err)
	}

	requiredValue, err := staged.Get(context.Background(), "work-id", "required_secret")
	if err != nil {
		t.Fatalf("Get(required_secret) error = %v", err)
	}
	if requiredValue != "required-value" {
		t.Fatalf("requiredValue = %q, want required-value", requiredValue)
	}

	if _, err := staged.Get(context.Background(), "work-id", "optional_secret"); !errors.Is(err, secrets.ErrSecretNotFound) {
		t.Fatalf("Get(optional_secret) error = %v, want ErrSecretNotFound", err)
	}
}

func TestMergeSecretsIncludesEmptyOptionalSecretPlaceholder(t *testing.T) {
	fields := []calendar.AccountField{
		{Key: "required_secret", Secret: true, Required: true},
		{Key: "optional_secret", Secret: true, Required: false},
	}

	merged := mergeSecrets(
		map[string]secretSnapshot{"required_secret": {value: "stored-required", found: true}},
		map[string]string{},
		fields,
	)

	if merged["required_secret"] != "stored-required" {
		t.Fatalf("merged[required_secret] = %q, want stored-required", merged["required_secret"])
	}
	value, ok := merged["optional_secret"]
	if !ok {
		t.Fatal("merged does not include optional_secret placeholder")
	}
	if value != "" {
		t.Fatalf("merged[optional_secret] = %q, want empty string", value)
	}
}

func TestAccountManagerDeleteAccountRemovesConfigSecretsAndToken(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}, t)
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
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}, t)
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

func TestAccountManagerDeleteAccountRollsBackOnTokenCommitFailure(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}, t)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", "client_secret", "secret-value")
	tokenStore := &failingTokenStore{clearErr: errors.New("token store down")}
	service := &stubAppService{serviceType: calendar.ServiceTypeGoogle, fields: []calendar.AccountField{{Key: "client_secret", Secret: true}}}
	manager := newTestAccountManager(loader, secretStore, tokenStore, service)

	_, err := manager.DeleteAccount(context.Background(), DeleteAccountInput{AccountID: "work-id"})
	if err == nil || !strings.Contains(err.Error(), "persist token removal") {
		t.Fatalf("error = %v, want token removal persistence error", err)
	}
	if len(loader.cfg.Accounts) != 1 || loader.cfg.Accounts[0].Name != "Work" {
		t.Fatalf("accounts after rollback = %+v", loader.cfg.Accounts)
	}
	secretValue, _ := secretStore.Get(context.Background(), "work-id", "client_secret")
	if secretValue != "secret-value" {
		t.Fatalf("secret after token rollback = %q, want secret-value", secretValue)
	}
}

func TestAccountManagerLoginAccountCommitsReplacementToken(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}, t)
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

func TestAccountManagerLoginAccountReturnsTokenCommitFailure(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}, t)
	secretStore := secrets.NewInMemoryStore()
	tokenStore := &failingTokenStore{setErr: errors.New("token store down")}
	service := &stubAppService{serviceType: calendar.ServiceTypeGoogle}
	manager := newTestAccountManager(loader, secretStore, tokenStore, service)

	_, err := manager.LoginAccount(context.Background(), LoginAccountInput{AccountID: "work-id"})
	if err == nil || !strings.Contains(err.Error(), "persist OAuth token") {
		t.Fatalf("error = %v, want token persistence error", err)
	}
}

func TestJoinCommitRollbackErrorWrapsSecretRollbackError(t *testing.T) {
	err := joinCommitRollbackError("persist OAuth token", errors.New("commit failed"), nil, errors.New("secret rollback failed"))
	if err == nil {
		t.Fatal("joinCommitRollbackError() error = nil")
	}
	if !strings.Contains(err.Error(), "restore secret: secret rollback failed") {
		t.Fatalf("error = %q, want wrapped secret rollback context", err.Error())
	}
}

func TestJoinCommitRollbackErrorWrapsBothRollbackErrors(t *testing.T) {
	err := joinCommitRollbackError("persist OAuth token", errors.New("commit failed"), errors.New("config rollback failed"), errors.New("secret rollback failed"))
	if err == nil {
		t.Fatal("joinCommitRollbackError() error = nil")
	}
	if !strings.Contains(err.Error(), "restore config: config rollback failed") {
		t.Fatalf("error = %q, want wrapped config rollback context", err.Error())
	}
	if !strings.Contains(err.Error(), "restore secret: secret rollback failed") {
		t.Fatalf("error = %q, want wrapped secret rollback context", err.Error())
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
	fetchErrs           map[string]error // per-account fetch errors, keyed by account.ID
	providerErrs        map[string]error // per-account provider errors, keyed by account.ID
}

func (s *stubAppService) Type() calendar.ServiceType             { return s.serviceType }
func (s *stubAppService) DisplayName() string                    { return string(s.serviceType) }
func (s *stubAppService) AccountFields() []calendar.AccountField { return s.fields }
func (s *stubAppService) Provider(_ context.Context, account calendar.Account, _ secrets.Store) (providers.Provider, error) {
	if err, ok := s.providerErrs[account.ID]; ok {
		return nil, err
	}
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
func (s *stubAppService) FetchEvents(_ context.Context, account calendar.Account, _ calendar.EventQuery, _ *http.Client) ([]calendar.Event, error) {
	if err, ok := s.fetchErrs[account.ID]; ok {
		return nil, err
	}
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
	loader    *config.Loader
	cfg       *config.Config
	loadError error
	saveError error
}

func newMemoryConfigLoader(tb ...testing.TB) *memoryConfigLoader {
	return newMemoryConfigLoaderWithAccounts(nil, tb...)
}
func newMemoryConfigLoaderWithAccounts(accounts []calendar.Account, tb ...testing.TB) *memoryConfigLoader {
	var configPath string
	if len(tb) > 0 && tb[0] != nil {
		tb[0].Helper()
		configPath = filepath.Join(tb[0].TempDir(), "config.json")
	} else {
		tempDir, err := os.MkdirTemp("", "waybar-next-events-test-*")
		if err != nil {
			panic(fmt.Sprintf("create temp dir: %v", err))
		}
		configPath = filepath.Join(tempDir, "config.json")
	}

	cfg := &config.Config{Accounts: accounts}
	cfg.Normalize()

	loader := &memoryConfigLoader{
		loader: config.NewLoaderWithPath(configPath),
		cfg:    cloneConfig(cfg),
	}

	if len(cfg.Accounts) > 0 {
		if err := loader.loader.Save(cloneConfig(cfg)); err != nil {
			if len(tb) > 0 && tb[0] != nil {
				tb[0].Fatalf("seed config loader: %v", err)
			}
			panic(fmt.Sprintf("seed config loader: %v", err))
		}
		if err := loader.syncConfigState(); err != nil {
			if len(tb) > 0 && tb[0] != nil {
				tb[0].Fatalf("sync config state: %v", err)
			}
			panic(fmt.Sprintf("sync config state: %v", err))
		}
	}

	return loader
}
func (l *memoryConfigLoader) Load() (*config.Config, error) {
	if l.loadError != nil {
		return nil, l.loadError
	}
	return cloneConfig(l.cfg), nil
}
func (l *memoryConfigLoader) LoadOrEmpty() (*config.Config, error) {
	if l.loadError != nil {
		return nil, l.loadError
	}
	return cloneConfig(l.cfg), nil
}
func (l *memoryConfigLoader) Save(cfg *config.Config) error {
	if l.saveError != nil {
		return l.saveError
	}
	if err := l.loader.Save(cfg); err != nil {
		return err
	}
	l.cfg = cloneConfig(cfg)
	return nil
}
func (l *memoryConfigLoader) Snapshot() (config.Snapshot, error) {
	return l.loader.Snapshot()
}
func (l *memoryConfigLoader) RestoreSnapshot(snapshot config.Snapshot) error {
	if err := l.loader.RestoreSnapshot(snapshot); err != nil {
		return err
	}
	return l.syncConfigState()
}

func (l *memoryConfigLoader) syncConfigState() error {
	typedCfg, err := l.loader.Load()
	if err != nil {
		var notFound *config.ErrConfigNotFound
		if errors.As(err, &notFound) {
			l.cfg = &config.Config{}
			l.cfg.Normalize()
			return nil
		}
		return err
	}
	l.cfg = cloneConfig(typedCfg)

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

func newTestAccountManager(loader ConfigLoader, secretStore secrets.Store, tokenStore tokenstore.TokenStore, service Service) *AccountManager {
	registry := NewRegistry()
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

type failingTokenStore struct {
	setErr   error
	clearErr error
	values   map[string]*oauth2.Token
}

func (s *failingTokenStore) Set(_ context.Context, key string, token *oauth2.Token) error {
	if s.setErr != nil {
		return s.setErr
	}
	if s.values == nil {
		s.values = map[string]*oauth2.Token{}
	}
	s.values[key] = token
	return nil
}
func (s *failingTokenStore) Get(_ context.Context, key string) (*oauth2.Token, bool, error) {
	if s.values == nil {
		return nil, false, nil
	}
	token, found := s.values[key]
	return token, found, nil
}
func (s *failingTokenStore) Clear(_ context.Context, key string) error {
	if s.clearErr != nil {
		return s.clearErr
	}
	delete(s.values, key)
	return nil
}

type failingSecretStore struct {
	setErr    error
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
func (s *failingSecretStore) Set(_ context.Context, accountID, key, value string) error {
	if s.setErr != nil {
		return s.setErr
	}
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[accountID+"/"+key] = value
	return nil
}
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
