package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hossainemruz/waybar-next-events/internal/auth/providers"
	"github.com/hossainemruz/waybar-next-events/internal/auth/tokenstore"
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

// Service defines the provider-specific operations app workflows require.
type Service interface {
	Type() calendar.ServiceType
	AccountFields() []calendar.AccountField
	Provider(ctx context.Context, account calendar.Account, secretStore secrets.Store) (providers.Provider, error)
	DiscoverCalendars(ctx context.Context, account calendar.Account, client *http.Client) ([]calendar.Calendar, error)
	FetchEvents(ctx context.Context, account calendar.Account, query calendar.EventQuery, client *http.Client) ([]calendar.Event, error)
}

// ServiceResolver resolves services by stable type.
type ServiceResolver interface {
	Service(serviceType calendar.ServiceType) (Service, error)
}

// Registry stores app services by stable type.
type Registry struct {
	services map[calendar.ServiceType]Service
}

// NewRegistry creates an empty app service registry.
func NewRegistry(services ...Service) (*Registry, error) {
	registry := &Registry{services: make(map[calendar.ServiceType]Service, len(services))}
	for _, service := range services {
		if err := registry.Register(service); err != nil {
			return nil, err
		}
	}

	return registry, nil
}

// Register adds a service to the registry.
func (r *Registry) Register(service Service) error {
	if service == nil {
		return fmt.Errorf("service cannot be nil")
	}

	serviceType := service.Type()
	if serviceType == "" {
		return fmt.Errorf("service type cannot be empty")
	}
	if _, exists := r.services[serviceType]; exists {
		return fmt.Errorf("service already registered: %s", serviceType)
	}

	r.services[serviceType] = service
	return nil
}

// Service resolves a service by type.
func (r *Registry) Service(serviceType calendar.ServiceType) (Service, error) {
	service, ok := r.services[serviceType]
	if !ok {
		return nil, fmt.Errorf("service not registered: %s", serviceType)
	}

	return service, nil
}

func tokenKey(account calendar.Account) string {
	return tokenstore.TokenKey(string(account.Service), account.ID)
}
