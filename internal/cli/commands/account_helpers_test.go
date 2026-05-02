package commands

import (
	"errors"
	"testing"

	appcalendar "github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
)

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
