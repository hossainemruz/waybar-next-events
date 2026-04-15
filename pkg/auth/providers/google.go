// Package providers contains OAuth2 provider implementations.
package providers

import (
	"github.com/hossainemruz/waybar-next-events/internal/config"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
)

const (
	// googleAuthURL is Google's OAuth2 authorization endpoint.
	googleAuthURL = "https://accounts.google.com/o/oauth2/auth"
	// googleTokenURL is Google's OAuth2 token endpoint.
	googleTokenURL = "https://oauth2.googleapis.com/token"
)

// Google implements the auth.Provider interface for Google OAuth2.
// It supports Google Calendar and other Google APIs.
type Google struct {
	clientID     string
	clientSecret string
	scopes       []string
}

// NewGoogle creates a new Google OAuth2 provider.
// clientID is required. clientSecret may be empty for public clients.
// If scopes is empty, default calendar read-only scope is used.
func NewGoogle(clientID, clientSecret string, scopes []string) *Google {
	if len(scopes) == 0 {
		scopes = []string{calendar.CalendarReadonlyScope}
	}
	return &Google{
		clientID:     clientID,
		clientSecret: clientSecret,
		scopes:       scopes,
	}
}

// Name returns a unique identifier for the provider.
// The clientId is used as the identity so that each Google account gets
// its own token storage key, ensuring independent OAuth sessions per account.
func (g *Google) Name() string {
	return g.clientID
}

// ClientID returns the OAuth2 client ID.
func (g *Google) ClientID() string {
	return g.clientID
}

// ClientSecret returns the OAuth2 client secret.
func (g *Google) ClientSecret() string {
	return g.clientSecret
}

// AuthURL returns the authorization endpoint URL.
func (g *Google) AuthURL() string {
	return googleAuthURL
}

// TokenURL returns the token endpoint URL.
func (g *Google) TokenURL() string {
	return googleTokenURL
}

// RedirectURL returns the callback URL.
func (g *Google) RedirectURL() string {
	return config.DefaultCallbackURL
}

// Scopes returns the OAuth2 scopes.
func (g *Google) Scopes() []string {
	return g.scopes
}

// AuthCodeOptions returns additional authorization URL parameters.
// For Google, this includes access_type=offline to get a refresh token
// and prompt=consent to ensure the refresh token is issued.
func (g *Google) AuthCodeOptions() []oauth2.AuthCodeOption {
	return []oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
	}
}

// ExchangeOptions returns additional token exchange parameters.
// Google does not require additional exchange options.
func (g *Google) ExchangeOptions() []oauth2.AuthCodeOption {
	return nil
}
