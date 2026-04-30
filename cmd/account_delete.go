package cmd

import (
	"context"
	"fmt"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/spf13/cobra"
)

type accountDeletePrompter interface {
	PromptAccountSelection(ctx context.Context, cfg *appconfig.Config) (string, error)
	PromptDeleteConfirmation(ctx context.Context, accountName string) (bool, error)
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
		return &huhAccountDeletePrompter{
			huhAccountAddPrompter: &huhAccountAddPrompter{
				input:  cmd.InOrStdin(),
				output: cmd.ErrOrStderr(),
			},
		}
	},
	deleteAccount: func(ctx context.Context, input app.DeleteAccountInput) (calendar.Account, error) {
		return newAccountManager().DeleteAccount(ctx, input)
	},
}

// accountDeleteCmd deletes a Google Calendar account via an interactive confirmation flow.
var accountDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a Google Calendar account",
	Long:  "Interactively delete a Google Calendar account by selecting an account and confirming deletion.",
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
	confirmed, err := prompter.PromptDeleteConfirmation(ctx, selectedAccount.Name)
	if err != nil {
		if isUserAbort(err) {
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

type huhAccountDeletePrompter struct {
	*huhAccountAddPrompter
}

func (p *huhAccountDeletePrompter) PromptAccountSelection(ctx context.Context, cfg *appconfig.Config) (string, error) {
	return promptAccountSelection(ctx, p.huhAccountAddPrompter, cfg, "Select an account to delete")
}

func (p *huhAccountDeletePrompter) PromptDeleteConfirmation(ctx context.Context, accountName string) (bool, error) {
	confirmed := false

	form := p.configureForm(
		huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Delete account %q?", accountName)).
					Description("This removes the account from config and clears its stored OAuth token and secrets.").
					Affirmative("Delete").
					Negative("Cancel").
					Value(&confirmed),
			),
		),
	)

	if err := form.RunWithContext(ctx); err != nil {
		return false, err
	}

	return confirmed, nil
}

var _ accountDeletePrompter = (*huhAccountDeletePrompter)(nil)
