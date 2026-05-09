package app

import "errors"

var (
	// ErrCalendarSelectionRequired indicates a workflow expected a calendar selector.
	ErrCalendarSelectionRequired = errors.New("calendar selection is required")
	// ErrNoCalendarsDiscovered indicates that calendar discovery returned no calendars.
	ErrNoCalendarsDiscovered = errors.New("no calendars discovered")
)
