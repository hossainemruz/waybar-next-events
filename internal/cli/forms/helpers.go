package forms

import (
	"context"
	"errors"
	"io"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

// ConfigureForm applies input, output, and accessibility settings to a form.
func ConfigureForm(form *huh.Form, input io.Reader, output io.Writer, accessible bool) *huh.Form {
	if output != nil {
		form = form.WithOutput(output)
	}
	if input != nil {
		form = form.WithInput(input)
	}
	if accessible {
		form = form.WithAccessible(true)
	}
	return form
}

// IsUserAbort reports whether an error represents the user canceling a form.
func IsUserAbort(err error) bool {
	return errors.Is(err, huh.ErrUserAborted) || errors.Is(err, context.Canceled)
}

// ToCalendarRefs maps selected calendar IDs back to CalendarRef values.
func ToCalendarRefs(calendars []calendar.Calendar, selectedIDs []string) []calendar.CalendarRef {
	selected := make(map[string]struct{}, len(selectedIDs))
	for _, id := range selectedIDs {
		selected[id] = struct{}{}
	}

	refs := make([]calendar.CalendarRef, 0, len(selectedIDs))
	for _, cal := range calendars {
		if _, ok := selected[cal.ID]; !ok {
			continue
		}
		refs = append(refs, calendar.CalendarRef{ID: cal.ID, Name: cal.Name})
	}

	if len(refs) == 0 {
		return []calendar.CalendarRef{}
	}
	return refs
}
