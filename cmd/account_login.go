package cmd

import (
	"context"
	"fmt"

	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/pkg/auth"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/providers"
	"github.com/spf13/cobra"
)

type accountLoginPrompter interface {
	PromptAccountSelection(ctx context.Context, cfg *appconfig.Config) (string, error)
}

type accountLoginDependencies struct {
	newLoader        func() *appconfig.Loader
	newPrompter      func(cmd *cobra.Command) accountLoginPrompter
	newAuthenticator func() *auth.Authenticator
	loginAccount     func(ctx context.Context, authenticator *auth.Authenticator, account *appconfig.Account) error
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
	newAuthenticator: func() *auth.Authenticator {
		return auth.NewAuthenticator(nil)
	},
	loginAccount: loginGoogleAccount,
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

	authenticator := deps.newAuthenticator()
	if err := deps.loginAccount(ctx, authenticator, selectedAccount); err != nil {
		return fmt.Errorf("failed to log in to account %q: %w", selectedAccount.Name, err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Logged in to account %q.\n", selectedAccount.Name)
	return nil
}

func loginGoogleAccount(ctx context.Context, authenticator *auth.Authenticator, account *appconfig.Account) error {
	provider := providers.NewGoogle(account.Setting("client_id"), account.Setting("client_secret"), nil)
	if _, err := authenticator.ForceAuthenticate(ctx, provider); err != nil {
		return err
	}

	return nil
}

type huhAccountLoginPrompter struct {
	*huhAccountAddPrompter
}

func (p *huhAccountLoginPrompter) PromptAccountSelection(ctx context.Context, cfg *appconfig.Config) (string, error) {
	return promptAccountSelection(ctx, p.huhAccountAddPrompter, cfg, "Select an account to log in")
}

var _ accountLoginPrompter = (*huhAccountLoginPrompter)(nil)
