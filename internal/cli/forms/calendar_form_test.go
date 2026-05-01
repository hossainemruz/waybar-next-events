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

func TestCalendarSelectFormHeightBounds(t *testing.T) {
	calendars := []calendar.Calendar{}
	selected := []string{}
	_ = NewCalendarSelectForm("Work", calendars, &selected)
}

func TestToCalendarRefsPreservesOrder(t *testing.T) {
	calendars := []calendar.Calendar{
		{ID: "a", Name: "Alpha"},
		{ID: "b", Name: "Beta"},
		{ID: "c", Name: "Gamma"},
	}
	refs := ToCalendarRefs(calendars, []string{"b", "a"})
	if len(refs) != 2 {
		t.Fatalf("len(refs) = %d, want 2", len(refs))
	}
	if refs[0].ID != "a" || refs[0].Name != "Alpha" {
		t.Fatalf("refs[0] = (%q, %q), want (Alpha, a)", refs[0].Name, refs[0].ID)
	}
	if refs[1].ID != "b" || refs[1].Name != "Beta" {
		t.Fatalf("refs[1] = (%q, %q), want (Beta, b)", refs[1].Name, refs[1].ID)
	}
}

func TestToCalendarRefsReturnsEmptySlice(t *testing.T) {
	empty := ToCalendarRefs([]calendar.Calendar{{ID: "a", Name: "A"}}, nil)
	if len(empty) != 0 {
		t.Fatalf("len(empty) = %d, want 0", len(empty))
	}
	if empty == nil {
		t.Fatal("ToCalendarRefs(nil) = nil, want empty slice")
	}
}
