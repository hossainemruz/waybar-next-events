package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
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
	PromptAccountDetails(ctx context.Context, cfg *appconfig.Config) (accountAddInput, error)
	PromptCalendarSelection(ctx context.Context, accountName string, discovered []calendars.DiscoveredCalendar) ([]appconfig.CalendarRef, error)
	ShowNoCalendarsFound(ctx context.Context, accountName string) error
}

type accountAddDependencies struct {
	newLoader   func() *appconfig.Loader
	newPrompter func(cmd *cobra.Command) accountAddPrompter
	addAccount  func(ctx context.Context, input app.AddAccountInput) (calendar.Account, error)
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
	addAccount: func(ctx context.Context, input app.AddAccountInput) (calendar.Account, error) {
		return newAccountManager().AddAccount(ctx, input)
	},
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
	cfg, err := loadConfigOrEmpty(loader)
	if err != nil {
		return err
	}

	prompter := deps.newPrompter(cmd)
	input, err := prompter.PromptAccountDetails(ctx, cfg)
	if err != nil {
		if isUserAbort(err) {
			return nil
		}
		return err
	}

	newAccount, err := deps.addAccount(ctx, app.AddAccountInput{
		Service:  calendar.ServiceTypeGoogle,
		Name:     strings.TrimSpace(input.Name),
		Settings: map[string]string{"client_id": strings.TrimSpace(input.ClientID)},
		Secrets:  map[string]string{googleClientSecretKey: strings.TrimSpace(input.ClientSecret)},
		CalendarSelector: accountCalendarSelector{
			accountName: input.Name,
			prompter:    prompter,
		},
	})
	if err != nil {
		if isUserAbort(err) {
			return nil
		}
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Added account %q.\n", newAccount.Name)
	return nil
}

type huhAccountAddPrompter struct {
	input      io.Reader
	output     io.Writer
	accessible bool
}

func (p *huhAccountAddPrompter) PromptAccountDetails(ctx context.Context, cfg *appconfig.Config) (accountAddInput, error) {
	var input accountAddInput

	form := p.configureForm(newAccountDetailsForm(&input, cfg))
	if err := form.RunWithContext(ctx); err != nil {
		return accountAddInput{}, err
	}

	input.Name = strings.TrimSpace(input.Name)
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.ClientSecret = strings.TrimSpace(input.ClientSecret)

	return input, nil
}

func (p *huhAccountAddPrompter) PromptCalendarSelection(ctx context.Context, accountName string, discovered []calendars.DiscoveredCalendar) ([]appconfig.CalendarRef, error) {
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

	return selectedCalendars(discovered, selectedCalendarIDs), nil
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

func newAccountDetailsForm(input *accountAddInput, cfg *appconfig.Config) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Account name").
				Placeholder("Work").
				Value(&input.Name).
				Validate(func(value string) error {
					return validateNewAccountName(cfg, value)
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

type accountCalendarSelector struct {
	accountName string
	prompter    accountAddPrompter
}

func (s accountCalendarSelector) SelectCalendars(ctx context.Context, account calendar.Account, discovered []calendar.Calendar) ([]calendar.CalendarRef, error) {
	return s.prompter.PromptCalendarSelection(ctx, account.Name, toDiscoveredCalendars(discovered))
}

func (s accountCalendarSelector) ConfirmEmptyCalendars(ctx context.Context, account calendar.Account) error {
	return s.prompter.ShowNoCalendarsFound(ctx, account.Name)
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

func toDiscoveredCalendars(calendarsList []calendar.Calendar) []calendars.DiscoveredCalendar {
	if len(calendarsList) == 0 {
		return []calendars.DiscoveredCalendar{}
	}

	discovered := make([]calendars.DiscoveredCalendar, 0, len(calendarsList))
	for _, discoveredCalendar := range calendarsList {
		discovered = append(discovered, calendars.DiscoveredCalendar{
			Calendar: appconfig.CalendarRef{ID: discoveredCalendar.ID, Name: discoveredCalendar.Name},
			Primary:  discoveredCalendar.Primary,
		})
	}

	return discovered
}
