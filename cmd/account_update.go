package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/auth"
	"github.com/hossainemruz/waybar-next-events/internal/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"github.com/hossainemruz/waybar-next-events/pkg/calendars"
	"github.com/spf13/cobra"
)

type accountUpdateInput struct {
	Name         string
	ClientID     string
	ClientSecret string
}

type accountUpdatePrompter interface {
	PromptAccountSelection(ctx context.Context, cfg *appconfig.Config) (string, error)
	PromptAccountDetails(ctx context.Context, cfg *appconfig.Config, input accountUpdateInput) (accountUpdateInput, error)
	PromptCalendarSelection(ctx context.Context, accountName string, discovered []calendars.DiscoveredCalendar, selectedCalendarIDs []string) ([]appconfig.CalendarRef, error)
	ShowNoCalendarsFound(ctx context.Context, accountName string) error
}

type accountUpdateDependencies struct {
	newLoader         func() *appconfig.Loader
	newPrompter       func(cmd *cobra.Command) accountUpdatePrompter
	newSecretStore    func() secrets.Store
	newTokenStore     func() tokenstore.TokenStore
	clearToken        func(ctx context.Context, authenticator *auth.Authenticator, secretStore secrets.Store, account *appconfig.Account) error
	discoverCalendars func(ctx context.Context, account *appconfig.Account, secretStore secrets.Store, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error)
}

var defaultAccountUpdateDependencies = accountUpdateDependencies{
	newLoader: func() *appconfig.Loader {
		return appconfig.NewLoader()
	},
	newPrompter: func(cmd *cobra.Command) accountUpdatePrompter {
		return &huhAccountUpdatePrompter{
			huhAccountAddPrompter: &huhAccountAddPrompter{
				input:  cmd.InOrStdin(),
				output: cmd.ErrOrStderr(),
			},
		}
	},
	newSecretStore: func() secrets.Store {
		return secrets.NewKeyringStore()
	},
	newTokenStore: func() tokenstore.TokenStore {
		return tokenstore.NewKeyringTokenStore()
	},
	clearToken:        clearAccountToken,
	discoverCalendars: calendars.DiscoverCalendarsWithAuthenticator,
}

