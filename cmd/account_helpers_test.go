package cmd

import (
	"errors"
	"testing"

	appcalendar "github.com/hossainemruz/waybar-next-events/internal/calendar"
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
}

func TestHasNoAccounts(t *testing.T) {
	if !hasNoAccounts(nil) {
		t.Fatal("hasNoAccounts(nil) = false, want true")
	}

	cfg := &appconfig.Config{Accounts: []appconfig.Account{{ID: "a", Service: appcalendar.ServiceType("outlook"), Name: "Mail"}}}
	if !hasNoAccounts(cfg) {
		t.Fatal("hasNoAccounts(no google accounts) = false, want true")
	}

	cfg.Accounts = append(cfg.Accounts, appconfig.Account{ID: "b", Service: appcalendar.ServiceTypeGoogle, Name: "Work"})
	if hasNoAccounts(cfg) {
		t.Fatal("hasNoAccounts(with google account) = true, want false")
	}
}

func TestEnsureAccountNameAvailable(t *testing.T) {
	cfg := &appconfig.Config{Accounts: []appconfig.Account{{ID: "a", Service: appcalendar.ServiceTypeGoogle, Name: "Work"}}}

	err := ensureAccountNameAvailable(cfg, "Work")
	if !errors.Is(err, appconfig.ErrDuplicateAccountName) {
		t.Fatalf("ensureAccountNameAvailable() error = %v, want ErrDuplicateAccountName", err)
	}

	if err := ensureAccountNameAvailable(cfg, "Personal"); err != nil {
		t.Fatalf("ensureAccountNameAvailable() error = %v, want nil", err)
	}
}

func TestFindAccountByIDUsesID(t *testing.T) {
	cfg := &appconfig.Config{Accounts: []appconfig.Account{{ID: "google-1", Service: appcalendar.ServiceTypeGoogle, Name: "Work"}}}

	account, err := findAccountByID(cfg, "google-1")
	if err != nil {
		t.Fatalf("findAccountByID() error = %v", err)
	}
	if account.Name != "Work" {
		t.Fatalf("account.Name = %q, want Work", account.Name)
	}
}

func TestAccountSelectionOptionsUseStableIDs(t *testing.T) {
	cfg := &appconfig.Config{Accounts: []appconfig.Account{
		{ID: "google-2", Service: appcalendar.ServiceTypeGoogle, Name: "Work"},
		{ID: "other-1", Service: appcalendar.ServiceType("outlook"), Name: "Mail"},
		{ID: "google-1", Service: appcalendar.ServiceTypeGoogle, Name: ""},
	}}

	options := accountSelectionOptions(cfg)
	if len(options) != 2 {
		t.Fatalf("len(options) = %d, want 2", len(options))
	}
	if options[0].Value != "google-2" {
		t.Fatalf("options[0].Value = %q, want google-2", options[0].Value)
	}
	if options[1].Key != "Account 2" || options[1].Value != "google-1" {
		t.Fatalf("options[1] = (%q, %q), want (Account 2, google-1)", options[1].Key, options[1].Value)
	}
}
