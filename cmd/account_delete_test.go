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
)

func TestRunAccountDeleteRemovesSelectedMiddleAccount(t *testing.T) {
	configPath := writeTestConfigFile(t, `{
		"google": {
			"name": "Google Calendar",
			"accounts": [
				{
					"name": "Work",
					"clientId": "work-client",
					"clientSecret": "work-secret",
					"calendars": []
				},
				{
					"name": "Personal",
					"clientId": "personal-client",
					"clientSecret": "personal-secret",
					"calendars": []
				},
				{
					"name": "Side Project",
					"clientId": "side-client",
					"clientSecret": "side-secret",
					"calendars": []
				}
			]
		}
	}`)

	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountDeletePrompter{
		selectedAccountName: "Personal",
		confirmed:           true,
	}

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountDeletePrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken: func(context.Context, *auth.Authenticator, *appconfig.GoogleAccount) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountDelete() error = %v", err)
	}

	if prompter.confirmedAccountName != "Personal" {
		t.Fatalf("confirmed account = %q, want %q", prompter.confirmedAccountName, "Personal")
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
	if googleCfg.Accounts[0].Name != "Work" {
		t.Fatalf("account[0].Name = %q, want %q", googleCfg.Accounts[0].Name, "Work")
	}
	if googleCfg.Accounts[1].Name != "Side Project" {
		t.Fatalf("account[1].Name = %q, want %q", googleCfg.Accounts[1].Name, "Side Project")
	}
}

func TestRunAccountDeletePreservesEmptyGoogleAccountsWhenDeletingLastAccount(t *testing.T) {
	configPath := writeTestConfigFile(t, `{
		"google": {
			"name": "Google Calendar",
			"accounts": [
				{
					"name": "Work",
					"clientId": "work-client",
					"clientSecret": "work-secret",
					"calendars": []
				}
			]
		}
	}`)

	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountDeletePrompter{
		selectedAccountName: "Work",
		confirmed:           true,
	}

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountDeletePrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken: func(context.Context, *auth.Authenticator, *appconfig.GoogleAccount) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountDelete() error = %v", err)
	}

	loaded, err := loader.Load()
	if err != nil {
		t.Fatalf("loader.Load() error = %v", err)
	}
	if loaded.Google == nil {
		t.Fatal("loaded.Google = nil, want preserved google section")
	}
	if loaded.Google.Accounts == nil {
		t.Fatal("loaded.Google.Accounts = nil, want empty slice")
	}
	if len(loaded.Google.Accounts) != 0 {
		t.Fatalf("accounts length = %d, want 0", len(loaded.Google.Accounts))
	}
	if loaded.Google.Name != "Google Calendar" {
		t.Fatalf("google name = %q, want %q", loaded.Google.Name, "Google Calendar")
	}
}

