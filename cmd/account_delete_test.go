package cmd

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/auth"
	"github.com/hossainemruz/waybar-next-events/internal/auth/tokenstore"
	appcalendar "github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

func TestRunAccountDeleteRemovesSelectedAccount(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{
		{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}},
		{ID: "personal-id", Service: appcalendar.ServiceTypeGoogle, Name: "Personal", Settings: map[string]string{"client_id": "personal-client"}},
	})
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "work-secret")
	_ = secretStore.Set(context.Background(), "personal-id", googleClientSecretKey, "personal-secret")
	prompter := &stubAccountDeletePrompter{selectedAccountID: "personal-id", confirmed: true}

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountDeletePrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken:     func(context.Context, *auth.Authenticator, secrets.Store, *appconfig.Account) error { return nil },
	})
	if err != nil {
		t.Fatalf("runAccountDelete() error = %v", err)
	}

	loaded, _ := loader.Load()
	accounts := loaded.AccountsByService(appcalendar.ServiceTypeGoogle)
	if len(accounts) != 1 || accounts[0].Name != "Work" {
		t.Fatalf("remaining accounts = %+v, want only Work", accounts)
	}
	if _, err := secretStore.Get(context.Background(), "personal-id", googleClientSecretKey); !errors.Is(err, secrets.ErrSecretNotFound) {
		t.Fatalf("deleted account secret lookup error = %v, want ErrSecretNotFound", err)
	}
}

func TestDeleteGoogleAccountByID(t *testing.T) {
	accounts := []appconfig.Account{{ID: "a", Name: "Work"}, {ID: "b", Name: "Personal"}}
	updated := deleteAccountByID(accounts, "a")
	if len(updated) != 1 || updated[0].ID != "b" {
		t.Fatalf("deleteAccountByID() = %+v, want only b", updated)
	}
}

func TestRunAccountDeleteReturnsNoAccountsErrorWhenConfigMissing(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(filepath.Join(t.TempDir(), "missing.json"))
	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountDeletePrompter { return &stubAccountDeletePrompter{} },
		newSecretStore: func() secrets.Store { return secrets.NewInMemoryStore() },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken:     func(context.Context, *auth.Authenticator, secrets.Store, *appconfig.Account) error { return nil },
	})
	if !errors.Is(err, appconfig.ErrNoAccounts) {
		t.Fatalf("runAccountDelete() error = %v, want ErrNoAccounts", err)
	}
}

func TestRunAccountDeleteRollsBackConfigWhenTokenCommitFails(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}}})
	original := readFile(t, configPath)
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "work-secret")
	prompter := &stubAccountDeletePrompter{selectedAccountID: "work-id", confirmed: true}
	backingStore := &failingTokenStore{clearErr: errors.New("keyring unavailable")}

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountDeletePrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return backingStore },
		clearToken:     clearAccountToken,
	})
	if err == nil || !strings.Contains(err.Error(), "failed to persist OAuth token removal") {
		t.Fatalf("error = %v, want token persistence error", err)
	}
	assertConfigUnchanged(t, configPath, original)
	storedSecret, err := secretStore.Get(context.Background(), "work-id", googleClientSecretKey)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if storedSecret != "work-secret" {
		t.Fatalf("stored secret = %q, want work-secret after rollback", storedSecret)
	}
}

func TestRunAccountDeleteRollsBackConfigWhenSecretCommitFails(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}}})
	original := readFile(t, configPath)
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := &failingSecretStore{deleteErr: errors.New("keyring unavailable"), values: map[string]string{"work-id/" + googleClientSecretKey: "work-secret"}}
	prompter := &stubAccountDeletePrompter{selectedAccountID: "work-id", confirmed: true}

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountDeletePrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken:     clearAccountToken,
	})
	if err == nil || !strings.Contains(err.Error(), "failed to remove account secrets") {
		t.Fatalf("error = %v, want secret persistence error", err)
	}
	assertConfigUnchanged(t, configPath, original)
	storedSecret, getErr := secretStore.Get(context.Background(), "work-id", googleClientSecretKey)
	if getErr != nil {
		t.Fatalf("Get() error = %v", getErr)
	}
	if storedSecret != "work-secret" {
		t.Fatalf("stored secret = %q, want work-secret after rollback", storedSecret)
	}
}

func TestRunAccountDeleteLeavesConfigOnAbort(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}}})
	original := readFile(t, configPath)
	loader := appconfig.NewLoaderWithPath(configPath)
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "work-secret")

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader: func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountDeletePrompter {
			return &stubAccountDeletePrompter{selectionErr: huh.ErrUserAborted}
		},
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken: func(context.Context, *auth.Authenticator, secrets.Store, *appconfig.Account) error {
			t.Fatal("clearToken should not be called")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountDelete() error = %v, want nil", err)
	}
	assertConfigUnchanged(t, configPath, original)
}

