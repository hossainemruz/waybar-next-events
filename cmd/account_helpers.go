package cmd

import (
	"fmt"
	"strings"

	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
)

const noAccountsConfiguredHint = "add an account first"

func loadConfigOrEmpty(loader *appconfig.Loader) (*appconfig.Config, error) {
	cfg, err := loader.LoadOrEmpty()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return cfg, nil
}

func hasNoAccounts(cfg *appconfig.Config) bool {
	return cfg == nil || len(cfg.Accounts) == 0
}

func ensureHasAccounts(cfg *appconfig.Config) error {
	if hasNoAccounts(cfg) {
		return fmt.Errorf("%w: %s", appconfig.ErrNoAccounts, noAccountsConfiguredHint)
	}

	return nil
}

func accountNameExists(cfg *appconfig.Config, name string) bool {
	if cfg == nil {
		return false
	}

	return cfg.FindAccountByName(name) != nil
}

func ensureAccountNameAvailable(cfg *appconfig.Config, name string) error {
	if accountNameExists(cfg, name) {
		return fmt.Errorf("%w: %q", appconfig.ErrDuplicateAccountName, name)
	}

	return nil
}

func findAccountByID(cfg *appconfig.Config, id string) (*appconfig.Account, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%w: %q", appconfig.ErrAccountNotFound, id)
	}

	account := cfg.FindAccountByID(id)
	if account == nil {
		return nil, fmt.Errorf("%w: %q", appconfig.ErrAccountNotFound, id)
	}

	return account, nil
}

func validateNewAccountName(cfg *appconfig.Config, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("account name is required")
	}

	if err := ensureAccountNameAvailable(cfg, trimmed); err != nil {
		return err
	}

	return nil
}

func validateUpdatedAccountName(cfg *appconfig.Config, currentAccountName string, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("account name is required")
	}

	if trimmed == currentAccountName {
		return nil
	}

	return ensureAccountNameAvailable(cfg, trimmed)
}
