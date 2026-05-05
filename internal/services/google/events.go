package google

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	googlecalendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// FetchEvents retrieves events from the selected calendars for the given account.
func (s *Service) FetchEvents(ctx context.Context, account calendar.Account, query calendar.EventQuery, client *http.Client) ([]calendar.Event, error) {
	srv, err := googlecalendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("create calendar service: %w", err)
	}

	loc := query.Now.Location()
	minDay, err := startOfDate(query.Now.Format(time.DateOnly), loc)
	if err != nil {
		return nil, err
	}
	maxDay, err := endOfDate(query.Now.AddDate(0, 0, query.DayLimit-1).Format(time.DateOnly), loc)
	if err != nil {
		return nil, err
	}

	events := make([]calendar.Event, 0)
	ids, fallback := calendarIDs(account)
	if fallback {
		slog.Warn("no calendars selected for account, falling back to primary", "account", account.Name)
	}
	for _, calendarID := range ids {
		var allItems []*googlecalendar.Event
		pageToken := ""
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			call := srv.Events.List(calendarID).
				ShowDeleted(false).
				SingleEvents(true).
				TimeMin(minDay.Format(time.RFC3339)).
				TimeMax(maxDay.Format(time.RFC3339)).
				OrderBy("startTime").
				TimeZone(loc.String()).
				MaxResults(2500)
			if pageToken != "" {
				call.PageToken(pageToken)
			}

			response, err := call.Do()
			if err != nil {
				if fallback {
					return nil, fmt.Errorf("no calendars selected for account %q and failed to fetch from the default primary calendar: %w", account.Name, err)
				}
				return nil, fmt.Errorf("fetch events for calendar %q from account %q: %w", calendarID, account.Name, err)
			}

			allItems = append(allItems, response.Items...)

			if response.NextPageToken == "" {
				break
			}
			pageToken = response.NextPageToken
		}

		converted, err := convertGoogleEvents(allItems, query.DayLimit, query.Now)
		if err != nil {
			return nil, err
		}
		events = append(events, converted...)
	}

	return events, nil
}

func calendarIDs(account calendar.Account) ([]string, bool) {
	ids := account.CalendarIDs()
	if len(ids) == 0 {
		return []string{"primary"}, true
	}
	return ids, false
}
