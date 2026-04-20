// Package config contains shared configuration constants for the application.
package config

const (
	// DefaultCallbackPort is the default port for the OAuth2 callback server.
	// This port is used by the local HTTP server that receives OAuth2 callbacks.
	DefaultCallbackPort = "18751"

	// DefaultCallbackURL is the full OAuth2 redirect callback URL.
	// Providers must use this exact URL as their RedirectURL to ensure the
	// callback server can receive the redirect. This constant centralizes the
	// contract between provider validation and the callback server implementation.
	DefaultCallbackURL = "http://127.0.0.1:" + DefaultCallbackPort + "/callback"
)

// Config represents the top-level configuration structure.
// It is designed to be extensible: additional calendar service providers
// (e.g., "outlook") can be added as new top-level fields.
type Config struct {
	Google *GoogleCalendar `json:"google"`
}
