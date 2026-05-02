package app

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/hossainemruz/waybar-next-events/internal/auth/providers"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
)

type testService struct {
	serviceType calendar.ServiceType
}

func (s testService) Type() calendar.ServiceType {
	return s.serviceType
}

func (s testService) DisplayName() string {
	return string(s.serviceType)
}

func (s testService) AccountFields() []calendar.AccountField {
	return nil
}

func (s testService) DiscoverCalendars(context.Context, calendar.Account, *http.Client) ([]calendar.Calendar, error) {
	return nil, nil
}

func (s testService) FetchEvents(context.Context, calendar.Account, calendar.EventQuery, *http.Client) ([]calendar.Event, error) {
	return nil, nil
}

func (s testService) Provider(context.Context, calendar.Account, secrets.Store) (providers.Provider, error) {
	return nil, nil
}

func TestRegistryRegisterAndResolve(t *testing.T) {
	registry := NewRegistry()
	service := testService{serviceType: calendar.ServiceTypeGoogle}

	if err := registry.Register(service); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	resolved, err := registry.Service(calendar.ServiceTypeGoogle)
	if err != nil {
		t.Fatalf("Service() error = %v", err)
	}

	if resolved.Type() != service.Type() {
		t.Fatalf("resolved service type = %q, want %q", resolved.Type(), service.Type())
	}
}

func TestRegistryRegisterDuplicateService(t *testing.T) {
	registry := NewRegistry()
	service := testService{serviceType: calendar.ServiceTypeGoogle}

	if err := registry.Register(service); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}

	err := registry.Register(service)
	if !errors.Is(err, ErrDuplicateService) {
		t.Fatalf("second Register() error = %v, want ErrDuplicateService", err)
	}
}

func TestRegistryServiceMissing(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Service(calendar.ServiceTypeGoogle)
	if !errors.Is(err, ErrServiceNotRegistered) {
		t.Fatalf("Service() error = %v, want ErrServiceNotRegistered", err)
	}
}

func TestRegistryRegisterNilService(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(nil)
	if !errors.Is(err, ErrNilService) {
		t.Fatalf("Register(nil) error = %v, want ErrNilService", err)
	}
}

func TestRegistryRegisterEmptyServiceType(t *testing.T) {
	registry := NewRegistry()
	service := testService{serviceType: ""}

	err := registry.Register(service)
	if !errors.Is(err, ErrEmptyServiceType) {
		t.Fatalf("Register() with empty type error = %v, want ErrEmptyServiceType", err)
	}
}

func TestRegistryAllReturnsSortedServices(t *testing.T) {
	registry := NewRegistry()

	googleService := testService{serviceType: calendar.ServiceTypeGoogle}
	appleService := testService{serviceType: calendar.ServiceType("apple")}
	outlookService := testService{serviceType: calendar.ServiceType("outlook")}

	for _, service := range []testService{outlookService, googleService, appleService} {
		if err := registry.Register(service); err != nil {
			t.Fatalf("Register(%q) error = %v", service.Type(), err)
		}
	}

	services := registry.All()
	if len(services) != 3 {
		t.Fatalf("All() returned %d services, want 3", len(services))
	}

	want := []calendar.ServiceType{"apple", calendar.ServiceTypeGoogle, "outlook"}
	for i, service := range services {
		if service.Type() != want[i] {
			t.Fatalf("All()[%d].Type() = %q, want %q", i, service.Type(), want[i])
		}
	}
}
