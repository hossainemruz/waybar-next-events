package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// accountUpdateCmd updates an existing Google Calendar account via an interactive form.
var accountUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update an existing Google Calendar account",
	Long:  "Interactively update an existing Google Calendar account by selecting an account, editing credentials, re-authenticating, and adjusting calendar selections.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("account update: not yet implemented")
	},
}

func init() {
	accountCmd.AddCommand(accountUpdateCmd)
}