// accountUpdateCmd updates an existing Google Calendar account via an interactive form.
var accountUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update an existing Google Calendar account",
	Long:  "Interactively update an existing Google Calendar account by selecting an account, editing credentials, re-authenticating, and adjusting calendar selections.",
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
	configSnapshot, err := loader.Snapshot()
	if err != nil {
		return fmt.Errorf("failed to snapshot config before save: %w", err)
	}
	secretStore := deps.newSecretStore()

	cfg, err := loadConfigOrEmpty(loader)
	if err != nil {
		return err
	}

	if err := ensureHasAccounts(cfg); err != nil {
		return err
	}

	prompter := deps.newPrompter(cmd)
	selectedAccountID, err := prompter.PromptAccountSelection(ctx, cfg)
	if err != nil {
		if isUserAbort(err) {
			return nil
		}
		return err
	}

	originalAccount, err := findAccountByID(cfg, selectedAccountID)
	if err != nil {
		return err
	}
	originalClientSecret, err := loadGoogleClientSecret(ctx, secretStore, originalAccount)
	if err != nil {
		return err
	}
	input, err := prompter.PromptAccountDetails(ctx, cfg, accountUpdateInput{
		Name:         originalAccount.Name,
		ClientID:     originalAccount.Setting("client_id"),
		ClientSecret: originalClientSecret,
	})
	if err != nil {
		if isUserAbort(err) {
			return nil
		}
		return err
	}

	collected := collectedAccountInput{
		Name:     strings.TrimSpace(input.Name),
		Settings: cloneSettings(originalAccount.Settings),
		Secrets: map[string]string{
			googleClientSecretKey: strings.TrimSpace(input.ClientSecret),
		},
	}
	collected.Settings["client_id"] = strings.TrimSpace(input.ClientID)

	updatedAccount := buildGoogleAccount(collected, originalAccount)
	updatedAccount.Service = calendar.ServiceTypeGoogle

	stagedSecretStore := secrets.NewStagedStore()
	if err := stageGoogleAccountSecrets(ctx, stagedSecretStore, updatedAccount.ID, collected); err != nil {
		return err
	}

	stagingStore := tokenstore.NewStagedTokenStore()
	authenticator := auth.NewAuthenticator(stagingStore)
	backingTokenStore := deps.newTokenStore()

	credentialsChanged := accountCredentialsChanged(*originalAccount, originalClientSecret, *updatedAccount, collected.Secrets[googleClientSecretKey])
	if credentialsChanged {
		if err := deps.clearToken(ctx, authenticator, secretStore, originalAccount); err != nil {
			return err
		}
	} else {
		if err := seedStagedTokenStore(ctx, stagingStore, backingTokenStore, originalAccount); err != nil {
			return err
		}
	}

	discovered, err := deps.discoverCalendars(ctx, updatedAccount, stagedSecretStore, authenticator)
	if err != nil {
		return err
	}

	if len(discovered) == 0 {
		if err := prompter.ShowNoCalendarsFound(ctx, updatedAccount.Name); err != nil {
			if isUserAbort(err) {
				return nil
			}
			return err
		}
		updatedAccount.Calendars = []appconfig.CalendarRef{}
	} else {
		selectedCalendars, err := prompter.PromptCalendarSelection(ctx, updatedAccount.Name, discovered, currentCalendarIDs(*originalAccount))
		if err != nil {
			if isUserAbort(err) {
				return nil
			}
			return err
		}
		updatedAccount.Calendars = selectedCalendars
	}

	for i := range cfg.Accounts {
		if cfg.Accounts[i].ID == updatedAccount.ID {
			cfg.Accounts[i] = *updatedAccount
			break
		}
	}

	if err := loader.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	secretSnapshot, err := snapshotAccountSecrets(ctx, secretStore, updatedAccount.ID, []string{googleClientSecretKey})
	if err != nil {
		rollbackErr := loader.RestoreSnapshot(configSnapshot)
		if rollbackErr != nil {
			return errors.Join(fmt.Errorf("failed to snapshot account secrets: %w", err), fmt.Errorf("failed to restore config after secret snapshot error: %w", rollbackErr))
		}
		return fmt.Errorf("failed to snapshot account secrets: %w", err)
	}

	if err := stagedSecretStore.Commit(ctx, secretStore); err != nil {
		rollbackErr := loader.RestoreSnapshot(configSnapshot)
		if rollbackErr != nil {
			return errors.Join(fmt.Errorf("failed to persist account secrets: %w", err), fmt.Errorf("failed to restore config after secret persistence error: %w", rollbackErr))
		}
		return fmt.Errorf("failed to persist account secrets: %w", err)
	}

	if err := stagingStore.Commit(ctx, backingTokenStore); err != nil {
		secretRollbackErr := restoreAccountSecrets(context.Background(), secretStore, updatedAccount.ID, secretSnapshot)
		configRollbackErr := loader.RestoreSnapshot(configSnapshot)
		if secretRollbackErr != nil && configRollbackErr != nil {
			return errors.Join(fmt.Errorf("failed to persist OAuth token: %w", err), secretRollbackErr, fmt.Errorf("failed to restore config after token persistence error: %w", configRollbackErr))
		}
		if secretRollbackErr != nil {
			return errors.Join(fmt.Errorf("failed to persist OAuth token: %w", err), secretRollbackErr)
		}
		if configRollbackErr != nil {
			return errors.Join(fmt.Errorf("failed to persist OAuth token: %w", err), fmt.Errorf("failed to restore config after token persistence error: %w", configRollbackErr))
		}
		return fmt.Errorf("failed to persist OAuth token: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updated account %q.\n", updatedAccount.Name)
	return nil
}

