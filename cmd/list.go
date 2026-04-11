package cmd

import (
	"fmt"

	"github.com/hossainemruz/waybar-next-events/pkg/calendars"
	"github.com/hossainemruz/waybar-next-events/pkg/types"
	"github.com/spf13/cobra"
)

var listLimit int

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Print upcoming calendar events",
	Long:  "Retrieve and display upcoming calendar events. Use --limit to control how many events are shown.",
	RunE: func(cmd *cobra.Command, args []string) error {
		events, err := calendars.GoogleEvents()
		if err != nil {
			return err
		}
		data := types.Result{
			Events: events,
		}
		if err := data.Print(); err != nil {
			fmt.Printf("{\"text\": \" Something went wrong!\", \"tooltip\": \"%s\"}\n", err)
		}
		return nil
	},
}

func init() {
	listCmd.Flags().IntVar(&listLimit, "limit", 5, "Maximum number of calendar events to show")
	rootCmd.AddCommand(listCmd)
}
