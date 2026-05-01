package forms

import (
	"strings"
	"testing"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

func TestCalendarSelectFormPreservesPreselected(t *testing.T) {
	calendars := []calendar.Calendar{
		{ID: "a", Name: "Alpha"},
		{ID: "b", Name: "Beta"},
		{ID: "c", Name: "Gamma", Primary: true},
	}
	selected := []string{"b"}
	out := &strings.Builder{}
	form := NewCalendarSelectForm("Work", calendars, &selected).WithAccessible(true).WithInput(strings.NewReader("\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	if len(selected) != 1 || selected[0] != "b" {
		t.Fatalf("selected = %+v, want [b]", selected)
	}
}

func TestCalendarSelectFormAddsPrimaryLabel(t *testing.T) {
	calendars := []calendar.Calendar{
		{ID: "primary", Name: "Primary", Primary: true},
	}
	selected := []string{}
	out := &strings.Builder{}
	form := NewCalendarSelectForm("Work", calendars, &selected).WithAccessible(true).WithInput(strings.NewReader("\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "Primary (Primary)") {
		t.Fatalf("output does not contain primary label: %q", output)
	}
}
