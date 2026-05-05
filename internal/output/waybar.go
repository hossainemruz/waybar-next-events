package output

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

// WaybarPayload is the JSON shape emitted by Render.
type WaybarPayload struct {
	Text    string `json:"text"`
	Tooltip string `json:"tooltip"`
}

// Render formats the given events as Waybar JSON output for the given point in time.
// Events must already be sorted by start time (ascending); the caller (e.g. EventFetcher)
// is responsible for ordering. The returned bytes are the JSON payload; the caller decides
// how to emit them.
func Render(events []calendar.Event, now time.Time) ([]byte, error) {
	groups := groupEventsByDay(events, now)

	payload := WaybarPayload{}

	// --- Determine Text (bar display) ---
	var todayEvents []calendar.Event
	if len(groups) > 0 && groups[0].Day == "Today" {
		todayEvents = groups[0].Events
	}

	// Find ongoing event (skip all-day events; if multiple, pick the latest-started)
	var ongoingEvent *calendar.Event
	for i := range todayEvents {
		e := &todayEvents[i]
		if e.IsAllDay() {
			continue
		}
		if !e.Start.After(now) && e.End.After(now) {
			if ongoingEvent == nil || e.Start.After(ongoingEvent.Start) {
				ongoingEvent = e
			}
		}
	}

	if ongoingEvent != nil {
		remaining := ongoingEvent.End.Sub(now)
		payload.Text = fmt.Sprintf("󰺏 %s (ends in %s)", html.EscapeString(ongoingEvent.Title), formatDuration(remaining))
	} else {
		// Find next upcoming event for today (skip all-day events)
		var nextEvent *calendar.Event
		for i := range todayEvents {
			e := &todayEvents[i]
			if e.IsAllDay() {
				continue
			}
			if e.Start.After(now) {
				nextEvent = e
				break // events are sorted by start, so first match is the next one
			}
		}

		if nextEvent != nil {
			until := nextEvent.Start.Sub(now)
			payload.Text = fmt.Sprintf("󰃰 %s (starts in %s)", html.EscapeString(nextEvent.Title), formatDuration(until))
		} else {
			payload.Text = " No more events today!"
		}
	}

	// --- Build Tooltip ---
	var tooltipEntries []string
	for i, g := range groups {
		tooltipEntries = append(tooltipEntries, fmt.Sprintf("<b>%s</b>", g.Day))
		var line string
		for _, e := range g.Events {
			if e.IsAllDay() {
				line = fmt.Sprintf("%s              %s", "All day", html.EscapeString(e.Title))
			} else {
				line = fmt.Sprintf("%7s - %7s    %s", e.Start.Format("3:04PM"), e.End.Format("3:04PM"), html.EscapeString(e.Title))
			}
			// Past events for today get a check mark
			if g.Day == "Today" && !e.End.After(now) {
				line = line + " ✓"
			}
			tooltipEntries = append(tooltipEntries, line)
		}
		// Add separator between groups
		if i < len(groups)-1 {
			tooltipEntries = append(tooltipEntries, "")
		}
	}
	payload.Tooltip = strings.Join(tooltipEntries, "\n")

	return json.Marshal(payload)
}

// formatDuration formats a duration as a human-readable string like "1h 25m" or "45m".
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 && minutes > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return "0m"
}

// groupEventsByDay groups pre-sorted events by calendar day.
// Events are grouped by actual date so that events on different weeks are
// never collapsed into the same group. Day labels: "Today", "Tomorrow", then
// the weekday name (e.g. "Monday").
func groupEventsByDay(events []calendar.Event, now time.Time) []calendar.EventsGroup {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.AddDate(0, 0, 1)

	grouped := make(map[string]*calendar.EventsGroup)
	var dayOrder []string

	for _, e := range events {
		eventDay := time.Date(e.Start.Year(), e.Start.Month(), e.Start.Day(), 0, 0, 0, 0, e.Start.Location())
		dateKey := eventDay.Format("2006-01-02")

		var label string
		switch {
		case eventDay.Equal(today):
			label = "Today"
		case eventDay.Equal(tomorrow):
			label = "Tomorrow"
		default:
			label = eventDay.Weekday().String()
		}

		if _, exists := grouped[dateKey]; !exists {
			grouped[dateKey] = &calendar.EventsGroup{Day: label}
			dayOrder = append(dayOrder, dateKey)
		}
		g := grouped[dateKey]
		g.Events = append(g.Events, e)
	}

	result := make([]calendar.EventsGroup, 0, len(dayOrder))
	for _, key := range dayOrder {
		result = append(result, *grouped[key])
	}
	return result
}
