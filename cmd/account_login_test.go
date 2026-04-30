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

func TestRunAccountLoginLogsIntoSelectedAccount(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{
		{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}},
		{ID: "personal-id", Service: appcalendar.ServiceTypeGoogle, Name: "Personal", Settings: map[string]string{"client_id": "personal-client"}},
	})
	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountLoginPrompter{selectedAccountID: "personal-id"}
	backingStore := tokenstore.NewInMemoryTokenStore()
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "work-secret")
	_ = secretStore.Set(context.Background(), "personal-id", googleClientSecretKey, "personal-secret")
	stdout := &strings.Builder{}
	cmd := newTestCommand()
	cmd.SetOut(stdout)

	var loggedInAccount *appconfig.Account
	err := runAccountLogin(cmd, accountLoginDependencies{
		newLoader:        func() *appconfig.Loader { return loader },
		newPrompter:      func(*cobra.Command) accountLoginPrompter { return prompter },
		newSecretStore:   func() secrets.Store { return secretStore },
		newAuthenticator: func() *auth.Authenticator { return auth.NewAuthenticator(backingStore) },
		loginAccount: func(ctx context.Context, _ *auth.Authenticator, secretStore secrets.Store, account *appconfig.Account) error {
			loggedInAccount = account
			secretValue, err := secretStore.Get(ctx, account.ID, googleClientSecretKey)
			if err != nil {
				return err
			}
			if secretValue != "personal-secret" {
				return errors.New("unexpected secret value")
			}
			return backingStore.Set(ctx, tokenstore.TokenKey(string(appcalendar.ServiceTypeGoogle), account.ID), &oauth2.Token{AccessToken: "new-token"})
		},
	})
	if err != nil {
		t.Fatalf("runAccountLogin() error = %v", err)
	}
	if loggedInAccount == nil || loggedInAccount.Name != "Personal" {
		t.Fatalf("loggedInAccount = %+v, want Personal", loggedInAccount)
	}
}

func TestRunAccountLoginReturnsNoAccountsErrorWhenConfigMissing(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(filepath.Join(t.TempDir(), "missing-config.json"))
	err := runAccountLogin(newTestCommand(), accountLoginDependencies{
		newLoader:        func() *appconfig.Loader { return loader },
		newPrompter:      func(*cobra.Command) accountLoginPrompter { return &stubAccountLoginPrompter{} },
		newSecretStore:   func() secrets.Store { return secrets.NewInMemoryStore() },
		newAuthenticator: func() *auth.Authenticator { return auth.NewAuthenticator(tokenstore.NewInMemoryTokenStore()) },
		loginAccount:     func(context.Context, *auth.Authenticator, secrets.Store, *appconfig.Account) error { return nil },
	})
	if !errors.Is(err, appconfig.ErrNoAccounts) {
		t.Fatalf("runAccountLogin() error = %v, want ErrNoAccounts", err)
	}
}

func TestRunAccountLoginPreservesConfigOnUserAbort(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}}})
	original := readFile(t, configPath)
	loader := appconfig.NewLoaderWithPath(configPath)
	backingStore := tokenstore.NewInMemoryTokenStore()
	secretStore := secrets.NewInMemoryStore()
	_ = secretStore.Set(context.Background(), "work-id", googleClientSecretKey, "work-secret")
	_ = backingStore.Set(context.Background(), tokenstore.TokenKey(string(appcalendar.ServiceTypeGoogle), "work-id"), &oauth2.Token{AccessToken: "existing-token"})

	err := runAccountLogin(newTestCommand(), accountLoginDependencies{
		newLoader: func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountLoginPrompter {
			return &stubAccountLoginPrompter{selectionErr: huh.ErrUserAborted}
		},
		newSecretStore:   func() secrets.Store { return secretStore },
		newAuthenticator: func() *auth.Authenticator { return auth.NewAuthenticator(backingStore) },
		loginAccount: func(context.Context, *auth.Authenticator, secrets.Store, *appconfig.Account) error {
			t.Fatal("loginAccount should not be called")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountLogin() error = %v, want nil", err)
	}
	assertConfigUnchanged(t, configPath, original)
}

func TestRunAccountLoginReturnsSecretError(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}}})
	loader := appconfig.NewLoaderWithPath(configPath)

	err := runAccountLogin(newTestCommand(), accountLoginDependencies{
		newLoader: func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountLoginPrompter {
			return &stubAccountLoginPrompter{selectedAccountID: "work-id"}
		},
		newSecretStore: func() secrets.Store { return secrets.NewInMemoryStore() },
		newAuthenticator: func() *auth.Authenticator {
			return auth.NewAuthenticator(tokenstore.NewInMemoryTokenStore())
		},
		loginAccount: loginGoogleAccount,
	})
	if err == nil || !strings.Contains(err.Error(), "missing stored secret") {
		t.Fatalf("error = %v, want missing stored secret error", err)
	}
}

type stubAccountLoginPrompter struct {
	selectedAccountID string
	selectionErr      error
}

func (s *stubAccountLoginPrompter) PromptAccountSelection(context.Context, *appconfig.Config) (string, error) {
	if s.selectionErr != nil {
		return "", s.selectionErr
	}
	return s.selectedAccountID, nil
}
