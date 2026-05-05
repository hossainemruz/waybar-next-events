package commands

import (
	"context"
	"fmt"

	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/cli/forms"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/spf13/cobra"
)

type accountDeletePrompter interface {
	SelectAccount(ctx context.Context, accounts []calendar.Account, title string) (string, error)
	ConfirmDelete(ctx context.Context, accountName string) (bool, error)
}

type accountDeleteManager interface {
	ListAccounts() ([]calendar.Account, error)
	DeleteAccount(ctx context.Context, input app.DeleteAccountInput) (calendar.Account, error)
}

type accountDeleteDeps struct {
	manager  accountDeleteManager
	prompter accountDeletePrompter
}

func buildAccountDeleteCmd(deps *AppDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "delete",
		Short: "Delete a calendar account",
		Long:  "Interactively delete a calendar account by selecting an account and confirming deletion.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAccountDelete(cmd, accountDeleteDeps{
				manager: deps.AccountManager,
				prompter: &forms.Prompter{
					Input:  cmd.InOrStdin(),
					Output: cmd.ErrOrStderr(),
				},
			})
		},
	}
}

func runAccountDelete(cmd *cobra.Command, deps accountDeleteDeps) error {
	ctx := cmd.Context()

	accounts, err := deps.manager.ListAccounts()
	if err != nil {
		return err
	}

	if len(accounts) == 0 {
		return fmt.Errorf("%w: %s", appconfig.ErrNoAccounts, noAccountsConfiguredHint)
	}

	selectedAccountID, err := deps.prompter.SelectAccount(ctx, accounts, "Select an account to delete")
	if err != nil {
		if forms.IsUserAbort(err) {
			return nil
		}
		return err
	}

	selectedAccount, err := findAccountByID(accounts, selectedAccountID)
	if err != nil {
		return err
	}

	confirmed, err := deps.prompter.ConfirmDelete(ctx, selectedAccount.Name)
	if err != nil {
		if forms.IsUserAbort(err) {
			return nil
		}
		return err
	}
	if !confirmed {
		return nil
	}

	deletedAccount, err := deps.manager.DeleteAccount(ctx, app.DeleteAccountInput{AccountID: selectedAccount.ID})
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted account %q.\n", deletedAccount.Name)
	return nil
}
