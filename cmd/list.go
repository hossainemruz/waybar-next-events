package cmd

import (
	"github.com/spf13/cobra"
)

var listLimit int

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Print upcoming calendar events",
	Long:  "Retrieve and display upcoming calendar events. Use --limit to control how many events are shown.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	listCmd.Flags().IntVar(&listLimit, "limit", 5, "Maximum number of calendar events to show")
	rootCmd.AddCommand(listCmd)
}
