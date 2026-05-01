package calendar

import (
	"context"
	"net/http"
)

// ServiceType is a stable identifier for a calendar provider.
type ServiceType string

const (
	ServiceTypeGoogle ServiceType = "google"
)

// Service describes the provider operations the application currently needs.
type Service interface {
	Type() ServiceType
	DisplayName() string
	AccountFields() []AccountField
	DiscoverCalendars(ctx context.Context, account Account, client *http.Client) ([]Calendar, error)
	FetchEvents(ctx context.Context, account Account, query EventQuery, client *http.Client) ([]Event, error)
}
