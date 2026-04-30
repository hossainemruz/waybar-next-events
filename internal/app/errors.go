package app

import "errors"

var (
	// ErrAccountSelectionRequired indicates a workflow expected a calendar selector.
	ErrAccountSelectionRequired = errors.New("calendar selection is required")
)
