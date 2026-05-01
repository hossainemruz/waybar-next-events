package calendar

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

type testService struct {
	serviceType ServiceType
}

func (s testService) Type() ServiceType {
	return s.serviceType
}

func (s testService) DisplayName() string {
	return string(s.serviceType)
}

func (s testService) AccountFields() []AccountField {
	return nil
}

func (s testService) DiscoverCalendars(context.Context, Account, *http.Client) ([]Calendar, error) {
	return nil, nil
}

func (s testService) FetchEvents(context.Context, Account, EventQuery, *http.Client) ([]Event, error) {
	return nil, nil
}

func TestRegistryRegisterAndResolve(t *testing.T) {
	registry := NewRegistry()
	service := testService{serviceType: ServiceTypeGoogle}

	if err := registry.Register(service); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	resolved, err := registry.Service(ServiceTypeGoogle)
	if err != nil {
		t.Fatalf("Service() error = %v", err)
	}

	if resolved.Type() != service.Type() {
		t.Fatalf("resolved service type = %q, want %q", resolved.Type(), service.Type())
	}
}

func TestRegistryRegisterDuplicateService(t *testing.T) {
	registry := NewRegistry()
	service := testService{serviceType: ServiceTypeGoogle}

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

	_, err := registry.Service(ServiceTypeGoogle)
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

	googleService := testService{serviceType: ServiceTypeGoogle}
	appleService := testService{serviceType: ServiceType("apple")}
	outlookService := testService{serviceType: ServiceType("outlook")}

	for _, service := range []testService{outlookService, googleService, appleService} {
		if err := registry.Register(service); err != nil {
			t.Fatalf("Register(%q) error = %v", service.Type(), err)
		}
	}

	services := registry.All()
	if len(services) != 3 {
		t.Fatalf("All() returned %d services, want 3", len(services))
	}

	want := []ServiceType{"apple", ServiceTypeGoogle, "outlook"}
	for i, service := range services {
		if service.Type() != want[i] {
			t.Fatalf("All()[%d].Type() = %q, want %q", i, service.Type(), want[i])
		}
	}
}
