package app

import (
	"context"
	"net/http"

	"github.com/hossainemruz/waybar-next-events/internal/auth/providers"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"golang.org/x/oauth2"
)

// ConfigLoader defines the config operations app workflows require.
type ConfigLoader interface {
	Load() (*config.Config, error)
	LoadOrEmpty() (*config.Config, error)
	Save(cfg *config.Config) error
	Snapshot() (config.Snapshot, error)
	RestoreSnapshot(snapshot config.Snapshot) error
}

// Authenticator defines the auth operations app workflows require.
type Authenticator interface {
	Authenticate(ctx context.Context, provider providers.Provider) (*oauth2.Token, error)
	ForceAuthenticate(ctx context.Context, provider providers.Provider) (*oauth2.Token, error)
	HTTPClient(ctx context.Context, provider providers.Provider) (*http.Client, error)
}

// Service extends the generic calendar service with secret-aware provider wiring.
type Service interface {
	calendar.Service
	Provider(ctx context.Context, account calendar.Account, secretStore secrets.Store) (providers.Provider, error)
}

// ServiceResolver resolves services by stable type.
type ServiceResolver interface {
	Service(serviceType calendar.ServiceType) (calendar.Service, error)
}

// CalendarSelector handles interactive calendar-selection decisions.
type CalendarSelector interface {
	SelectCalendars(ctx context.Context, account calendar.Account, discovered []calendar.Calendar) ([]calendar.CalendarRef, error)
	ConfirmEmptyCalendars(ctx context.Context, account calendar.Account) error
}
