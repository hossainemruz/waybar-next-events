package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/output"
	"github.com/spf13/cobra"
)

const (
	defaultLookAheadDays   = 4
	defaultEventCountLimit = 10
	listCommandTimeout     = 30 * time.Second
)

type listEventFetcher interface {
	Fetch(ctx context.Context, query calendar.EventQuery, limit int) ([]calendar.Event, error)
}

type listOptions struct {
	days  int
	limit int
}

type listDeps struct {
	listOptions
	now     func() time.Time
	fetcher listEventFetcher
	render  func([]calendar.Event, time.Time) ([]byte, error)
}

func buildListCmd(deps *AppDeps) *cobra.Command {
	opts := listOptions{
		days:  defaultLookAheadDays,
		limit: defaultEventCountLimit,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Print upcoming calendar events",
		Long:  "Retrieve and display upcoming calendar events. Use --limit to control how many events are shown and --days to set the look-ahead window.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), listCommandTimeout)
			defer cancel()
			cmd.SetContext(ctx)

			return runList(cmd, listDeps{
				listOptions: opts,
				now:         time.Now,
				fetcher:     deps.EventFetcher,
				render:      output.Render,
			})
		},
	}

	cmd.Flags().IntVar(&opts.days, "days", defaultLookAheadDays, "Number of days to look ahead")
	cmd.Flags().IntVar(&opts.limit, "limit", defaultEventCountLimit, "Maximum number of calendar events to show")
	return cmd
}

func runList(cmd *cobra.Command, deps listDeps) error {
	if deps.days <= 0 {
		return fmt.Errorf("--days must be a positive integer, got %d", deps.days)
	}
	if deps.limit <= 0 {
		return fmt.Errorf("--limit must be a positive integer, got %d", deps.limit)
	}

	ctx := cmd.Context()

	now := deps.now()
	events, err := deps.fetcher.Fetch(ctx, calendar.EventQuery{Now: now, DayLimit: deps.days}, deps.limit)
	if err != nil {
		if errors.Is(err, appconfig.ErrNoAccounts) {
			return fmt.Errorf("%w: %s", appconfig.ErrNoAccounts, noAccountsConfiguredHint)
		}
		return err
	}

	payload, err := deps.render(events, now)
	if err != nil {
		fallback, _ := json.Marshal(struct {
			Text    string `json:"text"`
			Tooltip string `json:"tooltip"`
		}{
			Text:    " Something went wrong!",
			Tooltip: err.Error(),
		})
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(fallback))
		return nil
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(payload))
	return nil
}
