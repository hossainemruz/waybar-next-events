// Package providers contains OAuth2 provider implementations and the Provider interface.
package providers

import (
	"fmt"

	"github.com/hossainemruz/waybar-next-events/internal/config"
	"golang.org/x/oauth2"
)

// Provider defines the interface for OAuth2 provider configuration.
// Each provider (Google, GitHub, etc.) implements this interface.
type Provider interface {
	// Name returns a unique, stable identifier for the provider.
	// This is used as the key for token storage.
	Name() string

	// ClientID returns the OAuth2 client ID.
	ClientID() string

	// ClientSecret returns the OAuth2 client secret.
	// May be empty for public clients.
	ClientSecret() string

	// AuthURL returns the authorization endpoint URL.
	AuthURL() string

	// TokenURL returns the token endpoint URL.
	TokenURL() string

	// RedirectURL returns the callback URL.
	// Must equal config.DefaultCallbackURL for the OAuth flow to work.
	RedirectURL() string

	// Scopes returns the OAuth2 scopes to request.
	Scopes() []string

	// AuthCodeOptions returns additional URL parameters for the authorization URL.
	// These are provider-specific (e.g., access_type=offline for Google).
	AuthCodeOptions() []oauth2.AuthCodeOption

	// ExchangeOptions returns additional parameters for the token exchange.
	// These are provider-specific options for the exchange call.
	ExchangeOptions() []oauth2.AuthCodeOption
}

// Validate checks that a provider configuration is valid and secure.
func Validate(p Provider) error {
	if p == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	if p.Name() == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	if p.ClientID() == "" {
		return fmt.Errorf("provider %s: client ID cannot be empty", p.Name())
	}

	if p.AuthURL() == "" {
		return fmt.Errorf("provider %s: auth URL cannot be empty", p.Name())
	}

	if p.TokenURL() == "" {
		return fmt.Errorf("provider %s: token URL cannot be empty", p.Name())
	}

	if p.RedirectURL() == "" {
		return fmt.Errorf("provider %s: redirect URL cannot be empty", p.Name())
	}

	// Security: the redirect URL must match the exact callback URL that the
	// local callback server serves. This prevents providers from validating
	// successfully with a redirect URL whose port or path does not match the
	// server, which would cause every OAuth flow to fail at runtime.
	if p.RedirectURL() != config.DefaultCallbackURL {
		return fmt.Errorf("provider %s: redirect URL must be %s, got %s", p.Name(), config.DefaultCallbackURL, p.RedirectURL())
	}

	return nil
}

// ToOAuth2Config converts a Provider to an oauth2.Config.
func ToOAuth2Config(p Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.ClientID(),
		ClientSecret: p.ClientSecret(),
		Endpoint: oauth2.Endpoint{
			AuthURL:  p.AuthURL(),
			TokenURL: p.TokenURL(),
		},
		RedirectURL: p.RedirectURL(),
		Scopes:      p.Scopes(),
	}
}
