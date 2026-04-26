package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/url"
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

func TestRunAccountLoginLogsIntoSelectedAccount(t *testing.T) {
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
	original := readFile(t, configPath)

	loader := appconfig.NewLoaderWithPath(configPath)
	prompter := &stubAccountLoginPrompter{selectedAccountName: "Personal"}
	backingStore := tokenstore.NewInMemoryTokenStore()
	stdout := &strings.Builder{}
	cmd := newTestCommand()
	cmd.SetOut(stdout)

	var loggedInAccount *appconfig.GoogleAccount
	err := runAccountLogin(cmd, accountLoginDependencies{
		newLoader:        func() *appconfig.Loader { return loader },
		newPrompter:      func(*cobra.Command) accountLoginPrompter { return prompter },
		newAuthenticator: func() *auth.Authenticator { return auth.NewAuthenticator(backingStore) },
		loginAccount: func(ctx context.Context, authenticator *auth.Authenticator, account *appconfig.GoogleAccount) error {
			loggedInAccount = account
			return backingStore.Set(ctx, account.ClientID, &oauth2.Token{AccessToken: "new-token"})
		},
	})
	if err != nil {
		t.Fatalf("runAccountLogin() error = %v", err)
	}

	if loggedInAccount == nil {
		t.Fatal("expected selected account to be passed to loginAccount")
	}
	if loggedInAccount.Name != "Personal" || loggedInAccount.ClientID != "personal-client" {
		t.Fatalf("loggedInAccount = %+v, want Personal account", loggedInAccount)
	}

	token, found, err := backingStore.Get(context.Background(), "personal-client")
	if err != nil {
		t.Fatalf("backingStore.Get() error = %v", err)
	}
	if !found || token == nil || token.AccessToken != "new-token" {
		t.Fatalf("stored token = %+v, found=%v, want new-token", token, found)
	}

	assertConfigUnchanged(t, configPath, original)

	if stdout.String() != "Logged in to account \"Personal\".\n" {
		t.Fatalf("stdout = %q, want success message", stdout.String())
	}
}

func TestRunAccountLoginReturnsNoAccountsErrorWhenConfigMissing(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(filepath.Join(t.TempDir(), "missing-config.json"))

	err := runAccountLogin(newTestCommand(), accountLoginDependencies{
		newLoader:        func() *appconfig.Loader { return loader },
		newPrompter:      func(*cobra.Command) accountLoginPrompter { return &stubAccountLoginPrompter{} },
		newAuthenticator: func() *auth.Authenticator { return auth.NewAuthenticator(tokenstore.NewInMemoryTokenStore()) },
		loginAccount: func(context.Context, *auth.Authenticator, *appconfig.GoogleAccount) error {
			return nil
		},
	})
	if !errors.Is(err, appconfig.ErrNoAccounts) {
		t.Fatalf("runAccountLogin() error = %v, want ErrNoAccounts", err)
	}
	if err.Error() != "no accounts configured: add an account first" {
		t.Fatalf("error = %q, want no-accounts hint", err.Error())
	}
}

func TestRunAccountLoginReturnsNoAccountsErrorWhenConfigHasNoAccounts(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeTestConfigFile(t, `{
		"google": {
			"name": "Google Calendar",
			"accounts": []
		}
	}`))

	err := runAccountLogin(newTestCommand(), accountLoginDependencies{
		newLoader:        func() *appconfig.Loader { return loader },
		newPrompter:      func(*cobra.Command) accountLoginPrompter { return &stubAccountLoginPrompter{} },
		newAuthenticator: func() *auth.Authenticator { return auth.NewAuthenticator(tokenstore.NewInMemoryTokenStore()) },
		loginAccount: func(context.Context, *auth.Authenticator, *appconfig.GoogleAccount) error {
			return nil
		},
	})
	if !errors.Is(err, appconfig.ErrNoAccounts) {
		t.Fatalf("runAccountLogin() error = %v, want ErrNoAccounts", err)
	}
}

