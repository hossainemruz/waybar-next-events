// Package config defines sentinel errors for configuration validation.
// These errors can be matched using errors.Is() for programmatic error handling.
package config

import "errors"

var (
	// ErrNoGoogleCalendar indicates that no google section was found in the config.
	ErrNoGoogleCalendar = errors.New("no google calendar configured")
	// ErrAccountMissingClientID indicates that a google account is missing the required clientId field.
	ErrAccountMissingClientID = errors.New("google calendar: account missing required field: clientId")
)
