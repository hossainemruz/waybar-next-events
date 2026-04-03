package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Set up authentication credentials",
	Long:  "Configure authentication credentials for accessing your calendar provider.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("auth: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
}
