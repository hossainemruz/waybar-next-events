package app

import (
	"errors"
	"fmt"
	"sort"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

var (
	ErrDuplicateService     = errors.New("service already registered")
	ErrServiceNotRegistered = errors.New("service not registered")
	ErrNilService           = errors.New("service cannot be nil")
	ErrEmptyServiceType     = errors.New("service type cannot be empty")
)

// Registry stores services by their stable type identifier.
type Registry struct {
	services map[calendar.ServiceType]Service
}

// NewRegistry creates an empty service registry.
func NewRegistry() *Registry {
	return &Registry{services: make(map[calendar.ServiceType]Service)}
}

// Register adds a service to the registry.
func (r *Registry) Register(service Service) error {
	if service == nil {
		return ErrNilService
	}

	serviceType := service.Type()
	if serviceType == "" {
		return ErrEmptyServiceType
	}

	if _, exists := r.services[serviceType]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateService, serviceType)
	}

	r.services[serviceType] = service
	return nil
}

// Service resolves a service by type.
func (r *Registry) Service(serviceType calendar.ServiceType) (Service, error) {
	service, ok := r.services[serviceType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrServiceNotRegistered, serviceType)
	}

	return service, nil
}

// All returns all registered services sorted by type.
func (r *Registry) All() []Service {
	services := make([]Service, 0, len(r.services))
	for _, service := range r.services {
		services = append(services, service)
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Type() < services[j].Type()
	})

	return services
}
