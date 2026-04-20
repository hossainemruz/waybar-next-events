package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// accountAddCmd adds a new Google Calendar account via an interactive form.
var accountAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new Google Calendar account",
	Long:  "Interactively add a new Google Calendar account by entering credentials, authenticating via OAuth2, and selecting calendars.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("account add: not yet implemented")
	},
}

func init() {
	accountCmd.AddCommand(accountAddCmd)
}