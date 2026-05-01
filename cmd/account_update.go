package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/cli/forms"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"github.com/spf13/cobra"
)

type accountUpdatePrompter interface {
	SelectAccount(ctx context.Context, accounts []calendar.Account, title string) (string, error)
	PromptAccountFields(ctx context.Context, fields []calendar.AccountField, defaults forms.AccountFieldsData, validateName func(string) error) (forms.AccountFieldsData, error)
	SelectCalendars(ctx context.Context, accountName string, calendars []calendar.Calendar, preselected []string) ([]calendar.CalendarRef, error)
	ConfirmEmptyCalendars(ctx context.Context, accountName string) error
}

type accountUpdateDependencies struct {
	newLoader      func() *appconfig.Loader
	newRegistry    func() *calendar.Registry
	newPrompter    func(cmd *cobra.Command) accountUpdatePrompter
	newSecretStore func() secrets.Store
	updateAccount  func(ctx context.Context, input app.UpdateAccountInput) (calendar.Account, error)
}

var defaultAccountUpdateDependencies = accountUpdateDependencies{
	newLoader: func() *appconfig.Loader {
		return appconfig.NewLoader()
	},
	newRegistry: newAppRegistry,
	newPrompter: func(cmd *cobra.Command) accountUpdatePrompter {
		return &forms.Prompter{
			Input:  cmd.InOrStdin(),
			Output: cmd.ErrOrStderr(),
		}
	},
	newSecretStore: func() secrets.Store { return secrets.NewKeyringStore() },
	updateAccount: func(ctx context.Context, input app.UpdateAccountInput) (calendar.Account, error) {
		return newAccountManager().UpdateAccount(ctx, input)
	},
}

// accountUpdateCmd updates an existing calendar account via an interactive form.
var accountUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update an existing calendar account",
	Long:  "Interactively update an existing calendar account by selecting an account, editing credentials, re-authenticating, and adjusting calendar selections.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAccountUpdate(cmd, defaultAccountUpdateDependencies)
	},
}

func init() {
	accountCmd.AddCommand(accountUpdateCmd)
}

func runAccountUpdate(cmd *cobra.Command, deps accountUpdateDependencies) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	loader := deps.newLoader()
	cfg, err := loadConfigOrEmpty(loader)
	if err != nil {
		return err
	}

	if err := ensureHasAccounts(cfg); err != nil {
		return err
	}

	registry := deps.newRegistry()
	prompter := deps.newPrompter(cmd)

	selectedAccountID, err := prompter.SelectAccount(ctx, cfg.Accounts, "Select an account to update")
	if err != nil {
		if forms.IsUserAbort(err) {
			return nil
		}
		return err
	}

	originalAccount, err := findAccountByID(cfg, selectedAccountID)
	if err != nil {
		return err
	}

	service, err := registry.Service(originalAccount.Service)
	if err != nil {
		return err
	}

	secretStore := deps.newSecretStore()
	defaults, err := loadAccountFieldDefaults(ctx, service.AccountFields(), secretStore, originalAccount)
	if err != nil {
		return err
	}

	input, err := prompter.PromptAccountFields(ctx, service.AccountFields(), defaults, func(name string) error {
		return validateUpdatedAccountName(cfg, originalAccount.Name, name)
	})
	if err != nil {
		if forms.IsUserAbort(err) {
			return nil
		}
		return err
	}

	// Merge form-managed settings back into the original settings so
	// unknown provider keys (e.g. a setting not listed in AccountFields)
	// are not dropped on update.
	settings := make(map[string]string, len(originalAccount.Settings))
	for k, v := range originalAccount.Settings {
		settings[k] = v
	}
	for k, v := range input.Settings {
		settings[k] = v
	}

	updatedAccount, err := deps.updateAccount(ctx, app.UpdateAccountInput{
		AccountID: originalAccount.ID,
		Name:      input.Name,
		Settings:  settings,
		Secrets:   input.Secrets,
		CalendarSelector: updateCalendarSelector{
			accountName:         input.Name,
			prompter:            prompter,
			selectedCalendarIDs: originalAccount.CalendarIDs(),
		},
	})
	if err != nil {
		if forms.IsUserAbort(err) {
			return nil
		}
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updated account %q.\n", updatedAccount.Name)
	return nil
}

func loadAccountFieldDefaults(ctx context.Context, fields []calendar.AccountField, store secrets.Store, account *appconfig.Account) (forms.AccountFieldsData, error) {
	// Seed all current settings so the form preserves unknown keys.
	settings := make(map[string]string, len(account.Settings))
	for k, v := range account.Settings {
		settings[k] = v
	}

	defaults := forms.AccountFieldsData{
		Name:     account.Name,
		Settings: settings,
		Secrets:  make(map[string]string),
	}
	for _, field := range fields {
		if !field.Secret {
			continue
		}
		value, err := store.Get(ctx, account.ID, field.Key)
		if err != nil {
			if errors.Is(err, secrets.ErrSecretNotFound) {
				continue
			}
			return forms.AccountFieldsData{}, fmt.Errorf("load stored secret %q: %w", field.Key, err)
		}
		defaults.Secrets[field.Key] = value
	}
	return defaults, nil
}

type updateCalendarSelector struct {
	accountName         string
	prompter            accountUpdatePrompter
	selectedCalendarIDs []string
}

func (s updateCalendarSelector) SelectCalendars(ctx context.Context, account calendar.Account, discovered []calendar.Calendar) ([]calendar.CalendarRef, error) {
	return s.prompter.SelectCalendars(ctx, account.Name, discovered, s.selectedCalendarIDs)
}

func (s updateCalendarSelector) ConfirmEmptyCalendars(ctx context.Context, account calendar.Account) error {
	return s.prompter.ConfirmEmptyCalendars(ctx, account.Name)
}
