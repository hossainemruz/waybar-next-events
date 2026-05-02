package commands

import (
	"errors"
	"testing"

	appcalendar "github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
)

func TestAccountCommandRegistration(t *testing.T) {
	root := BuildRoot(&AppDeps{})
	if root == nil {
		t.Fatal("root command = nil")
	}

	if root.Name() != "waybar-next-events" {
		t.Fatalf("root command name = %q, want %q", root.Name(), "waybar-next-events")
	}

	accountCommand, _, err := root.Find([]string{"account"})
	if err != nil {
		t.Fatalf("root.Find(account) error = %v", err)
	}
	if accountCommand == nil {
		t.Fatal("account command is not registered on root command")
	}

	for _, commandName := range []string{"list", "account"} {
		command, _, err := root.Find([]string{commandName})
		if err != nil {
			t.Fatalf("root.Find(%q) error = %v", commandName, err)
		}
		if command == nil {
			t.Fatalf("root.Find(%q) returned nil command", commandName)
		}
	}
}

func TestFindAccountByID(t *testing.T) {
	accounts := []appcalendar.Account{{ID: "other-1", Service: appcalendar.ServiceType("outlook"), Name: "Mail"}}

	account, err := findAccountByID(accounts, "other-1")
	if err != nil {
		t.Fatalf("findAccountByID() error = %v", err)
	}
	if account.Name != "Mail" {
		t.Fatalf("account.Name = %q, want Mail", account.Name)
	}

	_, err = findAccountByID(accounts, "missing")
	if !errors.Is(err, appconfig.ErrAccountNotFound) {
		t.Fatalf("findAccountByID() error = %v, want ErrAccountNotFound", err)
	}
}

func TestValidateNewAccountName(t *testing.T) {
	accounts := []appcalendar.Account{{ID: "a", Service: appcalendar.ServiceTypeGoogle, Name: "Work"}}

	err := validateNewAccountName(accounts, "Work")
	if !errors.Is(err, appconfig.ErrDuplicateAccountName) {
		t.Fatalf("validateNewAccountName() error = %v, want ErrDuplicateAccountName", err)
	}

	if err := validateNewAccountName(accounts, "Personal"); err != nil {
		t.Fatalf("validateNewAccountName() error = %v, want nil", err)
	}

	if err := validateNewAccountName(accounts, "  "); err == nil {
		t.Fatal("validateNewAccountName() error = nil, want error for empty name")
	}
}

func TestValidateUpdatedAccountName(t *testing.T) {
	accounts := []appcalendar.Account{
		{ID: "a", Service: appcalendar.ServiceTypeGoogle, Name: "Work"},
		{ID: "b", Service: appcalendar.ServiceTypeGoogle, Name: "Personal"},
	}

	if err := validateUpdatedAccountName(accounts, "Work", "Work"); err != nil {
		t.Fatalf("validateUpdatedAccountName() error = %v, want nil for same name", err)
	}

	if err := validateUpdatedAccountName(accounts, "Work", "Other"); err != nil {
		t.Fatalf("validateUpdatedAccountName() error = %v, want nil for new unique name", err)
	}

	err := validateUpdatedAccountName(accounts, "Work", "Personal")
	if !errors.Is(err, appconfig.ErrDuplicateAccountName) {
		t.Fatalf("validateUpdatedAccountName() error = %v, want ErrDuplicateAccountName", err)
	}
}