func TestRunAccountDeleteLeavesConfigAndTokensUnchangedOnCancellation(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}}})
	original := readFile(t, configPath)
	loader := appconfig.NewLoaderWithPath(configPath)
	backingStore := tokenstore.NewInMemoryTokenStore()
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "work-secret")
	if err := backingStore.Set(context.Background(), tokenstore.TokenKey(string(appcalendar.ServiceTypeGoogle), "work-id"), &oauth2.Token{AccessToken: "work-token"}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader: func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountDeletePrompter {
			return &stubAccountDeletePrompter{selectedAccountID: "work-id", confirmErr: huh.ErrUserAborted}
		},
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return backingStore },
		clearToken: func(context.Context, *auth.Authenticator, secrets.Store, *appconfig.Account) error {
			t.Fatal("clearToken should not be called")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountDelete() error = %v, want nil", err)
	}
	assertConfigUnchanged(t, configPath, original)
	token, found, err := backingStore.Get(context.Background(), tokenstore.TokenKey(string(appcalendar.ServiceTypeGoogle), "work-id"))
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found || token == nil || token.AccessToken != "work-token" {
		t.Fatalf("stored token = %+v, found=%v, want original token", token, found)
	}
}

func TestRunAccountDeleteClearsOnlyDeletedAccountToken(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{
		{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}},
		{ID: "personal-id", Service: appcalendar.ServiceTypeGoogle, Name: "Personal", Settings: map[string]string{"client_id": "personal-client"}},
	})
	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountDeletePrompter{selectedAccountID: "personal-id", confirmed: true}
	backingStore := tokenstore.NewInMemoryTokenStore()
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "work-secret")
	_ = secretStore.Set(context.Background(), "personal-id", googleClientSecretKey, "personal-secret")
	if err := backingStore.Set(context.Background(), tokenstore.TokenKey(string(appcalendar.ServiceTypeGoogle), "work-id"), &oauth2.Token{AccessToken: "work-token"}); err != nil {
		t.Fatalf("Set(work token) error = %v", err)
	}
	if err := backingStore.Set(context.Background(), tokenstore.TokenKey(string(appcalendar.ServiceTypeGoogle), "personal-id"), &oauth2.Token{AccessToken: "personal-token"}); err != nil {
		t.Fatalf("Set(personal token) error = %v", err)
	}

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader:      func() *appconfig.Loader { return loader },
		newPrompter:    func(*cobra.Command) accountDeletePrompter { return prompter },
		newSecretStore: func() secrets.Store { return secretStore },
		newTokenStore:  func() tokenstore.TokenStore { return backingStore },
		clearToken:     clearAccountToken,
	})
	if err != nil {
		t.Fatalf("runAccountDelete() error = %v", err)
	}
	workToken, found, err := backingStore.Get(context.Background(), tokenstore.TokenKey(string(appcalendar.ServiceTypeGoogle), "work-id"))
	if err != nil {
		t.Fatalf("Get(work token) error = %v", err)
	}
	if !found || workToken == nil || workToken.AccessToken != "work-token" {
		t.Fatalf("work token = %+v, found=%v, want preserved token", workToken, found)
	}
	personalToken, found, err := backingStore.Get(context.Background(), tokenstore.TokenKey(string(appcalendar.ServiceTypeGoogle), "personal-id"))
	if err != nil {
		t.Fatalf("Get(personal token) error = %v", err)
	}
	if found || personalToken != nil {
		t.Fatalf("personal token = %+v, found=%v, want cleared token", personalToken, found)
	}
	if _, err := secretStore.Get(context.Background(), "personal-id", googleClientSecretKey); !errors.Is(err, secrets.ErrSecretNotFound) {
		t.Fatalf("personal secret lookup error = %v, want ErrSecretNotFound", err)
	}
}

func TestRunAccountDeleteReturnsMissingSecretError(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}}})
	loader := appconfig.NewLoaderWithPath(configPath)

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader: func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountDeletePrompter {
			return &stubAccountDeletePrompter{selectedAccountID: "work-id", confirmed: true}
		},
		newSecretStore: func() secrets.Store { return secrets.NewInMemoryStore() },
		newTokenStore:  func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken:     clearAccountToken,
	})
	if err == nil || !strings.Contains(err.Error(), "missing stored secret") {
		t.Fatalf("error = %v, want missing stored secret error", err)
	}
}

type stubAccountDeletePrompter struct {
	selectedAccountID    string
	selectionErr         error
	confirmed            bool
	confirmErr           error
	confirmedAccountName string
}

func (s *stubAccountDeletePrompter) PromptAccountSelection(context.Context, *appconfig.Config) (string, error) {
	if s.selectionErr != nil {
		return "", s.selectionErr
	}
	return s.selectedAccountID, nil
}

func (s *stubAccountDeletePrompter) PromptDeleteConfirmation(_ context.Context, accountName string) (bool, error) {
	s.confirmedAccountName = accountName
	if s.confirmErr != nil {
		return false, s.confirmErr
	}
	return s.confirmed, nil
}
