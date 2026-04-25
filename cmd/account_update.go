package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/huh/v2"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/pkg/auth"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/providers"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/pkg/calendars"
	"github.com/spf13/cobra"
)

type accountUpdateInput struct {
	Name         string
	ClientID     string
	ClientSecret string
}

type accountUpdatePrompter interface {
	PromptAccountSelection(ctx context.Context, googleCfg *appconfig.GoogleCalendar) (string, error)
	PromptAccountDetails(ctx context.Context, googleCfg *appconfig.GoogleCalendar, input accountUpdateInput) (accountUpdateInput, error)
	PromptCalendarSelection(ctx context.Context, accountName string, discovered []calendars.DiscoveredCalendar, selectedCalendarIDs []string) ([]appconfig.Calendar, error)
	ShowNoCalendarsFound(ctx context.Context, accountName string) error
}

type accountUpdateDependencies struct {
	newLoader         func() *appconfig.Loader
	newPrompter       func(cmd *cobra.Command) accountUpdatePrompter
	newTokenStore     func() tokenstore.TokenStore
	clearToken        func(ctx context.Context, authenticator *auth.Authenticator, account *appconfig.GoogleAccount) error
	discoverCalendars func(ctx context.Context, account *appconfig.GoogleAccount, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error)
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
	newTokenStore: func() tokenstore.TokenStore {
		return tokenstore.NewKeyringTokenStore()
	},
	clearToken:        clearGoogleAccountToken,
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

	cfg, googleCfg, err := loadGoogleConfigOrEmpty(loader)
	if err != nil {
		return err
	}

	if err := ensureHasAccounts(googleCfg); err != nil {
		return err
	}

	prompter := deps.newPrompter(cmd)
	selectedAccountName, err := prompter.PromptAccountSelection(ctx, googleCfg)
	if err != nil {
		if isUserAbort(err) {
			return nil
		}
		return err
	}

	originalIndex := -1
	for i := range googleCfg.Accounts {
		if googleCfg.Accounts[i].Name == selectedAccountName {
			originalIndex = i
			break
		}
	}
	if originalIndex == -1 {
		return fmt.Errorf("%w: %q", appconfig.ErrAccountNotFound, selectedAccountName)
	}

	originalAccount := googleCfg.Accounts[originalIndex]
	input, err := prompter.PromptAccountDetails(ctx, googleCfg, accountUpdateInput{
		Name:         originalAccount.Name,
		ClientID:     originalAccount.ClientID,
		ClientSecret: originalAccount.ClientSecret,
	})
	if err != nil {
		if isUserAbort(err) {
			return nil
		}
		return err
	}

	updatedAccount := &appconfig.GoogleAccount{
		Name:         strings.TrimSpace(input.Name),
		ClientID:     strings.TrimSpace(input.ClientID),
		ClientSecret: strings.TrimSpace(input.ClientSecret),
		Calendars:    cloneCalendars(originalAccount.Calendars),
	}

	stagingStore := tokenstore.NewStagedTokenStore()
	authenticator := auth.NewAuthenticator(stagingStore)

	if accountCredentialsChanged(originalAccount, *updatedAccount) {
		if err := deps.clearToken(ctx, authenticator, &originalAccount); err != nil {
			return err
		}
	}

	discovered, err := deps.discoverCalendars(ctx, updatedAccount, authenticator)
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
		updatedAccount.Calendars = []appconfig.Calendar{}
	} else {
		selectedCalendars, err := prompter.PromptCalendarSelection(ctx, updatedAccount.Name, discovered, currentCalendarIDs(originalAccount))
		if err != nil {
			if isUserAbort(err) {
				return nil
			}
			return err
		}
		updatedAccount.Calendars = selectedCalendars
	}

	googleCfg.Accounts[originalIndex] = *updatedAccount

	if err := loader.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if err := stagingStore.Commit(ctx, deps.newTokenStore()); err != nil {
		rollbackErr := loader.RestoreSnapshot(configSnapshot)
		if rollbackErr != nil {
			return errors.Join(fmt.Errorf("failed to persist OAuth token: %w", err), fmt.Errorf("failed to restore config after token persistence error: %w", rollbackErr))
		}
		return fmt.Errorf("failed to persist OAuth token: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updated account %q.\n", updatedAccount.Name)
	return nil
}

type huhAccountUpdatePrompter struct {
	*huhAccountAddPrompter
}

func (p *huhAccountUpdatePrompter) PromptAccountSelection(ctx context.Context, googleCfg *appconfig.GoogleCalendar) (string, error) {
	selectedAccountName := googleCfg.Accounts[0].Name

	form := p.configureForm(
		huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select an account to update").
					Options(accountSelectionOptions(googleCfg)...).
					Value(&selectedAccountName),
			),
		),
	)

	if err := form.RunWithContext(ctx); err != nil {
		return "", err
	}

	return selectedAccountName, nil
}

func (p *huhAccountUpdatePrompter) PromptAccountDetails(ctx context.Context, googleCfg *appconfig.GoogleCalendar, input accountUpdateInput) (accountUpdateInput, error) {
	form := p.configureForm(newUpdateAccountDetailsForm(&input, googleCfg))
	if err := form.RunWithContext(ctx); err != nil {
		return accountUpdateInput{}, err
	}

	input.Name = strings.TrimSpace(input.Name)
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.ClientSecret = strings.TrimSpace(input.ClientSecret)

	return input, nil
}

func (p *huhAccountUpdatePrompter) PromptCalendarSelection(ctx context.Context, accountName string, discovered []calendars.DiscoveredCalendar, selectedCalendarIDs []string) ([]appconfig.Calendar, error) {
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

func newUpdateAccountDetailsForm(input *accountUpdateInput, googleCfg *appconfig.GoogleCalendar) *huh.Form {
	currentAccountName := input.Name

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Account name").
				Placeholder("Work").
				Value(&input.Name).
				Validate(func(value string) error {
					return validateUpdatedAccountName(googleCfg, currentAccountName, value)
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

func validateUpdatedAccountName(googleCfg *appconfig.GoogleCalendar, currentAccountName string, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("account name is required")
	}

	if trimmed == currentAccountName {
		return nil
	}

	return ensureAccountNameAvailable(googleCfg, trimmed)
}

func currentCalendarIDs(account appconfig.GoogleAccount) []string {
	if len(account.Calendars) == 0 {
		return []string{}
	}

	calendarIDs := make([]string, 0, len(account.Calendars))
	for _, calendar := range account.Calendars {
		calendarIDs = append(calendarIDs, calendar.ID)
	}

	return calendarIDs
}

func cloneCalendars(calendars []appconfig.Calendar) []appconfig.Calendar {
	if len(calendars) == 0 {
		return []appconfig.Calendar{}
	}

	cloned := make([]appconfig.Calendar, len(calendars))
	copy(cloned, calendars)
	return cloned
}

func accountCredentialsChanged(original appconfig.GoogleAccount, updated appconfig.GoogleAccount) bool {
	return original.ClientID != updated.ClientID || original.ClientSecret != updated.ClientSecret
}

func clearGoogleAccountToken(ctx context.Context, authenticator *auth.Authenticator, account *appconfig.GoogleAccount) error {
	provider := providers.NewGoogle(account.ClientID, account.ClientSecret, nil)
	if err := authenticator.ClearToken(ctx, provider); err != nil {
		return fmt.Errorf("failed to clear old OAuth token for account %q: %w", account.Name, err)
	}

	return nil
}

var _ accountUpdatePrompter = (*huhAccountUpdatePrompter)(nil)
