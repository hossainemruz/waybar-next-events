package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/output"
	"github.com/spf13/cobra"
)

var listLimit int

type listDependencies struct {
	now         func() time.Time
	fetchEvents func(cmd *cobra.Command, query calendar.EventQuery, limit int) ([]calendar.Event, error)
}

var defaultListDependencies = listDependencies{
	now: time.Now,
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

	now := deps.now()
	events, err := deps.fetchEvents(cmd, calendar.EventQuery{Now: now, DayLimit: 4}, listLimit)
	if err != nil {
		return err
	}

	payload, err := output.Render(events, now)
	if err != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "{\"text\": \" Something went wrong!\", \"tooltip\": \"%s\"}\n", err)
		return nil
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(payload))
	return nil
}

func init() {
	listCmd.Flags().IntVar(&listLimit, "limit", 5, "Maximum number of calendar events to show")
	rootCmd.AddCommand(listCmd)
}
