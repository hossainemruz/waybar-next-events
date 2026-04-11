package calendars

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hossainemruz/waybar-next-events/pkg/auth"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/providers"
	"github.com/hossainemruz/waybar-next-events/pkg/types"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// GoogleCalendarClientIDEnv is the environment variable name for Google OAuth client ID.
const GoogleCalendarClientIDEnv = "GOOGLE_CALENDAR_CLIENT_ID"

// GoogleCalendarClientSecretEnv is the environment variable name for Google OAuth client secret.
const GoogleCalendarClientSecretEnv = "GOOGLE_CALENDAR_CLIENT_SECRET"

// getCalendarClient returns a Google Calendar service client with automatic
// authentication and token refresh using the keyring-backed auth package.
func getCalendarClient() (*calendar.Service, error) {
	ctx := context.Background()

	// Get credentials from environment
	clientID := os.Getenv(GoogleCalendarClientIDEnv)
	if clientID == "" {
		return nil, fmt.Errorf("environment variable %s not set", GoogleCalendarClientIDEnv)
	}

	clientSecret := os.Getenv(GoogleCalendarClientSecretEnv)
	// clientSecret may be empty for public clients

	// Create Google OAuth provider
	googleProvider := providers.NewGoogle(
		clientID,
		clientSecret,
		[]string{calendar.CalendarReadonlyScope},
	)

	// Create authenticator with default keyring store
	authenticator := auth.NewAuthenticator(nil)

	// Get HTTP client with automatic token refresh
	client, err := authenticator.HTTPClient(ctx, googleProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated client: %w", err)
	}

	return calendar.NewService(ctx, option.WithHTTPClient(client))
}

// GoogleEvents retrieves upcoming calendar events from Google Calendar.
// It returns events for the next 4 days.
func GoogleEvents() ([]types.Event, error) {
	srv, err := getCalendarClient()
	if err != nil {
		return nil, err
	}

	dayLimit := 4
	today := time.Now()
	minDay, err := startOfDate(today.Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	maxDay, err := endOfDate(today.AddDate(0, 0, dayLimit-1).Format(time.DateOnly))
	if err != nil {
		return nil, err
	}

	// List upcoming events
	events, err := srv.Events.List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(minDay.Format(time.RFC3339)).
		TimeMax(maxDay.Format(time.RFC3339)).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch calendar events: %w", err)
	}

	// Convert google events to types.Event
	return convertGoogleEvents(events.Items, dayLimit, today)
}

// GogoleEvent is an alias for GoogleEvents for backward compatibility.
// Deprecated: Use GoogleEvents() instead.
func GogoleEvent() ([]types.Event, error) {
	return GoogleEvents()
}

func convertGoogleEvents(gEvents []*calendar.Event, dayLimit int, today time.Time) ([]types.Event, error) {
	events := make([]types.Event, 0)
	for _, item := range gEvents {
		title := item.Summary
		if title == "" {
			title = "<Event title missing>"
		}

		eventStartTime, eventEndTime, err := parseEventTime(*item)
		if err != nil {
			return nil, err
		}
		// For multi-day event, add one entry per day.
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
				event := types.Event{
					Title: title,
					Start: dayStart,
					End:   dayEnd,
				}
				if eventStartTime.After(dayStart) {
					event.Start = eventStartTime
				}
				if eventEndTime.Before(dayEnd) {
					event.End = eventEndTime
				}
				events = append(events, event)
			}
		} else {
			events = append(events, types.Event{
				Title: title,
				Start: eventStartTime,
				End:   eventEndTime,
			})
		}
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
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, types.EndOfDayNano, t.Location()), nil
}

func parseEventTime(e calendar.Event) (start time.Time, end time.Time, err error) {
	if e.Start == nil {
		return start, end, fmt.Errorf("event has nil Start")
	}
	if e.End == nil {
		return start, end, fmt.Errorf("event has nil End")
	}

	// Parse event start time.
	if e.Start.DateTime != "" {
		// Both date and time specified.
		start, err = time.Parse(time.RFC3339, e.Start.DateTime)
		if err != nil {
			return start, end, err
		}
	} else {
		// Only date provided but not time. In this case, we set time to start of the day.
		start, err = startOfDate(e.Start.Date)
		if err != nil {
			return start, end, err
		}
	}
	// Parse event end time.
	if e.End.DateTime != "" {
		// Both date and time specified.
		end, err = time.Parse(time.RFC3339, e.End.DateTime)
		if err != nil {
			return start, end, err
		}
	} else {
		// Only date provided but not time.
		// Google Calendar uses exclusive end dates for all-day events
		// (e.g. a single-day event on Jun 15 has End.Date = "Jun 16").
		// When start and end dates are the same, the event is a full day on that date.
		if e.Start.Date == e.End.Date {
			end, err = endOfDate(e.End.Date)
		} else {
			day, err := time.ParseInLocation(time.DateOnly, e.End.Date, time.Now().Location())
			if err != nil {
				return start, end, err
			}
			end, err = endOfDate(day.AddDate(0, 0, -1).Format(time.DateOnly))
		}
		if err != nil {
			return start, end, err
		}
	}
	return start, end, nil
}

func isMultiDayEvent(start, end time.Time) bool {
	// An event is multi-day if its duration exceeds 24 hours and 1 minute.
	return end.Sub(start) > 24*time.Hour+1*time.Minute
}

func eventStartToday(eventStartTime, dayEnd time.Time) bool {
	return eventStartTime.Before(dayEnd)
}

func eventEnded(eventEndTime, dayStart time.Time) bool {
	return eventEndTime.Add(-1 * time.Minute).Before(dayStart)
}
