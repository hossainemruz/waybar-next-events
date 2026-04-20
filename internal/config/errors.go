// Package config defines sentinel errors for configuration validation.
// These errors can be matched using errors.Is() for programmatic error handling.
package config

import "errors"

var (
	// ErrNoGoogleCalendar indicates that no google section was found in the config.
	ErrNoGoogleCalendar = errors.New("no google calendar configured")
	// ErrAccountMissingClientID indicates that a google account is missing the required clientId field.
	ErrAccountMissingClientID = errors.New("google calendar: account missing required field: clientId")
	// ErrNoAccounts indicates that no accounts are configured.
	ErrNoAccounts = errors.New("no accounts configured")
	// ErrAccountNotFound indicates that the specified account name was not found.
	ErrAccountNotFound = errors.New("account not found")
	// ErrDuplicateAccountName indicates that an account with the given name already exists.
	ErrDuplicateAccountName = errors.New("account name already exists")
)

// ErrConfigNotFound is returned when the config file does not exist.
type ErrConfigNotFound struct {
	Path string
}

func (e *ErrConfigNotFound) Error() string {
	return "config file not found at " + e.Path
}
