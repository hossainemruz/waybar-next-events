package cmd

import (
	"context"
	"errors"
	"fmt"

	"charm.land/huh/v2"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"github.com/hossainemruz/waybar-next-events/pkg/auth"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/tokenstore"
	"github.com/spf13/cobra"
)

type accountDeletePrompter interface {
	PromptAccountSelection(ctx context.Context, cfg *appconfig.Config) (string, error)
	PromptDeleteConfirmation(ctx context.Context, accountName string) (bool, error)
}

type accountDeleteDependencies struct {
	newLoader      func() *appconfig.Loader
	newPrompter    func(cmd *cobra.Command) accountDeletePrompter
	newSecretStore func() secrets.Store
	newTokenStore  func() tokenstore.TokenStore
	clearToken     func(ctx context.Context, authenticator *auth.Authenticator, secretStore secrets.Store, account *appconfig.Account) error
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
	newSecretStore: func() secrets.Store {
		return secrets.NewKeyringStore()
	},
	newTokenStore: func() tokenstore.TokenStore {
		return tokenstore.NewKeyringTokenStore()
	},
	clearToken: clearAccountToken,
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
	secretStore := deps.newSecretStore()

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

	stagingStore := tokenstore.NewStagedTokenStore()
	stagedSecretStore := secrets.NewStagedStore()
	authenticator := auth.NewAuthenticator(stagingStore)
	if err := deps.clearToken(ctx, authenticator, secretStore, selectedAccount); err != nil {
		return err
	}
	secretSnapshot, err := snapshotAccountSecrets(ctx, secretStore, selectedAccount.ID, []string{googleClientSecretKey})
	if err != nil {
		return fmt.Errorf("failed to snapshot account secrets: %w", err)
	}
	if err := stageGoogleAccountSecretDeletion(ctx, stagedSecretStore, selectedAccount.ID); err != nil {
		return err
	}

	cfg.Accounts = deleteAccountByID(cfg.Accounts, selectedAccount.ID)

	if err := loader.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if err := stagedSecretStore.Commit(ctx, secretStore); err != nil {
		rollbackErr := loader.RestoreSnapshot(configSnapshot)
		if rollbackErr != nil {
			return errors.Join(fmt.Errorf("failed to remove account secrets: %w", err), fmt.Errorf("failed to restore config after secret persistence error: %w", rollbackErr))
		}
		return fmt.Errorf("failed to remove account secrets: %w", err)
	}

	if err := stagingStore.Commit(ctx, deps.newTokenStore()); err != nil {
		secretRollbackErr := restoreAccountSecrets(context.Background(), secretStore, selectedAccount.ID, secretSnapshot)
		configRollbackErr := loader.RestoreSnapshot(configSnapshot)
		if secretRollbackErr != nil && configRollbackErr != nil {
			return errors.Join(fmt.Errorf("failed to persist OAuth token removal: %w", err), secretRollbackErr, fmt.Errorf("failed to restore config after token persistence error: %w", configRollbackErr))
		}
		if secretRollbackErr != nil {
			return errors.Join(fmt.Errorf("failed to persist OAuth token removal: %w", err), secretRollbackErr)
		}
		if configRollbackErr != nil {
			return errors.Join(fmt.Errorf("failed to persist OAuth token removal: %w", err), fmt.Errorf("failed to restore config after token persistence error: %w", configRollbackErr))
		}
		return fmt.Errorf("failed to persist OAuth token removal: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted account %q.\n", selectedAccount.Name)
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

func deleteAccountByID(accounts []appconfig.Account, id string) []appconfig.Account {
	updatedAccounts := make([]appconfig.Account, 0, len(accounts))
	for _, account := range accounts {
		if account.ID == id {
			continue
		}
		updatedAccounts = append(updatedAccounts, account)
	}
	return updatedAccounts
}

var _ accountDeletePrompter = (*huhAccountDeletePrompter)(nil)
