package commands

import (
	"fmt"
	"strings"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
)

const noAccountsConfiguredHint = "add an account first"

func findAccountByID(accounts []calendar.Account, id string) (*calendar.Account, error) {
	for i := range accounts {
		if accounts[i].ID == id {
			return &accounts[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %q", appconfig.ErrAccountNotFound, id)
}

func accountNameExists(accounts []calendar.Account, name string) bool {
	for _, account := range accounts {
		if account.Name == name {
			return true
		}
	}
	return false
}

func validateNewAccountName(accounts []calendar.Account, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("account name is required")
	}
	if accountNameExists(accounts, trimmed) {
		return fmt.Errorf("%w: %q", appconfig.ErrDuplicateAccountName, trimmed)
	}
	return nil
}

func validateUpdatedAccountName(accounts []calendar.Account, currentAccountName string, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("account name is required")
	}
	if trimmed == currentAccountName {
		return nil
	}
	if accountNameExists(accounts, trimmed) {
		return fmt.Errorf("%w: %q", appconfig.ErrDuplicateAccountName, trimmed)
	}
	return nil
}
