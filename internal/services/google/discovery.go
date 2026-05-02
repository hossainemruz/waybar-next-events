package google

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	googlecalendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// DiscoverCalendars lists calendars available to the authenticated account.
func (s *Service) DiscoverCalendars(ctx context.Context, account calendar.Account, client *http.Client) ([]calendar.Calendar, error) {
	srv, err := googlecalendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("create calendar service: %w", err)
	}

	calendarList, err := srv.CalendarList.List().Do()
	if err != nil {
		return nil, fmt.Errorf("list calendars for account %q: %w", account.Name, err)
	}

	calendars := make([]calendar.Calendar, 0, len(calendarList.Items))
	for _, item := range calendarList.Items {
		calendars = append(calendars, calendar.Calendar{
			ID:      item.Id,
			Name:    item.Summary,
			Primary: item.Primary,
		})
	}

	return calendars, nil
}
