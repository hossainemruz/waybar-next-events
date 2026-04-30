package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/huh/v2"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"github.com/hossainemruz/waybar-next-events/pkg/calendars"
)

const (
	noAccountsConfiguredHint = "add an account first"
	googleClientSecretKey    = "client_secret"
)

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

func accountSelectionOptions(cfg *appconfig.Config) []huh.Option[string] {
	if cfg == nil {
		return nil
	}

	accounts := cfg.Accounts
	options := make([]huh.Option[string], 0, len(accounts))
	for i, account := range accounts {
		options = append(options, huh.NewOption(accountSelectionLabel(account, i), account.ID))
	}

	return options
}

func promptAccountSelection(ctx context.Context, prompter *huhAccountAddPrompter, cfg *appconfig.Config, title string) (string, error) {
	accounts := cfg.Accounts
	selectedAccountID := accounts[0].ID

	form := prompter.configureForm(
		huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(title).
					Options(accountSelectionOptions(cfg)...).
					Value(&selectedAccountID),
			),
		),
	)

	if err := form.RunWithContext(ctx); err != nil {
		return "", err
	}

	return selectedAccountID, nil
}

func accountSelectionLabel(account appconfig.Account, index int) string {
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

func selectedCalendars(discovered []calendars.DiscoveredCalendar, selectedCalendarIDs []string) []appconfig.CalendarRef {
	selected := make(map[string]struct{}, len(selectedCalendarIDs))
	for _, id := range selectedCalendarIDs {
		selected[id] = struct{}{}
	}

	selectedCalendars := make([]appconfig.CalendarRef, 0, len(selectedCalendarIDs))
	for _, discoveredCalendar := range discovered {
		if _, ok := selected[discoveredCalendar.Calendar.ID]; !ok {
			continue
		}
		selectedCalendars = append(selectedCalendars, discoveredCalendar.Calendar)
	}

	if len(selectedCalendars) == 0 {
		return []appconfig.CalendarRef{}
	}

	return selectedCalendars
}

func loadGoogleClientSecret(ctx context.Context, store secrets.Store, account *appconfig.Account) (string, error) {
	if account == nil {
		return "", fmt.Errorf("account cannot be nil")
	}

	value, err := store.Get(ctx, account.ID, googleClientSecretKey)
	if err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			return "", fmt.Errorf("missing stored secret %q", googleClientSecretKey)
		}
		return "", fmt.Errorf("failed to load stored secret %q: %w", googleClientSecretKey, err)
	}

	return value, nil
}
