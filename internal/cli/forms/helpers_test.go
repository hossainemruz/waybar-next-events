package forms

import (
	"context"
	"errors"
	"testing"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

func TestIsUserAbort(t *testing.T) {
	if !IsUserAbort(huh.ErrUserAborted) {
		t.Fatal("IsUserAbort(ErrUserAborted) = false, want true")
	}
	if !IsUserAbort(context.Canceled) {
		t.Fatal("IsUserAbort(context.Canceled) = false, want true")
	}
	if IsUserAbort(errors.New("some error")) {
		t.Fatal("IsUserAbort(some error) = true, want false")
	}
	if IsUserAbort(nil) {
		t.Fatal("IsUserAbort(nil) = true, want false")
	}
}

func TestToCalendarRefs(t *testing.T) {
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

	empty := ToCalendarRefs(calendars, nil)
	if len(empty) != 0 {
		t.Fatalf("len(empty) = %d, want 0", len(empty))
	}
	if empty == nil {
		t.Fatal("ToCalendarRefs(nil) = nil, want empty slice")
	}
}
