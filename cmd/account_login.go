package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// accountLoginCmd re-authenticates a Google Calendar account via the browser OAuth2 flow.
var accountLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Re-authenticate a Google Calendar account",
	Long:  "Interactively re-authenticate a Google Calendar account by selecting an account and completing the browser-based OAuth2 login flow.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("account login: not yet implemented")
	},
}

func init() {
	accountCmd.AddCommand(accountLoginCmd)
}