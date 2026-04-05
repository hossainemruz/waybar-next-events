package types

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
)

type waybarOutput struct {
	Text    string `json:"text"`
	Tooltip string `json:"tooltip"`
}

type Result struct {
	Events []Event
}

func (r *Result) Print() error {
	now := time.Now()
	groups := groupEventsByDay(r.Events, now)

	output := waybarOutput{}

	// --- Determine Text (bar display) ---
	// Find today's events from the grouped data
	var todayEvents []Event
	if len(groups) > 0 && groups[0].Day == "Today" {
		todayEvents = groups[0].Events
	}

	// Find ongoing event (skip all-day events; if multiple, pick the latest-started)
	var ongoingEvent *Event
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
		output.Text = fmt.Sprintf("󰺏 %s (ends in %s)", html.EscapeString(ongoingEvent.Title), formatDuration(remaining))
	} else {
		// Find next upcoming event for today (skip all-day events)
		var nextEvent *Event
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
			output.Text = fmt.Sprintf("󰃰 %s (starts in %s)", html.EscapeString(nextEvent.Title), formatDuration(until))
		} else {
			output.Text = " No more events today!"
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
	output.Tooltip = strings.Join(tooltipEntries, "\n")

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		return err
	}
	fmt.Println(string(jsonBytes))
	return nil
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

// groupEventsByDay sorts events by start time and groups them by calendar day.
// Day labels: "Today", "Tomorrow", then weekday name (e.g. "Monday").
func groupEventsByDay(events []Event, now time.Time) []EventsGroup {
	sort.Slice(events, func(i, j int) bool {
		return events[i].Start.Before(events[j].Start)
	})

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.AddDate(0, 0, 1)

	grouped := make(map[string]*EventsGroup)
	var dayOrder []string

	for _, e := range events {
		eventDay := time.Date(e.Start.Year(), e.Start.Month(), e.Start.Day(), 0, 0, 0, 0, e.Start.Location())

		var label string
		switch {
		case eventDay.Equal(today):
			label = "Today"
		case eventDay.Equal(tomorrow):
			label = "Tomorrow"
		default:
			label = eventDay.Weekday().String()
		}

		if _, exists := grouped[label]; !exists {
			grouped[label] = &EventsGroup{Day: label}
			dayOrder = append(dayOrder, label)
		}
		g := grouped[label]
		g.Events = append(g.Events, e)
	}

	var result []EventsGroup
	for _, label := range dayOrder {
		result = append(result, *grouped[label])
	}
	return result
}
