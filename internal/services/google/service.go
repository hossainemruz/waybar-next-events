package google

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/auth/providers"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	googlecalendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const (
	clientIDKey     = "client_id"
	clientSecretKey = "client_secret"
)

// Service implements the Google calendar app service.
type Service struct{}

// NewService creates a Google service adapter.
func NewService() *Service {
	return &Service{}
}

func (s *Service) Type() calendar.ServiceType {
	return calendar.ServiceTypeGoogle
}

func (s *Service) DisplayName() string {
	return "Google"
}

func (s *Service) AccountFields() []calendar.AccountField {
	return []calendar.AccountField{
		{Key: clientIDKey, Label: "OAuth Client ID", Required: true},
		{Key: clientSecretKey, Label: "OAuth Client Secret", Required: true, Secret: true},
	}
}

func (s *Service) Provider(ctx context.Context, account calendar.Account, secretStore secrets.Store) (providers.Provider, error) {
	if strings.TrimSpace(account.ID) == "" {
		return nil, fmt.Errorf("account ID cannot be empty")
	}

	clientSecret, err := secretStore.Get(ctx, account.ID, clientSecretKey)
	if err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			return nil, fmt.Errorf("missing stored secret %q", clientSecretKey)
		}
		return nil, fmt.Errorf("load stored secret %q: %w", clientSecretKey, err)
	}

	return providers.NewGoogle(
		account.ID,
		account.Setting(clientIDKey),
		clientSecret,
		[]string{googlecalendar.CalendarReadonlyScope},
	), nil
}

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

	sort.Slice(events, func(i, j int) bool {
		return events[i].Start.Before(events[j].Start)
	})

	return events, nil
}

func calendarIDs(account calendar.Account) []string {
	ids := account.CalendarIDs()
	if len(ids) == 0 {
		return []string{"primary"}
	}

	return ids
}

func convertGoogleEvents(gEvents []*googlecalendar.Event, dayLimit int, today time.Time) ([]calendar.Event, error) {
	events := make([]calendar.Event, 0)
	for _, item := range gEvents {
		if item == nil {
			continue
		}

		title := item.Summary
		if title == "" {
			title = "<Event title missing>"
		}

		eventStartTime, eventEndTime, err := parseEventTime(*item)
		if err != nil {
			return nil, err
		}

		if isMultiDayEvent(eventStartTime, eventEndTime) {
			for offset := range dayLimit {
				date := today.AddDate(0, 0, offset).Format(time.DateOnly)
				dayStart, err := startOfDate(date)
				if err != nil {
					return nil, err
				}
				dayEnd, err := endOfDate(date)
				if err != nil {
					return nil, err
				}
				if !eventStartToday(eventStartTime, dayEnd) {
					continue
				}
				if eventEnded(eventEndTime, dayStart) {
					break
				}

				event := calendar.Event{Title: title, Start: dayStart, End: dayEnd}
				if eventStartTime.After(dayStart) {
					event.Start = eventStartTime
				}
				if eventEndTime.Before(dayEnd) {
					event.End = eventEndTime
				}
				events = append(events, event)
			}
			continue
		}

		events = append(events, calendar.Event{Title: title, Start: eventStartTime, End: eventEndTime})
	}

	return events, nil
}

func startOfDate(date string) (time.Time, error) {
	return time.ParseInLocation(time.DateOnly, date, time.Now().Location())
}

func endOfDate(date string) (time.Time, error) {
	t, err := time.ParseInLocation(time.DateOnly, date, time.Now().Location())
	if err != nil {
		return t, err
	}

	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, calendar.EndOfDayNano, t.Location()), nil
}

func parseEventTime(event googlecalendar.Event) (time.Time, time.Time, error) {
	if event.Start == nil {
		return time.Time{}, time.Time{}, fmt.Errorf("event has nil Start")
	}
	if event.End == nil {
		return time.Time{}, time.Time{}, fmt.Errorf("event has nil End")
	}

	var start time.Time
	var end time.Time
	var err error

	if event.Start.DateTime != "" {
		start, err = time.Parse(time.RFC3339, event.Start.DateTime)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	} else {
		start, err = startOfDate(event.Start.Date)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	if event.End.DateTime != "" {
		end, err = time.Parse(time.RFC3339, event.End.DateTime)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	} else {
		if event.Start.Date == event.End.Date {
			end, err = endOfDate(event.End.Date)
		} else {
			day, parseErr := time.ParseInLocation(time.DateOnly, event.End.Date, time.Now().Location())
			if parseErr != nil {
				return time.Time{}, time.Time{}, parseErr
			}
			end, err = endOfDate(day.AddDate(0, 0, -1).Format(time.DateOnly))
		}
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	return start, end, nil
}

func isMultiDayEvent(start, end time.Time) bool {
	return end.Sub(start) > 24*time.Hour+1*time.Minute
}

func eventStartToday(eventStartTime, dayEnd time.Time) bool {
	return eventStartTime.Before(dayEnd)
}

func eventEnded(eventEndTime, dayStart time.Time) bool {
	return eventEndTime.Add(-1 * time.Minute).Before(dayStart)
}
