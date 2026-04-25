package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"charm.land/huh/v2"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/pkg/auth"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/pkg/calendars"
	"github.com/spf13/cobra"
)

const (
	titleClientID              = "OAuth Client ID"
	titleClientSecret          = "OAuth Client Secret"
	minCalendarSelectionHeight = 4
	maxCalendarSelectionHeight = 10
)

type accountAddInput struct {
	Name         string
	ClientID     string
	ClientSecret string
}

type accountAddPrompter interface {
	PromptAccountDetails(ctx context.Context, googleCfg *appconfig.GoogleCalendar) (accountAddInput, error)
	PromptCalendarSelection(ctx context.Context, accountName string, discovered []calendars.DiscoveredCalendar) ([]appconfig.Calendar, error)
	ShowNoCalendarsFound(ctx context.Context, accountName string) error
}

type accountAddDependencies struct {
	newLoader         func() *appconfig.Loader
	newPrompter       func(cmd *cobra.Command) accountAddPrompter
	newTokenStore     func() tokenstore.TokenStore
	discoverCalendars func(ctx context.Context, account *appconfig.GoogleAccount, authenticator *auth.Authenticator) ([]calendars.DiscoveredCalendar, error)
}

var defaultAccountAddDependencies = accountAddDependencies{
	newLoader: func() *appconfig.Loader {
		return appconfig.NewLoader()
	},
	newPrompter: func(cmd *cobra.Command) accountAddPrompter {
		return &huhAccountAddPrompter{
			input:  cmd.InOrStdin(),
			output: cmd.ErrOrStderr(),
		}
	},
	newTokenStore: func() tokenstore.TokenStore {
		return tokenstore.NewKeyringTokenStore()
	},
	discoverCalendars: calendars.DiscoverCalendarsWithAuthenticator,
}

// accountAddCmd adds a new Google Calendar account via an interactive form.
var accountAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new Google Calendar account",
	Long:  "Interactively add a new Google Calendar account by entering credentials, authenticating via OAuth2, and selecting calendars.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAccountAdd(cmd, defaultAccountAddDependencies)
	},
}

func init() {
	accountCmd.AddCommand(accountAddCmd)
}

func runAccountAdd(cmd *cobra.Command, deps accountAddDependencies) error {
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

	prompter := deps.newPrompter(cmd)
	input, err := prompter.PromptAccountDetails(ctx, googleCfg)
	if err != nil {
		if isUserAbort(err) {
			return nil
		}
		return err
	}

	stagingStore := tokenstore.NewStagedTokenStore()
	authenticator := auth.NewAuthenticator(stagingStore)
	newAccount := &appconfig.GoogleAccount{
		Name:         strings.TrimSpace(input.Name),
		ClientID:     strings.TrimSpace(input.ClientID),
		ClientSecret: strings.TrimSpace(input.ClientSecret),
	}

	discovered, err := deps.discoverCalendars(ctx, newAccount, authenticator)
	if err != nil {
		return err
	}

	if len(discovered) == 0 {
		if err := prompter.ShowNoCalendarsFound(ctx, newAccount.Name); err != nil {
			if isUserAbort(err) {
				return nil
			}
			return err
		}
		newAccount.Calendars = []appconfig.Calendar{}
	} else {
		selectedCalendars, err := prompter.PromptCalendarSelection(ctx, newAccount.Name, discovered)
		if err != nil {
			if isUserAbort(err) {
				return nil
			}
			return err
		}
		newAccount.Calendars = selectedCalendars
	}

	googleCfg.Accounts = append(googleCfg.Accounts, *newAccount)

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

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Added account %q.\n", newAccount.Name)
	return nil
}

type huhAccountAddPrompter struct {
	input      io.Reader
	output     io.Writer
	accessible bool
}

func (p *huhAccountAddPrompter) PromptAccountDetails(ctx context.Context, googleCfg *appconfig.GoogleCalendar) (accountAddInput, error) {
	var input accountAddInput

	form := p.configureForm(newAccountDetailsForm(&input, googleCfg))
	if err := form.RunWithContext(ctx); err != nil {
		return accountAddInput{}, err
	}

	input.Name = strings.TrimSpace(input.Name)
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.ClientSecret = strings.TrimSpace(input.ClientSecret)

	return input, nil
}

func (p *huhAccountAddPrompter) PromptCalendarSelection(ctx context.Context, accountName string, discovered []calendars.DiscoveredCalendar) ([]appconfig.Calendar, error) {
	options := calendarSelectionOptions(discovered)
	selectedCalendarIDs := make([]string, 0)

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

	selected := make(map[string]struct{}, len(selectedCalendarIDs))
	for _, id := range selectedCalendarIDs {
		selected[id] = struct{}{}
	}

	calendars := make([]appconfig.Calendar, 0, len(selectedCalendarIDs))
	for _, calendar := range discovered {
		if _, ok := selected[calendar.Calendar.ID]; !ok {
			continue
		}
		calendars = append(calendars, calendar.Calendar)
	}

	if len(calendars) == 0 {
		return []appconfig.Calendar{}, nil
	}

	return calendars, nil
}

func (p *huhAccountAddPrompter) ShowNoCalendarsFound(ctx context.Context, accountName string) error {
	form := p.configureForm(
		huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("No calendars found").
					Description(fmt.Sprintf("No calendars were found for account %q. It will be saved with an empty calendars list.", accountName)).
					Next(true).
					NextLabel("Continue"),
			),
		),
	)

	return form.RunWithContext(ctx)
}

func (p *huhAccountAddPrompter) configureForm(form *huh.Form) *huh.Form {
	if p.output != nil {
		form = form.WithOutput(p.output)
	}
	if p.input != nil {
		form = form.WithInput(p.input)
	}
	if p.accessible {
		form = form.WithAccessible(true)
	}
	return form
}

func newAccountDetailsForm(input *accountAddInput, googleCfg *appconfig.GoogleCalendar) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Account name").
				Placeholder("Work").
				Value(&input.Name).
				Validate(func(value string) error {
					return validateNewAccountName(googleCfg, value)
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

func validateNewAccountName(googleCfg *appconfig.GoogleCalendar, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("account name is required")
	}

	if err := ensureAccountNameAvailable(googleCfg, trimmed); err != nil {
		return err
	}

	return nil
}
func calendarSelectionOptions(discovered []calendars.DiscoveredCalendar) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(discovered))
	for _, discoveredCalendar := range discovered {
		label := discoveredCalendar.Calendar.Name
		if discoveredCalendar.Primary {
			label += " (Primary)"
		}
		options = append(options, huh.NewOption(label, discoveredCalendar.Calendar.ID))
	}
	return options
}

func calendarSelectionHeight(optionCount int) int {
	height := optionCount + 1
	if height < minCalendarSelectionHeight {
		return minCalendarSelectionHeight
	}
	if height > maxCalendarSelectionHeight {
		return maxCalendarSelectionHeight
	}
	return height
}

func isUserAbort(err error) bool {
	return errors.Is(err, huh.ErrUserAborted) || errors.Is(err, context.Canceled)
}
