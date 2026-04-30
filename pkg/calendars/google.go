package calendars

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/auth"
	"github.com/hossainemruz/waybar-next-events/internal/auth/providers"
	appcalendar "github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"github.com/hossainemruz/waybar-next-events/pkg/types"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// defaultAuthenticator is a shared authenticator instance for calendar operations.
// This avoids creating a new authenticator on every call, which would miss any
// future in-memory token cache optimizations.
var defaultAuthenticator = auth.NewAuthenticator(nil)

// defaultSecretStore is the shared secrets store used by calendar operations.
var defaultSecretStore = secrets.NewKeyringStore()

// DiscoverCalendarsWithAuthenticator authenticates with the given Google
// account using the provided authenticator and fetches the list of available
// calendars. It returns a slice of DiscoveredCalendar values containing both
// the config-compatible calendar data and the Primary flag from the Google
// Calendar API. This lets interactive account-management flows stage tokens
// until the full flow succeeds while still reusing the shared discovery path.
//
// If the account has no calendars, an empty slice is returned.
func DiscoverCalendarsWithAuthenticator(ctx context.Context, account *config.Account, secretStore secrets.Store, authenticator *auth.Authenticator) ([]DiscoveredCalendar, error) {
	googleProvider, err := GoogleProviderForAccount(ctx, account, secretStore)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google provider for account %q: %w", account.Name, err)
	}

	client, err := authenticator.HTTPClient(ctx, googleProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate account %q: %w", account.Name, err)
	}

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create calendar service: %w", err)
	}

	calendarList, err := srv.CalendarList.List().Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list calendars: %w", err)
	}

	calendars := make([]DiscoveredCalendar, 0, len(calendarList.Items))
	for _, item := range calendarList.Items {
		calendars = append(calendars, DiscoveredCalendar{
			Calendar: config.CalendarRef{
				Name: item.Summary,
				ID:   item.Id,
			},
			Primary: item.Primary,
		})
	}

	return calendars, nil
}

// DiscoveredCalendar represents a calendar discovered from the Google Calendar API,
// including the Primary flag which is useful for display but not stored in config.
type DiscoveredCalendar struct {
	Calendar config.CalendarRef
	Primary  bool
}

// GoogleProviderForAccount builds a Google auth provider using non-secret
// account settings plus secrets loaded from the configured secret store.
func GoogleProviderForAccount(ctx context.Context, account *config.Account, secretStore secrets.Store) (*providers.Google, error) {
	if account == nil {
		return nil, fmt.Errorf("account cannot be nil")
	}
	if strings.TrimSpace(account.ID) == "" {
		return nil, fmt.Errorf("account ID cannot be empty")
	}

	clientSecret, err := secretStore.Get(ctx, account.ID, "client_secret")
	if err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			return nil, fmt.Errorf("missing stored secret %q", "client_secret")
		}
		return nil, fmt.Errorf("failed to load stored secret %q: %w", "client_secret", err)
	}

	return providers.NewGoogle(
		account.ID,
		account.Setting("client_id"),
		clientSecret,
		[]string{calendar.CalendarReadonlyScope},
	), nil
}

// getCalendarServiceForAccount returns a Google Calendar service client with
// automatic authentication and token refresh for the given account.
func getCalendarServiceForAccount(account *config.Account, secretStore secrets.Store) (*calendar.Service, error) {
	ctx := context.Background()

	googleProvider, err := GoogleProviderForAccount(ctx, account, secretStore)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google provider for account %q: %w", account.Name, err)
	}

	// Get HTTP client with automatic token refresh
	client, err := defaultAuthenticator.HTTPClient(ctx, googleProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated client for account %q: %w", account.Name, err)
	}

	return calendar.NewService(ctx, option.WithHTTPClient(client))
}

// GoogleEvents retrieves upcoming calendar events from all configured
// Google accounts and their calendars. It returns events for the next 4 days,
// sorted by start time.
func GoogleEvents() ([]types.Event, error) {
	// Load configuration from file
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Get Google calendar configuration
	googleCfg := cfg.AccountsByService(appcalendar.ServiceTypeGoogle)

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

	var allEvents []types.Event

	for i := range googleCfg {
		account := &googleCfg[i]

		srv, err := getCalendarServiceForAccount(account, defaultSecretStore)
		if err != nil {
			return nil, err
		}

		calendarIDs := googleCalendarIDs(*account)
		for _, calID := range calendarIDs {
			events, err := srv.Events.List(calID).
				ShowDeleted(false).
				SingleEvents(true).
				TimeMin(minDay.Format(time.RFC3339)).
				TimeMax(maxDay.Format(time.RFC3339)).
				OrderBy("startTime").
				Do()
			if err != nil {
				return nil, fmt.Errorf("failed to fetch events for calendar %q from account %q: %w", calID, account.Name, err)
			}

			converted, err := convertGoogleEvents(events.Items, dayLimit, today)
			if err != nil {
				return nil, err
			}
			allEvents = append(allEvents, converted...)
		}
	}

	// If no events were found across all accounts and calendars, return an empty slice
	if allEvents == nil {
		allEvents = []types.Event{}
	}

	// Sort all events by start time across accounts and calendars
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Start.Before(allEvents[j].Start)
	})

	return allEvents, nil
}

func googleCalendarIDs(account config.Account) []string {
	ids := account.CalendarIDs()
	if len(ids) == 0 {
		return []string{"primary"}
	}

	return ids
}

func convertGoogleEvents(gEvents []*calendar.Event, dayLimit int, today time.Time) ([]types.Event, error) {
	events := make([]types.Event, 0)
	for _, item := range gEvents {
		// Skip nil items (defensive check against malformed API responses)
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
			day, parseErr := time.ParseInLocation(time.DateOnly, e.End.Date, time.Now().Location())
			if parseErr != nil {
				return start, end, parseErr
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
