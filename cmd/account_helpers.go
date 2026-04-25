package cmd

import (
	"fmt"
	"strings"

	"charm.land/huh/v2"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/pkg/calendars"
)

const noAccountsConfiguredHint = "add an account first"

func loadConfig(loader *appconfig.Loader) (*appconfig.Config, error) {
	cfg, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return cfg, nil
}

func loadConfigOrEmpty(loader *appconfig.Loader) (*appconfig.Config, error) {
	cfg, err := loader.LoadOrEmpty()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return cfg, nil
}

func getGoogleConfig(cfg *appconfig.Config) (*appconfig.GoogleCalendar, error) {
	if cfg == nil {
		return nil, fmt.Errorf("failed to get google config: nil config")
	}

	googleCfg, err := cfg.GetGoogleConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get google config: %w", err)
	}

	return googleCfg, nil
}

func loadGoogleConfig(loader *appconfig.Loader) (*appconfig.Config, *appconfig.GoogleCalendar, error) {
	cfg, err := loadConfig(loader)
	if err != nil {
		return nil, nil, err
	}

	googleCfg, err := getGoogleConfig(cfg)
	if err != nil {
		return cfg, nil, err
	}

	return cfg, googleCfg, nil
}

func loadGoogleConfigOrEmpty(loader *appconfig.Loader) (*appconfig.Config, *appconfig.GoogleCalendar, error) {
	cfg, err := loadConfigOrEmpty(loader)
	if err != nil {
		return nil, nil, err
	}

	cfg.EnsureGoogleInitialized()

	googleCfg, err := getGoogleConfig(cfg)
	if err != nil {
		return cfg, nil, err
	}

	return cfg, googleCfg, nil
}

func hasNoAccounts(googleCfg *appconfig.GoogleCalendar) bool {
	return googleCfg == nil || len(googleCfg.Accounts) == 0
}

func ensureHasAccounts(googleCfg *appconfig.GoogleCalendar) error {
	if hasNoAccounts(googleCfg) {
		return fmt.Errorf("%w: %s", appconfig.ErrNoAccounts, noAccountsConfiguredHint)
	}

	return nil
}

func accountNameExists(googleCfg *appconfig.GoogleCalendar, name string) bool {
	if googleCfg == nil {
		return false
	}

	return googleCfg.FindAccountByName(name) != nil
}

func ensureAccountNameAvailable(googleCfg *appconfig.GoogleCalendar, name string) error {
	if accountNameExists(googleCfg, name) {
		return fmt.Errorf("%w: %q", appconfig.ErrDuplicateAccountName, name)
	}

	return nil
}

func findGoogleAccount(googleCfg *appconfig.GoogleCalendar, name string) (*appconfig.GoogleAccount, error) {
	if googleCfg == nil {
		return nil, fmt.Errorf("%w: %q", appconfig.ErrAccountNotFound, name)
	}

	account := googleCfg.FindAccountByName(name)
	if account == nil {
		return nil, fmt.Errorf("%w: %q", appconfig.ErrAccountNotFound, name)
	}

	return account, nil
}

func accountSelectionOptions(googleCfg *appconfig.GoogleCalendar) []huh.Option[string] {
	if googleCfg == nil {
		return nil
	}

	options := make([]huh.Option[string], 0, len(googleCfg.Accounts))
	for i, account := range googleCfg.Accounts {
		options = append(options, huh.NewOption(accountSelectionLabel(account, i), account.Name))
	}

	return options
}

func accountSelectionLabel(account appconfig.GoogleAccount, index int) string {
	if strings.TrimSpace(account.Name) != "" {
		return account.Name
	}

	return fmt.Sprintf("Account %d", index+1)
}

func requiredInput(fieldName string) func(string) error {
	return func(value string) error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", fieldName)
		}
		return nil
	}
}

func selectedCalendars(discovered []calendars.DiscoveredCalendar, selectedCalendarIDs []string) []appconfig.Calendar {
	selected := make(map[string]struct{}, len(selectedCalendarIDs))
	for _, id := range selectedCalendarIDs {
		selected[id] = struct{}{}
	}

	selectedCalendars := make([]appconfig.Calendar, 0, len(selectedCalendarIDs))
	for _, discoveredCalendar := range discovered {
		if _, ok := selected[discoveredCalendar.Calendar.ID]; !ok {
			continue
		}
		selectedCalendars = append(selectedCalendars, discoveredCalendar.Calendar)
	}

	if len(selectedCalendars) == 0 {
		return []appconfig.Calendar{}
	}

	return selectedCalendars
}
