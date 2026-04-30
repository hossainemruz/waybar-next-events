package cmd

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	appcalendar "github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/pkg/auth"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/tokenstore"
)

func TestRunAccountLoginLogsIntoSelectedAccount(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{
		{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client", "client_secret": "work-secret"}},
		{ID: "personal-id", Service: appcalendar.ServiceTypeGoogle, Name: "Personal", Settings: map[string]string{"client_id": "personal-client", "client_secret": "personal-secret"}},
	})
	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountLoginPrompter{selectedAccountID: "personal-id"}
	backingStore := tokenstore.NewInMemoryTokenStore()
	stdout := &strings.Builder{}
	cmd := newTestCommand()
	cmd.SetOut(stdout)

	var loggedInAccount *appconfig.Account
	err := runAccountLogin(cmd, accountLoginDependencies{
		newLoader:        func() *appconfig.Loader { return loader },
		newPrompter:      func(*cobra.Command) accountLoginPrompter { return prompter },
		newAuthenticator: func() *auth.Authenticator { return auth.NewAuthenticator(backingStore) },
		loginAccount: func(ctx context.Context, _ *auth.Authenticator, account *appconfig.Account) error {
			loggedInAccount = account
			return backingStore.Set(ctx, account.Setting("client_id"), &oauth2.Token{AccessToken: "new-token"})
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
		newAuthenticator: func() *auth.Authenticator { return auth.NewAuthenticator(tokenstore.NewInMemoryTokenStore()) },
		loginAccount:     func(context.Context, *auth.Authenticator, *appconfig.Account) error { return nil },
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
	_ = backingStore.Set(context.Background(), "work-client", &oauth2.Token{AccessToken: "existing-token"})

	err := runAccountLogin(newTestCommand(), accountLoginDependencies{
		newLoader: func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountLoginPrompter {
			return &stubAccountLoginPrompter{selectionErr: huh.ErrUserAborted}
		},
		newAuthenticator: func() *auth.Authenticator { return auth.NewAuthenticator(backingStore) },
		loginAccount: func(context.Context, *auth.Authenticator, *appconfig.Account) error {
			t.Fatal("loginAccount should not be called")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountLogin() error = %v, want nil", err)
	}
	assertConfigUnchanged(t, configPath, original)
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
