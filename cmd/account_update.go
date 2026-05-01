package cmd

import (
	"context"
	"fmt"
	"strings"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/app"
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
	newLoader      func() *appconfig.Loader
	newPrompter    func(cmd *cobra.Command) accountUpdatePrompter
	newSecretStore func() secrets.Store
	updateAccount  func(ctx context.Context, input app.UpdateAccountInput) (calendar.Account, error)
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
	newSecretStore: func() secrets.Store { return secrets.NewKeyringStore() },
	updateAccount: func(ctx context.Context, input app.UpdateAccountInput) (calendar.Account, error) {
		return newAccountManager().UpdateAccount(ctx, input)
	},
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
	secretStore := deps.newSecretStore()
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

	updatedAccount, err := deps.updateAccount(ctx, app.UpdateAccountInput{
		AccountID: originalAccount.ID,
		Name:      strings.TrimSpace(input.Name),
		Settings:  mergeUpdatedSettings(originalAccount.Settings, map[string]string{"client_id": strings.TrimSpace(input.ClientID)}),
		Secrets:   map[string]string{googleClientSecretKey: strings.TrimSpace(input.ClientSecret)},
		CalendarSelector: updateCalendarSelector{
			accountName:         input.Name,
			prompter:            prompter,
			selectedCalendarIDs: currentCalendarIDs(*originalAccount),
		},
	})
	if err != nil {
		if isUserAbort(err) {
			return nil
		}
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Updated account %q.\n", updatedAccount.Name)
	return nil
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

func mergeUpdatedSettings(existing map[string]string, updated map[string]string) map[string]string {
	merged := cloneSettings(existing)
	for key, value := range updated {
		merged[key] = value
	}
	return merged
}

var _ accountUpdatePrompter = (*huhAccountUpdatePrompter)(nil)

type updateCalendarSelector struct {
	accountName         string
	prompter            accountUpdatePrompter
	selectedCalendarIDs []string
}

func (s updateCalendarSelector) SelectCalendars(ctx context.Context, account calendar.Account, discovered []calendar.Calendar) ([]calendar.CalendarRef, error) {
	return s.prompter.PromptCalendarSelection(ctx, account.Name, toDiscoveredCalendars(discovered), s.selectedCalendarIDs)
}

func (s updateCalendarSelector) ConfirmEmptyCalendars(ctx context.Context, account calendar.Account) error {
	return s.prompter.ShowNoCalendarsFound(ctx, account.Name)
}
