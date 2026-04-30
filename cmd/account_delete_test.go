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

func TestRunAccountDeleteRemovesSelectedAccount(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{
		{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client", "client_secret": "work-secret"}},
		{ID: "personal-id", Service: appcalendar.ServiceTypeGoogle, Name: "Personal", Settings: map[string]string{"client_id": "personal-client", "client_secret": "personal-secret"}},
	})
	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountDeletePrompter{selectedAccountID: "personal-id", confirmed: true}

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountDeletePrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken:    func(context.Context, *auth.Authenticator, *appconfig.Account) error { return nil },
	})
	if err != nil {
		t.Fatalf("runAccountDelete() error = %v", err)
	}

	loaded, _ := loader.Load()
	accounts := loaded.AccountsByService(appcalendar.ServiceTypeGoogle)
	if len(accounts) != 1 || accounts[0].Name != "Work" {
		t.Fatalf("remaining accounts = %+v, want only Work", accounts)
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
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountDeletePrompter { return &stubAccountDeletePrompter{} },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken:    func(context.Context, *auth.Authenticator, *appconfig.Account) error { return nil },
	})
	if !errors.Is(err, appconfig.ErrNoAccounts) {
		t.Fatalf("runAccountDelete() error = %v, want ErrNoAccounts", err)
	}
}

func TestRunAccountDeleteRollsBackConfigWhenTokenCommitFails(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client", "client_secret": "work-secret"}}})
	original := readFile(t, configPath)
	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountDeletePrompter{selectedAccountID: "work-id", confirmed: true}
	backingStore := &failingTokenStore{clearErr: errors.New("keyring unavailable")}

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountDeletePrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return backingStore },
		clearToken:    clearAccountToken,
	})
	if err == nil || !strings.Contains(err.Error(), "failed to persist OAuth token removal") {
		t.Fatalf("error = %v, want token persistence error", err)
	}
	assertConfigUnchanged(t, configPath, original)
}

func TestRunAccountDeleteLeavesConfigOnAbort(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client"}}})
	original := readFile(t, configPath)
	loader := appconfig.NewLoaderWithPath(configPath)

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader: func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountDeletePrompter {
			return &stubAccountDeletePrompter{selectionErr: huh.ErrUserAborted}
		},
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken: func(context.Context, *auth.Authenticator, *appconfig.Account) error {
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
	configPath := writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client", "client_secret": "work-secret"}}})
	original := readFile(t, configPath)
	loader := appconfig.NewLoaderWithPath(configPath)
	backingStore := tokenstore.NewInMemoryTokenStore()
	if err := backingStore.Set(context.Background(), "work-client", &oauth2.Token{AccessToken: "work-token"}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader: func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountDeletePrompter {
			return &stubAccountDeletePrompter{selectedAccountID: "work-id", confirmErr: huh.ErrUserAborted}
		},
		newTokenStore: func() tokenstore.TokenStore { return backingStore },
		clearToken: func(context.Context, *auth.Authenticator, *appconfig.Account) error {
			t.Fatal("clearToken should not be called")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountDelete() error = %v, want nil", err)
	}
	assertConfigUnchanged(t, configPath, original)
	token, found, err := backingStore.Get(context.Background(), "work-client")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found || token == nil || token.AccessToken != "work-token" {
		t.Fatalf("stored token = %+v, found=%v, want original token", token, found)
	}
}

func TestRunAccountDeleteClearsOnlyDeletedAccountToken(t *testing.T) {
	configPath := writeGenericConfig(t, []appconfig.Account{
		{ID: "work-id", Service: appcalendar.ServiceTypeGoogle, Name: "Work", Settings: map[string]string{"client_id": "work-client", "client_secret": "work-secret"}},
		{ID: "personal-id", Service: appcalendar.ServiceTypeGoogle, Name: "Personal", Settings: map[string]string{"client_id": "personal-client", "client_secret": "personal-secret"}},
	})
	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountDeletePrompter{selectedAccountID: "personal-id", confirmed: true}
	backingStore := tokenstore.NewInMemoryTokenStore()
	if err := backingStore.Set(context.Background(), "work-client", &oauth2.Token{AccessToken: "work-token"}); err != nil {
		t.Fatalf("Set(work-client) error = %v", err)
	}
	if err := backingStore.Set(context.Background(), "personal-client", &oauth2.Token{AccessToken: "personal-token"}); err != nil {
		t.Fatalf("Set(personal-client) error = %v", err)
	}

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountDeletePrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return backingStore },
		clearToken:    clearAccountToken,
	})
	if err != nil {
		t.Fatalf("runAccountDelete() error = %v", err)
	}
	workToken, found, err := backingStore.Get(context.Background(), "work-client")
	if err != nil {
		t.Fatalf("Get(work-client) error = %v", err)
	}
	if !found || workToken == nil || workToken.AccessToken != "work-token" {
		t.Fatalf("work token = %+v, found=%v, want preserved token", workToken, found)
	}
	personalToken, found, err := backingStore.Get(context.Background(), "personal-client")
	if err != nil {
		t.Fatalf("Get(personal-client) error = %v", err)
	}
	if found || personalToken != nil {
		t.Fatalf("personal token = %+v, found=%v, want cleared token", personalToken, found)
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
