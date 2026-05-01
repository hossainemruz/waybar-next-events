package forms

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

func TestAccountSelectFormDefaultsToFirstAccount(t *testing.T) {
	accounts := []calendar.Account{
		{ID: "b", Name: "Beta"},
		{ID: "a", Name: "Alpha"},
	}
	selected := ""
	out := &strings.Builder{}
	form := NewAccountSelectForm(accounts, "Pick an account", &selected).WithAccessible(true).WithInput(strings.NewReader("\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	if selected != "b" {
		t.Fatalf("selected = %q, want b", selected)
	}
}

func TestAccountSelectFormLabelsEmptyNames(t *testing.T) {
	accounts := []calendar.Account{
		{ID: "a", Name: ""},
	}
	selected := ""
	out := &strings.Builder{}
	form := NewAccountSelectForm(accounts, "Pick an account", &selected).WithAccessible(true).WithInput(strings.NewReader("\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	if selected != "a" {
		t.Fatalf("selected = %q, want a", selected)
	}
}

func TestServiceSelectFormDefaultsToFirstService(t *testing.T) {
	services := []calendar.Service{
		&stubService{serviceType: calendar.ServiceTypeGoogle, displayName: "Google"},
		&stubService{serviceType: calendar.ServiceType("outlook"), displayName: "Outlook"},
	}
	selected := ""
	out := &strings.Builder{}
	form := NewServiceSelectForm(services, &selected).WithAccessible(true).WithInput(strings.NewReader("\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	if selected != "google" {
		t.Fatalf("selected = %q, want google", selected)
	}
}

func TestPrompterSelectServiceResolvesSelectedService(t *testing.T) {
	p := &Prompter{
		Input:      strings.NewReader("\n"),
		Output:     &strings.Builder{},
		Accessible: true,
	}
	svc, err := p.SelectService(context.Background(), []calendar.Service{
		&stubService{serviceType: calendar.ServiceTypeGoogle, displayName: "Google"},
	})
	if err != nil {
		t.Fatalf("SelectService() error = %v", err)
	}
	if svc.Type() != calendar.ServiceTypeGoogle {
		t.Fatalf("svc.Type() = %q, want google", svc.Type())
	}
}

func TestPrompterSelectServiceReturnsErrorForUnknownSelection(t *testing.T) {
	p := &Prompter{
		Input:      strings.NewReader("\n"),
		Output:     &strings.Builder{},
		Accessible: true,
	}
	_, err := p.SelectService(context.Background(), []calendar.Service{})
	if err == nil {
		t.Fatal("expected error for empty service list")
	}
}

type stubService struct {
	serviceType calendar.ServiceType
	displayName string
	fields      []calendar.AccountField
}

func (s *stubService) Type() calendar.ServiceType { return s.serviceType }
func (s *stubService) DisplayName() string        { return s.displayName }
func (s *stubService) AccountFields() []calendar.AccountField {
	if s.fields != nil {
		return s.fields
	}
	return []calendar.AccountField{}
}
func (s *stubService) DiscoverCalendars(context.Context, calendar.Account, *http.Client) ([]calendar.Calendar, error) {
	return nil, nil
}
func (s *stubService) FetchEvents(context.Context, calendar.Account, calendar.EventQuery, *http.Client) ([]calendar.Event, error) {
	return nil, nil
}
