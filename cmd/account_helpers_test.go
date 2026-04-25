package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
)

func TestAccountCommandRegistration(t *testing.T) {
	if rootCmd.Commands() == nil {
		t.Fatal("root command has no subcommands")
	}

	accountCommand, _, err := rootCmd.Find([]string{"account"})
	if err != nil {
		t.Fatalf("rootCmd.Find(account) error = %v", err)
	}
	if accountCommand != accountCmd {
		t.Fatal("account command is not registered on root command")
	}

	for _, subcommandName := range []string{"add", "update", "delete", "login"} {
		subcommand, _, err := accountCmd.Find([]string{subcommandName})
		if err != nil {
			t.Fatalf("accountCmd.Find(%s) error = %v", subcommandName, err)
		}
		if subcommand.Name() != subcommandName {
			t.Fatalf("subcommand name = %s, want %s", subcommand.Name(), subcommandName)
		}
	}
}

func TestHasNoAccounts(t *testing.T) {
	t.Run("NilGoogleConfig", func(t *testing.T) {
		if !hasNoAccounts(nil) {
			t.Fatal("hasNoAccounts(nil) = false, want true")
		}
	})

	t.Run("EmptyAccounts", func(t *testing.T) {
		googleCfg := &appconfig.GoogleCalendar{Name: "Google Calendar", Accounts: []appconfig.GoogleAccount{}}
		if !hasNoAccounts(googleCfg) {
			t.Fatal("hasNoAccounts(empty) = false, want true")
		}
	})

	t.Run("ConfiguredAccounts", func(t *testing.T) {
		googleCfg := &appconfig.GoogleCalendar{
			Name:     "Google Calendar",
			Accounts: []appconfig.GoogleAccount{{Name: "Work", ClientID: "client-id"}},
		}
		if hasNoAccounts(googleCfg) {
			t.Fatal("hasNoAccounts(configured) = true, want false")
		}
	})
}

func TestEnsureHasAccounts(t *testing.T) {
	t.Run("ReturnsErrNoAccountsForEmptyAccounts", func(t *testing.T) {
		googleCfg := &appconfig.GoogleCalendar{Name: "Google Calendar", Accounts: []appconfig.GoogleAccount{}}

		err := ensureHasAccounts(googleCfg)
		if !errors.Is(err, appconfig.ErrNoAccounts) {
			t.Fatalf("ensureHasAccounts() error = %v, want ErrNoAccounts", err)
		}
		if err.Error() != "no accounts configured: add an account first" {
			t.Fatalf("error message = %q, want %q", err.Error(), "no accounts configured: add an account first")
		}
	})

	t.Run("ReturnsNilWhenAccountsExist", func(t *testing.T) {
		googleCfg := &appconfig.GoogleCalendar{
			Name:     "Google Calendar",
			Accounts: []appconfig.GoogleAccount{{Name: "Work", ClientID: "client-id"}},
		}

		if err := ensureHasAccounts(googleCfg); err != nil {
			t.Fatalf("ensureHasAccounts() error = %v, want nil", err)
		}
	})
}

func TestEnsureAccountNameAvailable(t *testing.T) {
	googleCfg := &appconfig.GoogleCalendar{
		Name: "Google Calendar",
		Accounts: []appconfig.GoogleAccount{
			{Name: "Work", ClientID: "work-client-id"},
			{Name: "Personal", ClientID: "personal-client-id"},
		},
	}

	t.Run("DuplicateName", func(t *testing.T) {
		err := ensureAccountNameAvailable(googleCfg, "Work")
		if !errors.Is(err, appconfig.ErrDuplicateAccountName) {
			t.Fatalf("ensureAccountNameAvailable() error = %v, want ErrDuplicateAccountName", err)
		}
	})

	t.Run("AvailableName", func(t *testing.T) {
		if err := ensureAccountNameAvailable(googleCfg, "Side Project"); err != nil {
			t.Fatalf("ensureAccountNameAvailable() error = %v, want nil", err)
		}
	})
}

