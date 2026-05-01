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
	if hasNoAccounts(cfg) {
		t.Fatal("hasNoAccounts(with non-google account) = true, want false")
	}

	cfg.Accounts = append(cfg.Accounts, appconfig.Account{ID: "b", Service: appcalendar.ServiceTypeGoogle, Name: "Work"})
	if hasNoAccounts(cfg) {
		t.Fatal("hasNoAccounts(with accounts) = true, want false")
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
	cfg := &appconfig.Config{Accounts: []appconfig.Account{{ID: "other-1", Service: appcalendar.ServiceType("outlook"), Name: "Mail"}}}

	account, err := findAccountByID(cfg, "other-1")
	if err != nil {
		t.Fatalf("findAccountByID() error = %v", err)
	}
	if account.Name != "Mail" {
		t.Fatalf("account.Name = %q, want Mail", account.Name)
	}
}
