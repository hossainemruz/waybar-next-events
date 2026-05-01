package cmd

import (
	"context"
	"fmt"

	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/cli/forms"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/spf13/cobra"
)

type accountAddPrompter interface {
	SelectService(ctx context.Context, services []calendar.Service) (calendar.Service, error)
	PromptAccountFields(ctx context.Context, fields []calendar.AccountField, defaults forms.AccountFieldsInput, validateName func(string) error) (forms.AccountFieldsResult, error)
	SelectCalendars(ctx context.Context, accountName string, calendars []calendar.Calendar, preselected []string) ([]calendar.CalendarRef, error)
	ConfirmEmptyCalendars(ctx context.Context, accountName string) error
}

type accountAddDependencies struct {
	newLoader   func() *appconfig.Loader
	newRegistry func() *calendar.Registry
	newPrompter func(cmd *cobra.Command) accountAddPrompter
	addAccount  func(ctx context.Context, input app.AddAccountInput) (calendar.Account, error)
}

var defaultAccountAddDependencies = accountAddDependencies{
	newLoader: func() *appconfig.Loader {
		return appconfig.NewLoader()
	},
	newRegistry: newAppRegistry,
	newPrompter: func(cmd *cobra.Command) accountAddPrompter {
		return &forms.Prompter{
			Input:  cmd.InOrStdin(),
			Output: cmd.ErrOrStderr(),
		}
	},
	addAccount: func(ctx context.Context, input app.AddAccountInput) (calendar.Account, error) {
		return newAccountManager().AddAccount(ctx, input)
	},
}

// accountAddCmd adds a new calendar account via an interactive form.
var accountAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new calendar account",
	Long:  "Interactively add a new calendar account by entering credentials, authenticating, and selecting calendars.",
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

	registry := deps.newRegistry()
	prompter := deps.newPrompter(cmd)

	service, err := prompter.SelectService(ctx, registry.All())
	if err != nil {
		if forms.IsUserAbort(err) {
			return nil
		}
		return err
	}

	input, err := prompter.PromptAccountFields(ctx, service.AccountFields(), forms.AccountFieldsInput{}, func(name string) error {
		return validateNewAccountName(cfg, name)
	})
	if err != nil {
		if forms.IsUserAbort(err) {
			return nil
		}
		return err
	}

	newAccount, err := deps.addAccount(ctx, app.AddAccountInput{
		Service:  service.Type(),
		Name:     input.Name,
		Settings: input.Settings,
		Secrets:  input.Secrets,
		CalendarSelector: accountCalendarSelector{
			accountName: input.Name,
			prompter:    prompter,
		},
	})
	if err != nil {
		if forms.IsUserAbort(err) {
			return nil
		}
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Added account %q.\n", newAccount.Name)
	return nil
}

type accountCalendarSelector struct {
	accountName string
	prompter    accountAddPrompter
}

func (s accountCalendarSelector) SelectCalendars(ctx context.Context, account calendar.Account, discovered []calendar.Calendar) ([]calendar.CalendarRef, error) {
	return s.prompter.SelectCalendars(ctx, account.Name, discovered, nil)
}

func (s accountCalendarSelector) ConfirmEmptyCalendars(ctx context.Context, account calendar.Account) error {
	return s.prompter.ConfirmEmptyCalendars(ctx, account.Name)
}
