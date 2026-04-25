package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// accountDeleteCmd deletes a Google Calendar account via an interactive confirmation flow.
var accountDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a Google Calendar account",
	Long:  "Interactively delete a Google Calendar account by selecting an account and confirming deletion.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("account delete: not yet implemented")
	},
}

func init() {
	accountCmd.AddCommand(accountDeleteCmd)
}