func TestLoadGoogleConfig(t *testing.T) {
	t.Run("ReturnsMalformedConfigError", func(t *testing.T) {
		loader := appconfig.NewLoaderWithPath(writeTestConfigFile(t, `{invalid json}`))

		_, _, err := loadGoogleConfig(loader)
		if err == nil {
			t.Fatal("loadGoogleConfig() error = nil, want error")
		}
		if err.Error() != "failed to load config: failed to parse config file: invalid character 'i' looking for beginning of object key string" {
			t.Fatalf("unexpected error message: %q", err.Error())
		}
	})

	t.Run("ReturnsValidatedGoogleConfig", func(t *testing.T) {
		loader := appconfig.NewLoaderWithPath(writeTestConfigFile(t, `{
			"google": {
				"name": "Google Calendar",
				"accounts": [
					{
						"name": "Work",
						"clientId": "work-client-id"
					}
				]
			}
		}`))

		cfg, googleCfg, err := loadGoogleConfig(loader)
		if err != nil {
			t.Fatalf("loadGoogleConfig() error = %v", err)
		}
		if cfg == nil {
			t.Fatal("config = nil, want non-nil")
		}
		if googleCfg == nil {
			t.Fatal("google config = nil, want non-nil")
		}
		if len(googleCfg.Accounts) != 1 {
			t.Fatalf("accounts length = %d, want 1", len(googleCfg.Accounts))
		}
	})
}

func TestLoadGoogleConfigOrEmpty(t *testing.T) {
	t.Run("InitializesMissingConfigForFirstRun", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "missing.json")
		loader := appconfig.NewLoaderWithPath(configPath)

		cfg, googleCfg, err := loadGoogleConfigOrEmpty(loader)
		if err != nil {
			t.Fatalf("loadGoogleConfigOrEmpty() error = %v", err)
		}
		if cfg == nil {
			t.Fatal("config = nil, want non-nil")
		}
		if googleCfg == nil {
			t.Fatal("google config = nil, want non-nil")
		}
		if googleCfg.Name != "Google Calendar" {
			t.Fatalf("google config name = %q, want %q", googleCfg.Name, "Google Calendar")
		}
		if len(googleCfg.Accounts) != 0 {
			t.Fatalf("accounts length = %d, want 0", len(googleCfg.Accounts))
		}
	})

	t.Run("PassesThroughMalformedConfig", func(t *testing.T) {
		loader := appconfig.NewLoaderWithPath(writeTestConfigFile(t, `{invalid json}`))

		_, _, err := loadGoogleConfigOrEmpty(loader)
		if err == nil {
			t.Fatal("loadGoogleConfigOrEmpty() error = nil, want error")
		}
		if err.Error() != "failed to load config: failed to parse config file: invalid character 'i' looking for beginning of object key string" {
			t.Fatalf("unexpected error message: %q", err.Error())
		}
	})
}

func TestFindGoogleAccount(t *testing.T) {
	googleCfg := &appconfig.GoogleCalendar{
		Name:     "Google Calendar",
		Accounts: []appconfig.GoogleAccount{{Name: "Work", ClientID: "work-client-id"}},
	}

	t.Run("FindsExistingAccount", func(t *testing.T) {
		account, err := findGoogleAccount(googleCfg, "Work")
		if err != nil {
			t.Fatalf("findGoogleAccount() error = %v", err)
		}
		if account.Name != "Work" {
			t.Fatalf("account name = %q, want %q", account.Name, "Work")
		}
	})

	t.Run("ReturnsAccountNotFound", func(t *testing.T) {
		_, err := findGoogleAccount(googleCfg, "Missing")
		if !errors.Is(err, appconfig.ErrAccountNotFound) {
			t.Fatalf("findGoogleAccount() error = %v, want ErrAccountNotFound", err)
		}
	})
}

func TestAccountSelectionOptions(t *testing.T) {
	googleCfg := &appconfig.GoogleCalendar{
		Name: "Google Calendar",
		Accounts: []appconfig.GoogleAccount{
			{Name: "Work", ClientID: "work-client-id"},
			{Name: "", ClientID: "unnamed-client-id"},
		},
	}

	options := accountSelectionOptions(googleCfg)
	if len(options) != 2 {
		t.Fatalf("options length = %d, want 2", len(options))
	}

	if options[0].Key != "Work" || options[0].Value != "Work" {
		t.Fatalf("option[0] = (%q, %q), want (%q, %q)", options[0].Key, options[0].Value, "Work", "Work")
	}

	if options[1].Key != "Account 2" || options[1].Value != "" {
		t.Fatalf("option[1] = (%q, %q), want (%q, %q)", options[1].Key, options[1].Value, "Account 2", "")
	}
}

func writeTestConfigFile(t *testing.T, content string) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test config file: %v", err)
	}

	return configPath
}
