package google

import (
	"fmt"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	googlecalendar "google.golang.org/api/calendar/v3"
)

func convertGoogleEvents(gEvents []*googlecalendar.Event, dayLimit int, today time.Time) ([]calendar.Event, error) {
	loc := today.Location()
	events := make([]calendar.Event, 0)
	for _, item := range gEvents {
		if item == nil {
			continue
		}

		title := item.Summary
		if title == "" {
			title = "<Event title missing>"
		}

		eventStartTime, eventEndTime, err := parseEventTime(*item, loc)
		if err != nil {
			return nil, err
		}

		if isMultiDayEvent(eventStartTime, eventEndTime) {
			for offset := range dayLimit {
				date := today.AddDate(0, 0, offset).Format(time.DateOnly)
				dayStart, err := startOfDate(date, loc)
				if err != nil {
					return nil, err
				}
				dayEnd, err := endOfDate(date, loc)
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

func startOfDate(date string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation(time.DateOnly, date, loc)
}

func endOfDate(date string, loc *time.Location) (time.Time, error) {
	t, err := time.ParseInLocation(time.DateOnly, date, loc)
	if err != nil {
		return t, err
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, calendar.EndOfDayNano, t.Location()), nil
}

func parseEventTime(event googlecalendar.Event, loc *time.Location) (time.Time, time.Time, error) {
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
		start, err = startOfDate(event.Start.Date, loc)
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
		// Google Calendar uses exclusive end dates for all-day events
		// (e.g. a single-day event on Jun 15 has End.Date = "Jun 16").
		// When start and end dates are the same, the event is a full day on that date.
		if event.Start.Date == event.End.Date {
			end, err = endOfDate(event.End.Date, loc)
		} else {
			day, parseErr := time.ParseInLocation(time.DateOnly, event.End.Date, loc)
			if parseErr != nil {
				return time.Time{}, time.Time{}, parseErr
			}
			end, err = endOfDate(day.AddDate(0, 0, -1).Format(time.DateOnly), loc)
		}
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	return start, end, nil
}

// isMultiDayEvent returns true when an event spans more than 24 hours.
// The 1-minute margin provides a small tolerance for events that are
// technically slightly longer than 24h due to clock granularity or
// provider-side rounding, ensuring they are treated as multi-day.
func isMultiDayEvent(start, end time.Time) bool {
	return end.Sub(start) > 24*time.Hour+1*time.Minute
}

func eventStartToday(eventStartTime, dayEnd time.Time) bool {
	return eventStartTime.Before(dayEnd)
}

// eventEnded returns true when the event has ended before the given dayStart.
// The 1-minute margin provides a safety buffer for boundary comparisons so
// that events ending exactly at dayStart (e.g. an all-day event whose
// inclusive end time was computed as the last nanosecond of the previous day)
// are not incorrectly treated as continuing into the new day.
func eventEnded(eventEndTime, dayStart time.Time) bool {
	return eventEndTime.Add(-1 * time.Minute).Before(dayStart)
}
