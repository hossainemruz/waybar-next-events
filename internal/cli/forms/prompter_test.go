package forms

import (
	"context"
	"strings"
	"testing"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

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

func TestPrompterSelectServiceReturnsErrorForEmptyList(t *testing.T) {
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

func TestPrompterSelectAccountReturnsSelectedID(t *testing.T) {
	p := &Prompter{
		Input:      strings.NewReader("\n"),
		Output:     &strings.Builder{},
		Accessible: true,
	}
	id, err := p.SelectAccount(context.Background(), []calendar.Account{
		{ID: "work", Name: "Work"},
		{ID: "personal", Name: "Personal"},
	}, "Pick an account")
	if err != nil {
		t.Fatalf("SelectAccount() error = %v", err)
	}
	if id != "work" {
		t.Fatalf("id = %q, want work", id)
	}
}

func TestPrompterSelectCalendarsPreservesPreselected(t *testing.T) {
	p := &Prompter{
		Input:      strings.NewReader("\n"),
		Output:     &strings.Builder{},
		Accessible: true,
	}
	calendars := []calendar.Calendar{
		{ID: "a", Name: "Alpha"},
		{ID: "b", Name: "Beta"},
	}
	refs, err := p.SelectCalendars(context.Background(), "Work", calendars, []string{"b"})
	if err != nil {
		t.Fatalf("SelectCalendars() error = %v", err)
	}
	if len(refs) != 1 || refs[0].ID != "b" {
		t.Fatalf("refs = %+v, want [Beta]", refs)
	}
}

func TestPrompterConfirmDeleteAcceptsYes(t *testing.T) {
	p := &Prompter{
		Input:      strings.NewReader("y\n"),
		Output:     &strings.Builder{},
		Accessible: true,
	}
	confirmed, err := p.ConfirmDelete(context.Background(), "Work")
	if err != nil {
		t.Fatalf("ConfirmDelete() error = %v", err)
	}
	if !confirmed {
		t.Fatal("confirmed = false, want true")
	}
}

func TestPrompterConfirmDeleteDefaultsToNo(t *testing.T) {
	p := &Prompter{
		Input:      strings.NewReader("\n"),
		Output:     &strings.Builder{},
		Accessible: true,
	}
	confirmed, err := p.ConfirmDelete(context.Background(), "Work")
	if err != nil {
		t.Fatalf("ConfirmDelete() error = %v", err)
	}
	if confirmed {
		t.Fatal("confirmed = true, want false")
	}
}

func TestPrompterConfirmEmptyCalendarsRunsWithoutError(t *testing.T) {
	p := &Prompter{
		Input:      strings.NewReader("\n"),
		Output:     &strings.Builder{},
		Accessible: true,
	}
	if err := p.ConfirmEmptyCalendars(context.Background(), "Work"); err != nil {
		t.Fatalf("ConfirmEmptyCalendars() error = %v", err)
	}
}

func TestPrompterPromptAccountFieldsReturnsPopulatedData(t *testing.T) {
	p := &Prompter{
		Input:      strings.NewReader("Work\n"),
		Output:     &strings.Builder{},
		Accessible: true,
	}
	fields := []calendar.AccountField{}
	data, err := p.PromptAccountFields(context.Background(), fields, AccountFieldsData{}, func(string) error { return nil })
	if err != nil {
		t.Fatalf("PromptAccountFields() error = %v", err)
	}
	if data.Name != "Work" {
		t.Fatalf("data.Name = %q, want Work", data.Name)
	}
}
