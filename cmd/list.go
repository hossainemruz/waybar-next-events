package cmd

import (
	"context"
	"fmt"

	"time"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/pkg/types"
	"github.com/spf13/cobra"
)

var listLimit int

type listDependencies struct {
	fetchEvents func(cmd *cobra.Command, query calendar.EventQuery, limit int) ([]calendar.Event, error)
}

var defaultListDependencies = listDependencies{
	fetchEvents: func(cmd *cobra.Command, query calendar.EventQuery, limit int) ([]calendar.Event, error) {
		return newEventFetcher().Fetch(cmd.Context(), query, limit)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Print upcoming calendar events",
	Long:  "Retrieve and display upcoming calendar events. Use --limit to control how many events are shown.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runList(cmd, defaultListDependencies)
	},
}

func runList(cmd *cobra.Command, deps listDependencies) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
		cmd.SetContext(ctx)
	}

	events, err := deps.fetchEvents(cmd, calendar.EventQuery{Now: time.Now(), DayLimit: 4}, listLimit)
	if err != nil {
		return err
	}
	data := types.Result{
		Events: calendarsToLegacyEvents(events),
	}
	if err := data.Print(); err != nil {
		fmt.Printf("{\"text\": \" Something went wrong!\", \"tooltip\": \"%s\"}\n", err)
	}
	return nil
}

func calendarsToLegacyEvents(events []calendar.Event) []types.Event {
	if len(events) == 0 {
		return []types.Event{}
	}

	legacy := make([]types.Event, len(events))
	copy(legacy, events)
	return legacy
}

func init() {
	listCmd.Flags().IntVar(&listLimit, "limit", 5, "Maximum number of calendar events to show")
	rootCmd.AddCommand(listCmd)
}
