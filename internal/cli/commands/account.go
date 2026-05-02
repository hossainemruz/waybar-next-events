package commands

import (
	"github.com/spf13/cobra"
)

func buildAccountCmd(deps *AppDeps) *cobra.Command {
	account := &cobra.Command{
		Use:   "account",
		Short: "Manage calendar accounts",
		Long:  "Manage calendar accounts: add, update, delete, and re-authenticate accounts interactively.",
	}

	account.AddCommand(buildAccountAddCmd(deps))
	account.AddCommand(buildAccountUpdateCmd(deps))
	account.AddCommand(buildAccountDeleteCmd(deps))
	account.AddCommand(buildAccountLoginCmd(deps))

	return account
}
