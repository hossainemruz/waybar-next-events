package calendar

import (
	"testing"
	"time"
)

func TestAccountSetting(t *testing.T) {
	a := Account{}
	if got := a.Setting("key"); got != "" {
		t.Fatalf("Setting() = %q, want empty", got)
	}

	a.SetSetting("key", "value")
	if got := a.Setting("key"); got != "value" {
		t.Fatalf("Setting() = %q, want value", got)
	}
}

func TestAccountCalendarIDs(t *testing.T) {
	a := Account{}
	ids := a.CalendarIDs()
	if len(ids) != 0 {
		t.Fatalf("CalendarIDs() = %v, want empty", ids)
	}

	a.Calendars = []CalendarRef{{ID: "a"}, {ID: "b"}}
	ids = a.CalendarIDs()
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Fatalf("CalendarIDs() = %v, want [a b]", ids)
	}
}

func TestEventIsAllDay(t *testing.T) {
	loc := time.UTC
	allDay := Event{
		Start: time.Date(2026, 1, 15, 0, 0, 0, 0, loc),
		End:   time.Date(2026, 1, 15, 23, 59, 59, EndOfDayNano, loc),
	}
	if !allDay.IsAllDay() {
		t.Fatal("IsAllDay() = false, want true")
	}

	timed := Event{
		Start: time.Date(2026, 1, 15, 10, 0, 0, 0, loc),
		End:   time.Date(2026, 1, 15, 11, 0, 0, 0, loc),
	}
	if timed.IsAllDay() {
		t.Fatal("IsAllDay() = true, want false")
	}

	est := time.FixedZone("EST", -5*3600)
	allDayEST := Event{
		Start: time.Date(2026, 1, 15, 0, 0, 0, 0, est),
		End:   time.Date(2026, 1, 15, 23, 59, 59, EndOfDayNano, est),
	}
	if !allDayEST.IsAllDay() {
		t.Fatal("IsAllDay() = false for EST, want true")
	}
}
