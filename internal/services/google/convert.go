package google

import (
	"fmt"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	googlecalendar "google.golang.org/api/calendar/v3"
)

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
