package forms

import (
	"context"
	"strings"
	"testing"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

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
