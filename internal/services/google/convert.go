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
				if !eventStartsBeforeDayEnd(eventStartTime, dayEnd) {
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
		start, err = parseRFC3339(event.Start.DateTime)
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
		end, err = parseRFC3339(event.End.DateTime)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	} else {
		// Google Calendar uses exclusive end dates for all-day events
		// (e.g. a single-day event on Jun 15 has End.Date = "Jun 16").
		// Per the API spec, Start.Date and End.Date should never be equal for
		// well-formed data. When they are equal, this is treated as a defensive
		// fallback: the event is interpreted as a full day on that date.
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

// parseRFC3339 parses an RFC 3339 timestamp, accepting both fractional-second
// and non-fractional-second formats. It tries time.RFC3339Nano first (which
// handles sub-second precision), then falls back to time.RFC3339.
func parseRFC3339(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

// isMultiDayEvent returns true when an event spans more than 24 hours plus a
// 1-minute tolerance. The tolerance accounts for events that are technically
// slightly longer than 24h due to clock granularity or provider-side rounding.
//
// Note: A timed event that starts and ends at exactly midnight (duration ==
// 24h) is NOT classified as multi-day. Such events are treated as a single
// same-day occurrence. If the intent is to classify exactly-24h events as
// multi-day, the threshold should be changed to >= 24*time.Hour.
func isMultiDayEvent(start, end time.Time) bool {
	return end.Sub(start) > 24*time.Hour+1*time.Minute
}

// eventStartsBeforeDayEnd returns true when the event starts before the end
// of the given day. The name clarifies that this checks a boundary condition
// against an arbitrary day, not specifically "today".
func eventStartsBeforeDayEnd(eventStartTime, dayEnd time.Time) bool {
	return eventStartTime.Before(dayEnd)
}

// eventEnded returns true when the event has ended before the given dayStart.
// The 1-minute buffer subtracts from eventEndTime to handle boundary edge
// cases: events whose computed End time lands exactly at dayStart (e.g. an
// all-day event whose inclusive end was the last nanosecond of the previous
// day) are not incorrectly treated as continuing into the new day.
//
// Tradeoff: events that genuinely end within 1 minute after midnight will be
// excluded from that day's display. This is acceptable because sub-minute end
// times past midnight are rare in practice and the buffer prevents far more
// common all-day boundary misclassifications.
func eventEnded(eventEndTime, dayStart time.Time) bool {
	return eventEndTime.Add(-1 * time.Minute).Before(dayStart)
}