func TestRunAccountDeleteLeavesConfigAndTokensUnchangedOnCancellation(t *testing.T) {
	configPath := writeTestConfigFile(t, `{
		"google": {
			"name": "Google Calendar",
			"accounts": [
				{
					"name": "Work",
					"clientId": "work-client",
					"clientSecret": "work-secret",
					"calendars": []
				}
			]
		}
	}`)
	original := readFile(t, configPath)

	t.Run("confirm false", func(t *testing.T) {
		loader := appconfig.NewLoaderWithPath(configPath)
		backingStore := tokenstore.NewInMemoryTokenStore()
		if err := backingStore.Set(context.Background(), "work-client", &oauth2.Token{AccessToken: "work-token"}); err != nil {
			t.Fatalf("backingStore.Set() error = %v", err)
		}

		prompter := &stubAccountDeletePrompter{
			selectedAccountName: "Work",
			confirmed:           false,
		}

		err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
			newLoader:     func() *appconfig.Loader { return loader },
			newPrompter:   func(*cobra.Command) accountDeletePrompter { return prompter },
			newTokenStore: func() tokenstore.TokenStore { return backingStore },
			clearToken: func(context.Context, *auth.Authenticator, *appconfig.GoogleAccount) error {
				t.Fatal("clearToken should not be called when deletion is not confirmed")
				return nil
			},
		})
		if err != nil {
			t.Fatalf("runAccountDelete() error = %v, want nil", err)
		}

		assertConfigUnchanged(t, configPath, original)
		token, found, err := backingStore.Get(context.Background(), "work-client")
		if err != nil {
			t.Fatalf("backingStore.Get() error = %v", err)
		}
		if !found || token == nil || token.AccessToken != "work-token" {
			t.Fatalf("stored token after cancellation = %+v, found=%v, want original token", token, found)
		}
	})

	t.Run("selection prompt aborted", func(t *testing.T) {
		if err := os.WriteFile(configPath, original, appconfig.ConfigFilePermission); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}

		loader := appconfig.NewLoaderWithPath(configPath)
		backingStore := tokenstore.NewInMemoryTokenStore()
		if err := backingStore.Set(context.Background(), "work-client", &oauth2.Token{AccessToken: "work-token"}); err != nil {
			t.Fatalf("backingStore.Set() error = %v", err)
		}

		prompter := &stubAccountDeletePrompter{
			selectionErr: huh.ErrUserAborted,
		}

		err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
			newLoader:     func() *appconfig.Loader { return loader },
			newPrompter:   func(*cobra.Command) accountDeletePrompter { return prompter },
			newTokenStore: func() tokenstore.TokenStore { return backingStore },
			clearToken: func(context.Context, *auth.Authenticator, *appconfig.GoogleAccount) error {
				t.Fatal("clearToken should not be called when selection is aborted")
				return nil
			},
		})
		if err != nil {
			t.Fatalf("runAccountDelete() error = %v, want nil", err)
		}

		assertConfigUnchanged(t, configPath, original)
		token, found, err := backingStore.Get(context.Background(), "work-client")
		if err != nil {
			t.Fatalf("backingStore.Get() error = %v", err)
		}
		if !found || token == nil || token.AccessToken != "work-token" {
			t.Fatalf("stored token after selection abort = %+v, found=%v, want original token", token, found)
		}
	})

	t.Run("confirm prompt aborted", func(t *testing.T) {
		if err := os.WriteFile(configPath, original, appconfig.ConfigFilePermission); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}

		loader := appconfig.NewLoaderWithPath(configPath)
		backingStore := tokenstore.NewInMemoryTokenStore()
		if err := backingStore.Set(context.Background(), "work-client", &oauth2.Token{AccessToken: "work-token"}); err != nil {
			t.Fatalf("backingStore.Set() error = %v", err)
		}

		prompter := &stubAccountDeletePrompter{
			selectedAccountName: "Work",
			confirmErr:          huh.ErrUserAborted,
		}

		err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
			newLoader:     func() *appconfig.Loader { return loader },
			newPrompter:   func(*cobra.Command) accountDeletePrompter { return prompter },
			newTokenStore: func() tokenstore.TokenStore { return backingStore },
			clearToken: func(context.Context, *auth.Authenticator, *appconfig.GoogleAccount) error {
				t.Fatal("clearToken should not be called when confirmation is aborted")
				return nil
			},
		})
		if err != nil {
			t.Fatalf("runAccountDelete() error = %v, want nil", err)
		}

		assertConfigUnchanged(t, configPath, original)
		token, found, err := backingStore.Get(context.Background(), "work-client")
		if err != nil {
			t.Fatalf("backingStore.Get() error = %v", err)
		}
		if !found || token == nil || token.AccessToken != "work-token" {
			t.Fatalf("stored token after abort = %+v, found=%v, want original token", token, found)
		}
	})
}

func TestDeleteGoogleAccountAtReturnsOriginalSliceForInvalidIndex(t *testing.T) {
	accounts := []appconfig.GoogleAccount{{Name: "Work"}, {Name: "Personal"}}

	t.Run("negative index", func(t *testing.T) {
		updated := deleteGoogleAccountAt(accounts, -1)
		if len(updated) != len(accounts) {
			t.Fatalf("len(updated) = %d, want %d", len(updated), len(accounts))
		}
		for i := range accounts {
			if updated[i].Name != accounts[i].Name {
				t.Fatalf("updated[%d].Name = %q, want %q", i, updated[i].Name, accounts[i].Name)
			}
		}
	})

	t.Run("index too large", func(t *testing.T) {
		updated := deleteGoogleAccountAt(accounts, len(accounts))
		if len(updated) != len(accounts) {
			t.Fatalf("len(updated) = %d, want %d", len(updated), len(accounts))
		}
		for i := range accounts {
			if updated[i].Name != accounts[i].Name {
				t.Fatalf("updated[%d].Name = %q, want %q", i, updated[i].Name, accounts[i].Name)
			}
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		updated := deleteGoogleAccountAt(nil, 0)
		if updated != nil {
			t.Fatalf("updated = %#v, want nil", updated)
		}
	})
}

func TestRunAccountDeleteClearsOnlyDeletedAccountToken(t *testing.T) {
	configPath := writeTestConfigFile(t, `{
		"google": {
			"name": "Google Calendar",
			"accounts": [
				{
					"name": "Work",
					"clientId": "work-client",
					"clientSecret": "work-secret",
					"calendars": []
				},
				{
					"name": "Personal",
					"clientId": "personal-client",
					"clientSecret": "personal-secret",
					"calendars": []
				}
			]
		}
	}`)

	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountDeletePrompter{
		selectedAccountName: "Personal",
		confirmed:           true,
	}
	backingStore := tokenstore.NewInMemoryTokenStore()
	if err := backingStore.Set(context.Background(), "work-client", &oauth2.Token{AccessToken: "work-token"}); err != nil {
		t.Fatalf("backingStore.Set(work-client) error = %v", err)
	}
	if err := backingStore.Set(context.Background(), "personal-client", &oauth2.Token{AccessToken: "personal-token"}); err != nil {
		t.Fatalf("backingStore.Set(personal-client) error = %v", err)
	}

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountDeletePrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return backingStore },
		clearToken:    clearGoogleAccountToken,
	})
	if err != nil {
		t.Fatalf("runAccountDelete() error = %v", err)
	}

	workToken, found, err := backingStore.Get(context.Background(), "work-client")
	if err != nil {
		t.Fatalf("backingStore.Get(work-client) error = %v", err)
	}
	if !found || workToken == nil || workToken.AccessToken != "work-token" {
		t.Fatalf("work token after delete = %+v, found=%v, want preserved token", workToken, found)
	}

	personalToken, found, err := backingStore.Get(context.Background(), "personal-client")
	if err != nil {
		t.Fatalf("backingStore.Get(personal-client) error = %v", err)
	}
	if found || personalToken != nil {
		t.Fatalf("personal token after delete = %+v, found=%v, want cleared token", personalToken, found)
	}
}

