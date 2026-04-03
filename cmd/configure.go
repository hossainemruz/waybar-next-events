package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure calendar and personalization options",
	Long:  "Select calendars and set personalization options such as display format and event filters.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("configure: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configureCmd)
}
