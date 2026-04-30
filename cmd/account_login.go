package cmd

import (
	"context"
	"fmt"

	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/spf13/cobra"
)

type accountLoginPrompter interface {
	PromptAccountSelection(ctx context.Context, cfg *appconfig.Config) (string, error)
}

type accountLoginDependencies struct {
	newLoader    func() *appconfig.Loader
	newPrompter  func(cmd *cobra.Command) accountLoginPrompter
	loginAccount func(ctx context.Context, input app.LoginAccountInput) (calendar.Account, error)
}

var defaultAccountLoginDependencies = accountLoginDependencies{
	newLoader: func() *appconfig.Loader {
		return appconfig.NewLoader()
	},
	newPrompter: func(cmd *cobra.Command) accountLoginPrompter {
		return &huhAccountLoginPrompter{
			huhAccountAddPrompter: &huhAccountAddPrompter{
				input:  cmd.InOrStdin(),
				output: cmd.ErrOrStderr(),
			},
		}
	},
	loginAccount: func(ctx context.Context, input app.LoginAccountInput) (calendar.Account, error) {
		return newAccountManager().LoginAccount(ctx, input)
	},
}

// accountLoginCmd re-authenticates a Google Calendar account via the browser OAuth2 flow.
var accountLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Re-authenticate a Google Calendar account",
	Long:  "Interactively re-authenticate a Google Calendar account by selecting an account and completing the browser-based OAuth2 login flow.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAccountLogin(cmd, defaultAccountLoginDependencies)
	},
}

func init() {
	accountCmd.AddCommand(accountLoginCmd)
}

func runAccountLogin(cmd *cobra.Command, deps accountLoginDependencies) error {
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

	selectedAccount, err := findAccountByID(cfg, selectedAccountID)
	if err != nil {
		return err
	}

	loggedInAccount, err := deps.loginAccount(ctx, app.LoginAccountInput{AccountID: selectedAccount.ID})
	if err != nil {
		return fmt.Errorf("failed to log in to account %q: %w", selectedAccount.Name, err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Logged in to account %q.\n", loggedInAccount.Name)
	return nil
}

type huhAccountLoginPrompter struct {
	*huhAccountAddPrompter
}

func (p *huhAccountLoginPrompter) PromptAccountSelection(ctx context.Context, cfg *appconfig.Config) (string, error) {
	return promptAccountSelection(ctx, p.huhAccountAddPrompter, cfg, "Select an account to log in")
}

var _ accountLoginPrompter = (*huhAccountLoginPrompter)(nil)