func TestRunAccountLoginReturnsMalformedConfigError(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeTestConfigFile(t, `{invalid json}`))

	err := runAccountLogin(newTestCommand(), accountLoginDependencies{
		newLoader:        func() *appconfig.Loader { return loader },
		newPrompter:      func(*cobra.Command) accountLoginPrompter { return &stubAccountLoginPrompter{} },
		newAuthenticator: func() *auth.Authenticator { return auth.NewAuthenticator(tokenstore.NewInMemoryTokenStore()) },
		loginAccount: func(context.Context, *auth.Authenticator, *appconfig.GoogleAccount) error {
			return nil
		},
	})
	if err == nil {
		t.Fatal("runAccountLogin() error = nil, want error")
	}

	want := "failed to load config: failed to parse config file: invalid character 'i' looking for beginning of object key string"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestRunAccountLoginPreservesConfigAndTokenOnLoginFailure(t *testing.T) {
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
	prompter := &stubAccountLoginPrompter{selectedAccountName: "Work"}
	backingStore := tokenstore.NewInMemoryTokenStore()
	if err := backingStore.Set(context.Background(), "work-client", &oauth2.Token{AccessToken: "existing-token"}); err != nil {
		t.Fatalf("backingStore.Set() error = %v", err)
	}

	err := runAccountLogin(newTestCommand(), accountLoginDependencies{
		newLoader:   func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountLoginPrompter { return prompter },
		newAuthenticator: func() *auth.Authenticator {
			return auth.NewAuthenticator(backingStore, auth.WithBrowserOpener(func(authURL string) error {
				parsedURL, err := url.Parse(authURL)
				if err != nil {
					return err
				}

				if parsedURL.Query().Get("client_id") != "work-client" {
					return fmt.Errorf("unexpected client_id: %q", parsedURL.Query().Get("client_id"))
				}
				if parsedURL.Query().Get("redirect_uri") != appconfig.DefaultCallbackURL {
					return fmt.Errorf("unexpected redirect_uri: %q", parsedURL.Query().Get("redirect_uri"))
				}

				return errors.New("browser unavailable")
			}))
		},
		loginAccount: loginGoogleAccount,
	})
	if err == nil {
		t.Fatal("runAccountLogin() error = nil, want error")
	}
	if !strings.Contains(err.Error(), `failed to log in to account "Work": failed to open browser: browser unavailable`) {
		t.Fatalf("error = %q, want wrapped login failure", err.Error())
	}
	if !strings.Contains(err.Error(), "client_id=work-client") {
		t.Fatalf("error = %q, want auth URL with selected client ID", err.Error())
	}

	assertConfigUnchanged(t, configPath, original)

	token, found, err := backingStore.Get(context.Background(), "work-client")
	if err != nil {
		t.Fatalf("backingStore.Get() error = %v", err)
	}
	if !found || token == nil || token.AccessToken != "existing-token" {
		t.Fatalf("stored token after failure = %+v, found=%v, want existing-token", token, found)
	}
}

func TestRunAccountLoginPreservesConfigOnUserAbort(t *testing.T) {
	tests := []struct {
		name         string
		selectionErr error
	}{
		{name: "selection user aborted", selectionErr: huh.ErrUserAborted},
		{name: "selection context canceled", selectionErr: context.Canceled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			prompter := &stubAccountLoginPrompter{selectionErr: tt.selectionErr}
			backingStore := tokenstore.NewInMemoryTokenStore()
			if err := backingStore.Set(context.Background(), "work-client", &oauth2.Token{AccessToken: "existing-token"}); err != nil {
				t.Fatalf("backingStore.Set() error = %v", err)
			}

			err := runAccountLogin(newTestCommand(), accountLoginDependencies{
				newLoader:        func() *appconfig.Loader { return loader },
				newPrompter:      func(*cobra.Command) accountLoginPrompter { return prompter },
				newAuthenticator: func() *auth.Authenticator { return auth.NewAuthenticator(backingStore) },
				loginAccount: func(context.Context, *auth.Authenticator, *appconfig.GoogleAccount) error {
					t.Fatal("loginAccount should not be called when selection is aborted")
					return nil
				},
			})
			if err != nil {
				t.Fatalf("runAccountLogin() error = %v, want nil", err)
			}

			assertConfigUnchanged(t, configPath, original)

			token, found, err := backingStore.Get(context.Background(), "work-client")
			if err != nil {
				t.Fatalf("backingStore.Get() error = %v", err)
			}
			if !found || token == nil || token.AccessToken != "existing-token" {
				t.Fatalf("stored token after abort = %+v, found=%v, want existing-token", token, found)
			}
		})
	}
}

type stubAccountLoginPrompter struct {
	selectedAccountName string
	selectionErr        error
}

func (s *stubAccountLoginPrompter) PromptAccountSelection(context.Context, *appconfig.GoogleCalendar) (string, error) {
	if s.selectionErr != nil {
		return "", s.selectionErr
	}
	return s.selectedAccountName, nil
}

var _ accountLoginPrompter = (*stubAccountLoginPrompter)(nil)
