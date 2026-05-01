package app

import "errors"

var (
	// ErrCalendarSelectionRequired indicates a workflow expected a calendar selector.
	ErrCalendarSelectionRequired = errors.New("calendar selection is required")
)
