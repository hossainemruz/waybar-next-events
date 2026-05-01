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

type accountDeletePrompter interface {
	SelectAccount(ctx context.Context, accounts []calendar.Account, title string) (string, error)
	ConfirmDelete(ctx context.Context, accountName string) (bool, error)
}

type accountDeleteDependencies struct {
	newLoader     func() *appconfig.Loader
	newPrompter   func(cmd *cobra.Command) accountDeletePrompter
	deleteAccount func(ctx context.Context, input app.DeleteAccountInput) (calendar.Account, error)
}

var defaultAccountDeleteDependencies = accountDeleteDependencies{
	newLoader: func() *appconfig.Loader {
		return appconfig.NewLoader()
	},
	newPrompter: func(cmd *cobra.Command) accountDeletePrompter {
		return &forms.Prompter{
			Input:  cmd.InOrStdin(),
			Output: cmd.ErrOrStderr(),
		}
	},
	deleteAccount: func(ctx context.Context, input app.DeleteAccountInput) (calendar.Account, error) {
		return newAccountManager().DeleteAccount(ctx, input)
	},
}

// accountDeleteCmd deletes a calendar account via an interactive confirmation flow.
var accountDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a calendar account",
	Long:  "Interactively delete a calendar account by selecting an account and confirming deletion.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAccountDelete(cmd, defaultAccountDeleteDependencies)
	},
}

func init() {
	accountCmd.AddCommand(accountDeleteCmd)
}

func runAccountDelete(cmd *cobra.Command, deps accountDeleteDependencies) error {
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
	selectedAccountID, err := prompter.SelectAccount(ctx, cfg.Accounts, "Select an account to delete")
	if err != nil {
		if forms.IsUserAbort(err) {
			return nil
		}
		return err
	}

	selectedAccount, err := findAccountByID(cfg, selectedAccountID)
	if err != nil {
		return err
	}
	confirmed, err := prompter.ConfirmDelete(ctx, selectedAccount.Name)
	if err != nil {
		if forms.IsUserAbort(err) {
			return nil
		}
		return err
	}
	if !confirmed {
		return nil
	}

	deletedAccount, err := deps.deleteAccount(ctx, app.DeleteAccountInput{AccountID: selectedAccount.ID})
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted account %q.\n", deletedAccount.Name)
	return nil
}
