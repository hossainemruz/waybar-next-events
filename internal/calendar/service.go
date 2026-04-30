package calendar

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

// ServiceType is a stable identifier for a calendar provider.
type ServiceType string

const (
	ServiceTypeGoogle ServiceType = "google"
)

// AuthProvider describes the OAuth configuration the auth layer currently needs.
type AuthProvider interface {
	Name() string
	ClientID() string
	ClientSecret() string
	AuthURL() string
	TokenURL() string
	RedirectURL() string
	Scopes() []string
	AuthCodeOptions() []oauth2.AuthCodeOption
	ExchangeOptions() []oauth2.AuthCodeOption
}

// Service describes the provider operations the application currently needs.
type Service interface {
	Type() ServiceType
	DisplayName() string
	AccountFields() []AccountField
	AuthProvider(account Account) (AuthProvider, error)
	DiscoverCalendars(ctx context.Context, account Account, client *http.Client) ([]Calendar, error)
	FetchEvents(ctx context.Context, account Account, query EventQuery, client *http.Client) ([]Event, error)
}
