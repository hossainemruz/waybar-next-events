// Package providers contains OAuth2 provider implementations and the Provider interface.
package providers

import (
	"fmt"
	"net/url"

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
	// Must use 127.0.0.1 as the host.
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

	// Parse and validate redirect URL
	redirectURL, err := url.Parse(p.RedirectURL())
	if err != nil {
		return fmt.Errorf("provider %s: invalid redirect URL: %w", p.Name(), err)
	}

	// Security: Only allow localhost callbacks
	if redirectURL.Hostname() != "127.0.0.1" {
		return fmt.Errorf("provider %s: redirect URL must use 127.0.0.1, got %s", p.Name(), redirectURL.Hostname())
	}

	// Security: Require HTTP scheme for localhost
	if redirectURL.Scheme != "http" {
		return fmt.Errorf("provider %s: redirect URL must use http scheme", p.Name())
	}

	// Validate that a port is specified (required for loopback redirects)
	if redirectURL.Port() == "" {
		return fmt.Errorf("provider %s: redirect URL must specify a port", p.Name())
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
