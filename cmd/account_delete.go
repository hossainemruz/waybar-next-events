package cmd

import (
	"context"
	"errors"
	"fmt"

	"charm.land/huh/v2"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/pkg/auth"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/tokenstore"
	"github.com/spf13/cobra"
)

type accountDeletePrompter interface {
	PromptAccountSelection(ctx context.Context, googleCfg *appconfig.GoogleCalendar) (string, error)
	PromptDeleteConfirmation(ctx context.Context, accountName string) (bool, error)
}

type accountDeleteDependencies struct {
	newLoader     func() *appconfig.Loader
	newPrompter   func(cmd *cobra.Command) accountDeletePrompter
	newTokenStore func() tokenstore.TokenStore
	clearToken    func(ctx context.Context, authenticator *auth.Authenticator, account *appconfig.GoogleAccount) error
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
	newTokenStore: func() tokenstore.TokenStore {
		return tokenstore.NewKeyringTokenStore()
	},
	clearToken: clearGoogleAccountToken,
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
	configSnapshot, err := loader.Snapshot()
	if err != nil {
		return fmt.Errorf("failed to snapshot config before save: %w", err)
	}

	cfg, googleCfg, err := loadGoogleConfigOrEmpty(loader)
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

	selectedIndex := -1
	for i := range googleCfg.Accounts {
		if googleCfg.Accounts[i].Name == selectedAccountName {
			selectedIndex = i
			break
		}
	}
	if selectedIndex == -1 {
		return fmt.Errorf("%w: %q", appconfig.ErrAccountNotFound, selectedAccountName)
	}

	selectedAccount := googleCfg.Accounts[selectedIndex]
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

	stagingStore := tokenstore.NewStagedTokenStore()
	authenticator := auth.NewAuthenticator(stagingStore)
	if err := deps.clearToken(ctx, authenticator, &selectedAccount); err != nil {
		return err
	}

	googleCfg.Accounts = deleteGoogleAccountAt(googleCfg.Accounts, selectedIndex)

	if err := loader.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if err := stagingStore.Commit(ctx, deps.newTokenStore()); err != nil {
		rollbackErr := loader.RestoreSnapshot(configSnapshot)
		if rollbackErr != nil {
			return errors.Join(fmt.Errorf("failed to persist OAuth token removal: %w", err), fmt.Errorf("failed to restore config after token persistence error: %w", rollbackErr))
		}
		return fmt.Errorf("failed to persist OAuth token removal: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted account %q.\n", selectedAccount.Name)
	return nil
}

type huhAccountDeletePrompter struct {
	*huhAccountAddPrompter
}

func (p *huhAccountDeletePrompter) PromptAccountSelection(ctx context.Context, googleCfg *appconfig.GoogleCalendar) (string, error) {
	selectedAccountName := googleCfg.Accounts[0].Name

	form := p.configureForm(
		huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select an account to delete").
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

func (p *huhAccountDeletePrompter) PromptDeleteConfirmation(ctx context.Context, accountName string) (bool, error) {
	confirmed := false

	form := p.configureForm(
		huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Delete account %q?", accountName)).
					Description("This removes the account from config and clears its stored OAuth token.").
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

func deleteGoogleAccountAt(accounts []appconfig.GoogleAccount, index int) []appconfig.GoogleAccount {
	if index < 0 || index >= len(accounts) {
		return accounts
	}

	updatedAccounts := make([]appconfig.GoogleAccount, 0, len(accounts)-1)
	updatedAccounts = append(updatedAccounts, accounts[:index]...)
	updatedAccounts = append(updatedAccounts, accounts[index+1:]...)
	return updatedAccounts
}

var _ accountDeletePrompter = (*huhAccountDeletePrompter)(nil)