func TestRunAccountDeleteReturnsNoAccountsErrorWhenConfigMissing(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(filepath.Join(t.TempDir(), "missing-config.json"))

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountDeletePrompter { return &stubAccountDeletePrompter{} },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken: func(context.Context, *auth.Authenticator, *appconfig.GoogleAccount) error {
			return nil
		},
	})
	if !errors.Is(err, appconfig.ErrNoAccounts) {
		t.Fatalf("runAccountDelete() error = %v, want ErrNoAccounts", err)
	}
	if err.Error() != "no accounts configured: add an account first" {
		t.Fatalf("error = %q, want no-accounts hint", err.Error())
	}
}

func TestRunAccountDeleteReturnsNoAccountsErrorWhenConfigHasNoAccounts(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeTestConfigFile(t, `{
		"google": {
			"name": "Google Calendar",
			"accounts": []
		}
	}`))

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountDeletePrompter { return &stubAccountDeletePrompter{} },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken: func(context.Context, *auth.Authenticator, *appconfig.GoogleAccount) error {
			return nil
		},
	})
	if !errors.Is(err, appconfig.ErrNoAccounts) {
		t.Fatalf("runAccountDelete() error = %v, want ErrNoAccounts", err)
	}
}

func TestRunAccountDeleteReturnsMalformedConfigError(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeTestConfigFile(t, `{invalid json}`))

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountDeletePrompter { return &stubAccountDeletePrompter{} },
		newTokenStore: func() tokenstore.TokenStore { return tokenstore.NewInMemoryTokenStore() },
		clearToken: func(context.Context, *auth.Authenticator, *appconfig.GoogleAccount) error {
			return nil
		},
	})
	if err == nil {
		t.Fatal("runAccountDelete() error = nil, want error")
	}

	want := "failed to load config: failed to parse config file: invalid character 'i' looking for beginning of object key string"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestRunAccountDeleteRollsBackConfigWhenTokenCommitFails(t *testing.T) {
	configPath := writeTestConfigFile(t, `{
		"google": {
			"name": "Google Calendar",
			"accounts": [
				{
					"name": "Work",
					"clientId": "work-client",
					"clientSecret": "work-secret",
					"calendars": []
				}
			]
		}
	}`)
	original := readFile(t, configPath)

	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountDeletePrompter{
		selectedAccountName: "Work",
		confirmed:           true,
	}
	backingStore := &failingTokenStore{clearErr: errors.New("keyring unavailable")}

	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader:     func() *appconfig.Loader { return loader },
		newPrompter:   func(*cobra.Command) accountDeletePrompter { return prompter },
		newTokenStore: func() tokenstore.TokenStore { return backingStore },
		clearToken:    clearGoogleAccountToken,
	})
	if err == nil {
		t.Fatal("runAccountDelete() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to persist OAuth token removal") {
		t.Fatalf("error = %q, want token persistence error", err.Error())
	}

	assertConfigUnchanged(t, configPath, original)
}

type stubAccountDeletePrompter struct {
	selectedAccountName  string
	selectionErr         error
	confirmed            bool
	confirmErr           error
	confirmedAccountName string
}

func (s *stubAccountDeletePrompter) PromptAccountSelection(context.Context, *appconfig.GoogleCalendar) (string, error) {
	if s.selectionErr != nil {
		return "", s.selectionErr
	}
	return s.selectedAccountName, nil
}

func (s *stubAccountDeletePrompter) PromptDeleteConfirmation(_ context.Context, accountName string) (bool, error) {
	s.confirmedAccountName = accountName
	if s.confirmErr != nil {
		return false, s.confirmErr
	}
	return s.confirmed, nil
}
