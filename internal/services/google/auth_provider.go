package google

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hossainemruz/waybar-next-events/internal/auth/providers"
	"github.com/hossainemruz/waybar-next-events/internal/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"golang.org/x/oauth2"
	googlecalendar "google.golang.org/api/calendar/v3"
)

const (
	// googleAuthURL is Google's OAuth2 authorization endpoint.
	googleAuthURL = "https://accounts.google.com/o/oauth2/auth"
	// googleTokenURL is Google's OAuth2 token endpoint.
	googleTokenURL = "https://oauth2.googleapis.com/token"
)

// Provider builds a Google OAuth provider for the given account using secrets
// loaded from the secret store.
func (s *Service) Provider(ctx context.Context, account calendar.Account, secretStore secrets.Store) (providers.Provider, error) {
	if strings.TrimSpace(account.ID) == "" {
		return nil, fmt.Errorf("account ID cannot be empty")
	}

	clientID := strings.TrimSpace(account.Setting(clientIDKey))
	if clientID == "" {
		return nil, fmt.Errorf("missing required setting %s", clientIDKey)
	}

	clientSecret, err := secretStore.Get(ctx, account.ID, clientSecretKey)
	if err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			return nil, fmt.Errorf("missing stored secret %s", clientSecretKey)
		}
		return nil, fmt.Errorf("load stored secret %s: %w", clientSecretKey, err)
	}

	return newOAuthProvider(
		account.ID,
		clientID,
		clientSecret,
		[]string{googlecalendar.CalendarReadonlyScope},
	), nil
}

// oauthProvider implements the auth.Provider interface for Google OAuth2.
// It supports Google Calendar and other Google APIs.
type oauthProvider struct {
	accountID    string
	clientID     string
	clientSecret string
	scopes       []string
}

// newOAuthProvider creates a new Google OAuth2 provider.
// clientID is required. clientSecret may be empty for public clients.
// If scopes is empty, default calendar read-only scope is used.
func newOAuthProvider(accountID, clientID, clientSecret string, scopes []string) *oauthProvider {
	if len(scopes) == 0 {
		scopes = []string{googlecalendar.CalendarReadonlyScope}
	}
	return &oauthProvider{
		accountID:    accountID,
		clientID:     clientID,
		clientSecret: clientSecret,
		scopes:       scopes,
	}
}

// Ensure oauthProvider implements the auth.Provider interface.
var _ providers.Provider = (*oauthProvider)(nil)

// Name returns a unique identifier for the provider.
// Tokens are keyed by stable service/account identity so account renames and
// shared OAuth apps cannot collide.
func (g *oauthProvider) Name() string {
	return tokenstore.TokenKey(string(calendar.ServiceTypeGoogle), g.accountID)
}

// ClientID returns the OAuth2 client ID.
func (g *oauthProvider) ClientID() string {
	return g.clientID
}

// ClientSecret returns the OAuth2 client secret.
func (g *oauthProvider) ClientSecret() string {
	return g.clientSecret
}

// AuthURL returns the authorization endpoint URL.
func (g *oauthProvider) AuthURL() string {
	return googleAuthURL
}

// TokenURL returns the token endpoint URL.
func (g *oauthProvider) TokenURL() string {
	return googleTokenURL
}

// RedirectURL returns the callback URL.
func (g *oauthProvider) RedirectURL() string {
	return config.DefaultCallbackURL
}

// Scopes returns the OAuth2 scopes.
func (g *oauthProvider) Scopes() []string {
	return g.scopes
}

// AuthCodeOptions returns additional authorization URL parameters.
// For Google, this includes access_type=offline to get a refresh token
// and prompt=consent to ensure the refresh token is issued.
func (g *oauthProvider) AuthCodeOptions() []oauth2.AuthCodeOption {
	return []oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
	}
}

// ExchangeOptions returns additional token exchange parameters.
// Google does not require additional exchange options.
func (g *oauthProvider) ExchangeOptions() []oauth2.AuthCodeOption {
	return nil
}
