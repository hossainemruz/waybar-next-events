package cmd

import (
	"github.com/spf13/cobra"
)

// accountCmd is the parent command for managing calendar accounts.
// It provides subcommands for adding, updating, deleting, and
// re-authenticating accounts.
var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage calendar accounts",
	Long:  "Manage calendar accounts: add, update, delete, and re-authenticate accounts interactively.",
}

func init() {
	rootCmd.AddCommand(accountCmd)
}
