package cmd

import (
	"github.com/spf13/cobra"
)

// accountCmd is the parent command for managing Google accounts.
// It provides subcommands for adding, updating, deleting, and
// re-authenticating Google Calendar accounts.
var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage Google Calendar accounts",
	Long:  "Manage Google Calendar accounts: add, update, delete, and re-authenticate accounts interactively.",
}

func init() {
	rootCmd.AddCommand(accountCmd)
}
