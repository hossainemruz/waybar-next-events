package cmd

import (
	"context"
	"fmt"

	"charm.land/huh/v2"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/pkg/auth"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/providers"
	"github.com/spf13/cobra"
)

type accountLoginPrompter interface {
	PromptAccountSelection(ctx context.Context, googleCfg *appconfig.GoogleCalendar) (string, error)
}

type accountLoginDependencies struct {
	newLoader        func() *appconfig.Loader
	newPrompter      func(cmd *cobra.Command) accountLoginPrompter
	newAuthenticator func() *auth.Authenticator
	loginAccount     func(ctx context.Context, authenticator *auth.Authenticator, account *appconfig.GoogleAccount) error
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
	_, googleCfg, err := loadGoogleConfigOrEmpty(loader)
	if err != nil {
		return err
	}

	if err := ensureHasAccounts(googleCfg); err != nil {
		return err
	}

	prompter := deps.newPrompter(cmd)
	selectedAccountName, err := prompter.PromptAccountSelection(ctx, googleCfg)
	if err != nil {
		if isUserAbort(err) {
			return nil
		}
		return err
	}

	selectedAccount, err := findGoogleAccount(googleCfg, selectedAccountName)
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

func loginGoogleAccount(ctx context.Context, authenticator *auth.Authenticator, account *appconfig.GoogleAccount) error {
	provider := providers.NewGoogle(account.ClientID, account.ClientSecret, nil)
	if _, err := authenticator.ForceAuthenticate(ctx, provider); err != nil {
		return err
	}

	return nil
}

type huhAccountLoginPrompter struct {
	*huhAccountAddPrompter
}

func (p *huhAccountLoginPrompter) PromptAccountSelection(ctx context.Context, googleCfg *appconfig.GoogleCalendar) (string, error) {
	selectedAccountName := googleCfg.Accounts[0].Name

	form := p.configureForm(
		huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select an account to log in").
					Options(accountSelectionOptions(googleCfg)...).
					Value(&selectedAccountName),
			),
		),
	)

	if err := form.RunWithContext(ctx); err != nil {
		return "", err
	}

	return selectedAccountName, nil
}

var _ accountLoginPrompter = (*huhAccountLoginPrompter)(nil)
