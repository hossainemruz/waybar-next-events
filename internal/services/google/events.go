package google

import (
	"context"
	"fmt"
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

	minDay, err := startOfDate(query.Now.Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	maxDay, err := endOfDate(query.Now.AddDate(0, 0, query.DayLimit-1).Format(time.DateOnly))
	if err != nil {
		return nil, err
	}

	events := make([]calendar.Event, 0)
	for _, calendarID := range calendarIDs(account) {
		response, err := srv.Events.List(calendarID).
			ShowDeleted(false).
			SingleEvents(true).
			TimeMin(minDay.Format(time.RFC3339)).
			TimeMax(maxDay.Format(time.RFC3339)).
			OrderBy("startTime").
			Do()
		if err != nil {
			return nil, fmt.Errorf("fetch events for calendar %q from account %q: %w", calendarID, account.Name, err)
		}

		converted, err := convertGoogleEvents(response.Items, query.DayLimit, query.Now)
		if err != nil {
			return nil, err
		}
		events = append(events, converted...)
	}

	return events, nil
}

func calendarIDs(account calendar.Account) []string {
	ids := account.CalendarIDs()
	if len(ids) == 0 {
		return []string{"primary"}
	}
	return ids
}