func seedStagedTokenStore(ctx context.Context, stagedStore *tokenstore.StagedTokenStore, backingStore tokenstore.TokenStore, account *appconfig.Account) error {
	tokenKey, err := googleTokenKey(account)
	if err != nil {
		return err
	}

	token, found, err := backingStore.Get(ctx, tokenKey)
	if err != nil {
		return fmt.Errorf("failed to load existing OAuth token for account %q: %w", account.Name, err)
	}
	if !found {
		return nil
	}

	if err := stagedStore.Set(ctx, tokenKey, token); err != nil {
		return fmt.Errorf("failed to stage existing OAuth token for account %q: %w", account.Name, err)
	}

	return nil
}

func googleTokenKey(account *appconfig.Account) (string, error) {
	if account == nil {
		return "", fmt.Errorf("account cannot be nil")
	}
	if strings.TrimSpace(account.ID) == "" {
		return "", fmt.Errorf("account ID cannot be empty")
	}

	return tokenstore.TokenKey(string(calendar.ServiceTypeGoogle), account.ID), nil
}

type huhAccountUpdatePrompter struct {
	*huhAccountAddPrompter
}

func (p *huhAccountUpdatePrompter) PromptAccountSelection(ctx context.Context, cfg *appconfig.Config) (string, error) {
	return promptAccountSelection(ctx, p.huhAccountAddPrompter, cfg, "Select an account to update")
}

func (p *huhAccountUpdatePrompter) PromptAccountDetails(ctx context.Context, cfg *appconfig.Config, input accountUpdateInput) (accountUpdateInput, error) {
	form := p.configureForm(newUpdateAccountDetailsForm(&input, cfg))
	if err := form.RunWithContext(ctx); err != nil {
		return accountUpdateInput{}, err
	}

	input.Name = strings.TrimSpace(input.Name)
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.ClientSecret = strings.TrimSpace(input.ClientSecret)

	return input, nil
}

func (p *huhAccountUpdatePrompter) PromptCalendarSelection(ctx context.Context, accountName string, discovered []calendars.DiscoveredCalendar, selectedCalendarIDs []string) ([]appconfig.CalendarRef, error) {
	options := calendarSelectionOptions(discovered)

	form := p.configureForm(
		huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title(fmt.Sprintf("Select calendars for %q", accountName)).
					Options(options...).
					Value(&selectedCalendarIDs).
					Height(calendarSelectionHeight(len(options))),
			),
		),
	)

	if err := form.RunWithContext(ctx); err != nil {
		return nil, err
	}

	return selectedCalendars(discovered, selectedCalendarIDs), nil
}

func newUpdateAccountDetailsForm(input *accountUpdateInput, cfg *appconfig.Config) *huh.Form {
	currentAccountName := input.Name

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Account name").
				Placeholder("Work").
				Value(&input.Name).
				Validate(func(value string) error {
					return validateUpdatedAccountName(cfg, currentAccountName, value)
				}),
			huh.NewInput().
				Title(titleClientID).
				Value(&input.ClientID).
				Validate(requiredInput(titleClientID)),
			huh.NewInput().
				Title(titleClientSecret).
				Value(&input.ClientSecret).
				Validate(requiredInput(titleClientSecret)),
		),
	)
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

func currentCalendarIDs(account appconfig.Account) []string {
	if len(account.Calendars) == 0 {
		return []string{}
	}

	calendarIDs := make([]string, 0, len(account.Calendars))
	for _, calendar := range account.Calendars {
		calendarIDs = append(calendarIDs, calendar.ID)
	}

	return calendarIDs
}

func cloneCalendars(calendars []appconfig.CalendarRef) []appconfig.CalendarRef {
	if len(calendars) == 0 {
		return []appconfig.CalendarRef{}
	}

	cloned := make([]appconfig.CalendarRef, len(calendars))
	copy(cloned, calendars)
	return cloned
}

func cloneSettings(settings map[string]string) map[string]string {
	if len(settings) == 0 {
		return map[string]string{}
	}

	cloned := make(map[string]string, len(settings))
	for key, value := range settings {
		cloned[key] = value
	}

	return cloned
}

func accountCredentialsChanged(original appconfig.Account, originalSecret string, updated appconfig.Account, updatedSecret string) bool {
	return original.Setting("client_id") != updated.Setting("client_id") || originalSecret != updatedSecret
}

var _ accountUpdatePrompter = (*huhAccountUpdatePrompter)(nil)
