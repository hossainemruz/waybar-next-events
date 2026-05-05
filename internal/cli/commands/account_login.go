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

type accountLoginPrompter interface {
	SelectAccount(ctx context.Context, accounts []calendar.Account, title string) (string, error)
}

type accountLoginManager interface {
	ListAccounts() ([]calendar.Account, error)
	LoginAccount(ctx context.Context, input app.LoginAccountInput) (calendar.Account, error)
}

type accountLoginDeps struct {
	manager  accountLoginManager
	prompter accountLoginPrompter
}

func buildAccountLoginCmd(deps *AppDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Re-authenticate a calendar account",
		Long:  "Interactively re-authenticate a calendar account by selecting an account and completing the browser-based OAuth2 login flow.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAccountLogin(cmd, accountLoginDeps{
				manager: deps.AccountManager,
				prompter: &forms.Prompter{
					Input:  cmd.InOrStdin(),
					Output: cmd.ErrOrStderr(),
				},
			})
		},
	}
}

func runAccountLogin(cmd *cobra.Command, deps accountLoginDeps) error {
	ctx := cmd.Context()

	accounts, err := deps.manager.ListAccounts()
	if err != nil {
		return err
	}

	if len(accounts) == 0 {
		return fmt.Errorf("%w: %s", appconfig.ErrNoAccounts, noAccountsConfiguredHint)
	}

	selectedAccountID, err := deps.prompter.SelectAccount(ctx, accounts, "Select an account to log in")
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

	loggedInAccount, err := deps.manager.LoginAccount(ctx, app.LoginAccountInput{AccountID: selectedAccount.ID})
	if err != nil {
		return fmt.Errorf("failed to log in to account %q: %w", selectedAccount.Name, err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Logged in to account %q.\n", loggedInAccount.Name)
	return nil
}
