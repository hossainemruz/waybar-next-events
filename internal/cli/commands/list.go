package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/output"
	"github.com/spf13/cobra"
)

var listLimit int

type listEventFetcher interface {
	Fetch(ctx context.Context, query calendar.EventQuery, limit int) ([]calendar.Event, error)
}

type listDeps struct {
	now     func() time.Time
	fetcher listEventFetcher
	render  func([]calendar.Event, time.Time) ([]byte, error)
}

func buildListCmd(fetcher listEventFetcher) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Print upcoming calendar events",
		Long:  "Retrieve and display upcoming calendar events. Use --limit to control how many events are shown.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, listDeps{
				now:     time.Now,
				fetcher: fetcher,
				render:  output.Render,
			})
		},
	}

	cmd.Flags().IntVar(&listLimit, "limit", 5, "Maximum number of calendar events to show")
	return cmd
}

func runList(cmd *cobra.Command, deps listDeps) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
		cmd.SetContext(ctx)
	}

	now := deps.now()
	events, err := deps.fetcher.Fetch(ctx, calendar.EventQuery{Now: now, DayLimit: 4}, listLimit)
	if err != nil {
		return err
	}

	payload, err := deps.render(events, now)
	if err != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "{\"text\": \" Something went wrong!\", \"tooltip\": \"%s\"}\n", err)
		return nil
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(payload))
	return nil
}
