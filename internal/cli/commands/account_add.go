package commands

import (
	"context"
	"fmt"

	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/cli/forms"
	"github.com/spf13/cobra"
)

type accountAddPrompter interface {
	SelectService(ctx context.Context, services []calendar.Service) (calendar.Service, error)
	PromptAccountFields(ctx context.Context, fields []calendar.AccountField, defaults forms.AccountFieldsData, validateName func(string) error) (forms.AccountFieldsData, error)
	SelectCalendars(ctx context.Context, accountName string, calendars []calendar.Calendar, preselected []string) ([]calendar.CalendarRef, error)
	ConfirmEmptyCalendars(ctx context.Context, accountName string) error
}

type accountAddManager interface {
	ListAccounts() ([]calendar.Account, error)
	AddAccount(ctx context.Context, input app.AddAccountInput) (calendar.Account, error)
}

type accountAddDeps struct {
	registry *app.Registry
	manager  accountAddManager
	prompter accountAddPrompter
}

func buildAccountAddCmd(deps *AppDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a new calendar account",
		Long:  "Interactively add a new calendar account by entering credentials, authenticating, and selecting calendars.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAccountAdd(cmd, accountAddDeps{
				registry: deps.Registry,
				manager:  deps.AccountManager,
				prompter: &forms.Prompter{
					Input:  cmd.InOrStdin(),
					Output: cmd.ErrOrStderr(),
				},
			})
		},
	}
}

func runAccountAdd(cmd *cobra.Command, deps accountAddDeps) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	appServices := deps.registry.All()
	calendarServices := make([]calendar.Service, len(appServices))
	for i, s := range appServices {
		calendarServices[i] = s
	}

	service, err := deps.prompter.SelectService(ctx, calendarServices)
	if err != nil {
		if forms.IsUserAbort(err) {
			return nil
		}
		return err
	}

	accounts, err := deps.manager.ListAccounts()
	if err != nil {
		return err
	}

	input, err := deps.prompter.PromptAccountFields(ctx, service.AccountFields(), forms.AccountFieldsData{}, func(name string) error {
		return validateNewAccountName(accounts, name)
	})
	if err != nil {
		if forms.IsUserAbort(err) {
			return nil
		}
		return err
	}

	newAccount, err := deps.manager.AddAccount(ctx, app.AddAccountInput{
		Service:  service.Type(),
		Name:     input.Name,
		Settings: input.Settings,
		Secrets:  input.Secrets,
		CalendarSelector: accountCalendarSelector{
			accountName: input.Name,
			prompter:    deps.prompter,
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
