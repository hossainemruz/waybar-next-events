package google

import (
	"testing"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	googlecalendar "google.golang.org/api/calendar/v3"
)

func Test_parseEventTime(t *testing.T) {
	loc := time.Now().Location()

	tests := []struct {
		name      string
		event     googlecalendar.Event
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			name: "both start and end have DateTime (RFC3339)",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{DateTime: "2025-06-15T10:00:00+02:00"},
				End:   &googlecalendar.EventDateTime{DateTime: "2025-06-15T11:00:00+02:00"},
			},
			wantStart: time.Date(2025, 6, 15, 10, 0, 0, 0, time.FixedZone("", 2*3600)),
			wantEnd:   time.Date(2025, 6, 15, 11, 0, 0, 0, time.FixedZone("", 2*3600)),
		},
		{
			name: "both start and end have DateTime in UTC",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{DateTime: "2025-06-15T08:00:00Z"},
				End:   &googlecalendar.EventDateTime{DateTime: "2025-06-15T09:30:00Z"},
			},
			wantStart: time.Date(2025, 6, 15, 8, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 6, 15, 9, 30, 0, 0, time.UTC),
		},
		{
			name: "all-day event (date only, single day)",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{Date: "2025-06-15"},
				End:   &googlecalendar.EventDateTime{Date: "2025-06-16"},
			},
			wantStart: time.Date(2025, 6, 15, 0, 0, 0, 0, loc),
			wantEnd:   time.Date(2025, 6, 15, 23, 59, 59, calendar.EndOfDayNano, loc),
		},
		{
			name: "all-day event spanning multiple days",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{Date: "2025-06-15"},
				End:   &googlecalendar.EventDateTime{Date: "2025-06-18"},
			},
			wantStart: time.Date(2025, 6, 15, 0, 0, 0, 0, loc),
			wantEnd:   time.Date(2025, 6, 17, 23, 59, 59, calendar.EndOfDayNano, loc),
		},
		{
			name: "start has DateTime, end has Date only",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{DateTime: "2025-06-15T14:00:00Z"},
				End:   &googlecalendar.EventDateTime{Date: "2025-06-16"},
			},
			wantStart: time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 6, 15, 23, 59, 59, calendar.EndOfDayNano, loc),
		},
		{
			name: "start has Date only, end has DateTime",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{Date: "2025-06-15"},
				End:   &googlecalendar.EventDateTime{DateTime: "2025-06-15T18:00:00Z"},
			},
			wantStart: time.Date(2025, 6, 15, 0, 0, 0, 0, loc),
			wantEnd:   time.Date(2025, 6, 15, 18, 0, 0, 0, time.UTC),
		},
		{
			name: "invalid start DateTime format",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{DateTime: "not-a-date"},
				End:   &googlecalendar.EventDateTime{DateTime: "2025-06-15T11:00:00Z"},
			},
			wantErr: true,
		},
		{
			name: "invalid end DateTime format",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{DateTime: "2025-06-15T10:00:00Z"},
				End:   &googlecalendar.EventDateTime{DateTime: "not-a-date"},
			},
			wantErr: true,
		},
		{
			name: "invalid start Date format",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{Date: "15/06/2025"},
				End:   &googlecalendar.EventDateTime{Date: "2025-06-16"},
			},
			wantErr: true,
		},
		{
			name: "invalid end Date format",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{Date: "2025-06-15"},
				End:   &googlecalendar.EventDateTime{Date: "16-Jun-2025"},
			},
			wantErr: true,
		},
		{
			name: "empty start DateTime and Date (both zero-value)",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{},
				End:   &googlecalendar.EventDateTime{DateTime: "2025-06-15T11:00:00Z"},
			},
			wantErr: true,
		},
		{
			name: "empty end DateTime and Date (both zero-value)",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{DateTime: "2025-06-15T10:00:00Z"},
				End:   &googlecalendar.EventDateTime{},
			},
			wantErr: true,
		},
		{
			name: "nil Start pointer",
			event: googlecalendar.Event{
				Start: nil,
				End:   &googlecalendar.EventDateTime{DateTime: "2025-06-15T11:00:00Z"},
			},
			wantErr: true,
		},
		{
			name: "nil End pointer",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{DateTime: "2025-06-15T10:00:00Z"},
				End:   nil,
			},
			wantErr: true,
		},
		{
			name: "both Start and End nil",
			event: googlecalendar.Event{
				Start: nil,
				End:   nil,
			},
			wantErr: true,
		},
		{
			name: "DateTime takes precedence over Date when both are set",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{DateTime: "2025-06-15T10:00:00Z", Date: "2025-06-20"},
				End:   &googlecalendar.EventDateTime{DateTime: "2025-06-15T11:00:00Z", Date: "2025-06-20"},
			},
			wantStart: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC),
		},
		{
			name: "negative timezone offset",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{DateTime: "2025-06-15T10:00:00-05:00"},
				End:   &googlecalendar.EventDateTime{DateTime: "2025-06-15T11:00:00-05:00"},
			},
			wantStart: time.Date(2025, 6, 15, 10, 0, 0, 0, time.FixedZone("", -5*3600)),
			wantEnd:   time.Date(2025, 6, 15, 11, 0, 0, 0, time.FixedZone("", -5*3600)),
		},
		{
			name: "DateTime with fractional seconds (RFC3339Nano)",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{DateTime: "2025-06-15T10:00:00.123Z"},
				End:   &googlecalendar.EventDateTime{DateTime: "2025-06-15T11:30:00.456Z"},
			},
			wantStart: time.Date(2025, 6, 15, 10, 0, 0, 123000000, time.UTC),
			wantEnd:   time.Date(2025, 6, 15, 11, 30, 0, 456000000, time.UTC),
		},
		{
			name: "DateTime with fractional seconds and timezone offset",
			event: googlecalendar.Event{
				Start: &googlecalendar.EventDateTime{DateTime: "2025-06-15T10:00:00.999+02:00"},
				End:   &googlecalendar.EventDateTime{DateTime: "2025-06-15T11:00:00.500+02:00"},
			},
			wantStart: time.Date(2025, 6, 15, 10, 0, 0, 999000000, time.FixedZone("", 2*3600)),
			wantEnd:   time.Date(2025, 6, 15, 11, 0, 0, 500000000, time.FixedZone("", 2*3600)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd, err := parseEventTime(tt.event, loc)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (start=%v, end=%v)", gotStart, gotEnd)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !gotStart.Equal(tt.wantStart) {
				t.Errorf("start mismatch:\n  got:  %v\n  want: %v", gotStart, tt.wantStart)
			}
			if !gotEnd.Equal(tt.wantEnd) {
				t.Errorf("end mismatch:\n  got:  %v\n  want: %v", gotEnd, tt.wantEnd)
			}
		})
	}
}

func Test_parseEventTime_nonLocalTimezone(t *testing.T) {
	// Use a fixed non-local timezone to verify date-only parsing does not
	// depend on the machine's local timezone.
	loc := time.FixedZone("Asia/Dhaka", 6*3600)

	gotStart, gotEnd, err := parseEventTime(googlecalendar.Event{
		Start: &googlecalendar.EventDateTime{Date: "2025-06-15"},
		End:   &googlecalendar.EventDateTime{Date: "2025-06-16"},
	}, loc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantStart := time.Date(2025, 6, 15, 0, 0, 0, 0, loc)
	wantEnd := time.Date(2025, 6, 15, 23, 59, 59, calendar.EndOfDayNano, loc)

	if !gotStart.Equal(wantStart) {
		t.Errorf("start mismatch:\n  got:  %v\n  want: %v", gotStart, wantStart)
	}
	if !gotEnd.Equal(wantEnd) {
		t.Errorf("end mismatch:\n  got:  %v\n  want: %v", gotEnd, wantEnd)
	}

	// Multi-day all-day event
	gotStart, gotEnd, err = parseEventTime(googlecalendar.Event{
		Start: &googlecalendar.EventDateTime{Date: "2025-06-15"},
		End:   &googlecalendar.EventDateTime{Date: "2025-06-18"},
	}, loc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantStart = time.Date(2025, 6, 15, 0, 0, 0, 0, loc)
	wantEnd = time.Date(2025, 6, 17, 23, 59, 59, calendar.EndOfDayNano, loc)

	if !gotStart.Equal(wantStart) {
		t.Errorf("multi-day start mismatch:\n  got:  %v\n  want: %v", gotStart, wantStart)
	}
	if !gotEnd.Equal(wantEnd) {
		t.Errorf("multi-day end mismatch:\n  got:  %v\n  want: %v", gotEnd, wantEnd)
	}
}

func Test_convertGoogleCalendarEvents(t *testing.T) {
	loc := time.Now().Location()

	edt := func(dateTime, date string) *googlecalendar.EventDateTime {
		return &googlecalendar.EventDateTime{DateTime: dateTime, Date: date}
	}

	today := time.Date(2025, 6, 15, 0, 0, 0, 0, loc)

	tests := []struct {
		name     string
		gEvents  []*googlecalendar.Event
		dayLimit int
		today    time.Time
		want     []calendar.Event
		wantErr  bool
	}{
		{
			name:     "nil input returns empty slice",
			gEvents:  nil,
			dayLimit: 4,
			today:    today,
			want:     []calendar.Event{},
		},
		{
			name:     "empty slice input returns empty slice",
			gEvents:  []*googlecalendar.Event{},
			dayLimit: 4,
			today:    today,
			want:     []calendar.Event{},
		},
		{
			name: "single timed event",
			gEvents: []*googlecalendar.Event{
				{
					Summary: "Standup",
					Start:   edt("2025-06-15T09:00:00Z", ""),
					End:     edt("2025-06-15T09:30:00Z", ""),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []calendar.Event{
				{Title: "Standup", Start: time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC), End: time.Date(2025, 6, 15, 9, 30, 0, 0, time.UTC)},
			},
		},
		{
			name: "multiple timed events",
			gEvents: []*googlecalendar.Event{
				{
					Summary: "Meeting A",
					Start:   edt("2025-06-15T10:00:00Z", ""),
					End:     edt("2025-06-15T11:00:00Z", ""),
				},
				{
					Summary: "Meeting B",
					Start:   edt("2025-06-15T14:00:00Z", ""),
					End:     edt("2025-06-15T15:00:00Z", ""),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []calendar.Event{
				{Title: "Meeting A", Start: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC), End: time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC)},
				{Title: "Meeting B", Start: time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC), End: time.Date(2025, 6, 15, 15, 0, 0, 0, time.UTC)},
			},
		},
		{
			name: "single all-day event (date only)",
			gEvents: []*googlecalendar.Event{
				{
					Summary: "Holiday",
					Start:   edt("", "2025-06-15"),
					End:     edt("", "2025-06-16"),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []calendar.Event{
				{Title: "Holiday", Start: time.Date(2025, 6, 15, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 15, 23, 59, 59, calendar.EndOfDayNano, loc)},
			},
		},
		{
			name: "event with invalid start time returns error",
			gEvents: []*googlecalendar.Event{
				{
					Summary: "Bad Event",
					Start:   edt("bad-time", ""),
					End:     edt("2025-06-15T11:00:00Z", ""),
				},
			},
			dayLimit: 4,
			today:    today,
			wantErr:  true,
		},
		{
			name: "event with nil Start returns error",
			gEvents: []*googlecalendar.Event{
				{
					Summary: "Nil Start",
					Start:   nil,
					End:     edt("2025-06-15T11:00:00Z", ""),
				},
			},
			dayLimit: 4,
			today:    today,
			wantErr:  true,
		},
		{
			name:     "dayLimit of 0 with no events",
			gEvents:  []*googlecalendar.Event{},
			dayLimit: 0,
			today:    today,
			want:     []calendar.Event{},
		},
		{
			name: "event with empty summary gets default title",
			gEvents: []*googlecalendar.Event{
				{
					Summary: "",
					Start:   edt("2025-06-15T10:00:00Z", ""),
					End:     edt("2025-06-15T11:00:00Z", ""),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []calendar.Event{
				{Title: "<Event title missing>", Start: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC), End: time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC)},
			},
		},
		{
			name: "multi-day event ends before the day limit",
			gEvents: []*googlecalendar.Event{
				{
					Summary: "Conference",
					Start:   edt("", "2025-06-15"),
					End:     edt("", "2025-06-17"),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []calendar.Event{
				{Title: "Conference", Start: time.Date(2025, 6, 15, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 15, 23, 59, 59, calendar.EndOfDayNano, loc)},
				{Title: "Conference", Start: time.Date(2025, 6, 16, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 16, 23, 59, 59, calendar.EndOfDayNano, loc)},
			},
		},
		{
			name: "multi-day event ends after the day limit",
			gEvents: []*googlecalendar.Event{
				{
					Summary: "Long Trip",
					Start:   edt("", "2025-06-15"),
					End:     edt("", "2025-06-20"),
				},
			},
			dayLimit: 3,
			today:    today,
			want: []calendar.Event{
				{Title: "Long Trip", Start: time.Date(2025, 6, 15, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 15, 23, 59, 59, calendar.EndOfDayNano, loc)},
				{Title: "Long Trip", Start: time.Date(2025, 6, 16, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 16, 23, 59, 59, calendar.EndOfDayNano, loc)},
				{Title: "Long Trip", Start: time.Date(2025, 6, 17, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 17, 23, 59, 59, calendar.EndOfDayNano, loc)},
			},
		},
		{
			name: "multi-day event starts after the day limit",
			gEvents: []*googlecalendar.Event{
				{
					Summary: "Future Trip",
					Start:   edt("", "2025-06-18"),
					End:     edt("", "2025-06-20"),
				},
			},
			dayLimit: 2,
			today:    today,
			want:     []calendar.Event{},
		},
		{
			name: "multi-day event started in the past",
			gEvents: []*googlecalendar.Event{
				{
					Summary: "Ongoing Sprint",
					Start:   edt("", "2025-06-13"),
					End:     edt("", "2025-06-17"),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []calendar.Event{
				{Title: "Ongoing Sprint", Start: time.Date(2025, 6, 15, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 15, 23, 59, 59, calendar.EndOfDayNano, loc)},
				{Title: "Ongoing Sprint", Start: time.Date(2025, 6, 16, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 16, 23, 59, 59, calendar.EndOfDayNano, loc)},
			},
		},
		{
			name: "multi-day event starts in the future",
			gEvents: []*googlecalendar.Event{
				{
					Summary: "Upcoming Workshop",
					Start:   edt("", "2025-06-17"),
					End:     edt("", "2025-06-20"),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []calendar.Event{
				{Title: "Upcoming Workshop", Start: time.Date(2025, 6, 17, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 17, 23, 59, 59, calendar.EndOfDayNano, loc)},
				{Title: "Upcoming Workshop", Start: time.Date(2025, 6, 18, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 18, 23, 59, 59, calendar.EndOfDayNano, loc)},
			},
		},
		{
			name: "multi-day timed event",
			gEvents: []*googlecalendar.Event{
				{
					Summary: "Hackathon",
					Start:   edt("2025-06-15T14:00:00Z", ""),
					End:     edt("2025-06-17T10:00:00Z", ""),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []calendar.Event{
				{Title: "Hackathon", Start: time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC), End: time.Date(2025, 6, 15, 23, 59, 59, calendar.EndOfDayNano, loc)},
				{Title: "Hackathon", Start: time.Date(2025, 6, 16, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 16, 23, 59, 59, calendar.EndOfDayNano, loc)},
				{Title: "Hackathon", Start: time.Date(2025, 6, 17, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 17, 10, 0, 0, 0, time.UTC)},
			},
		},
		{
			name: "timed event with same start and end time",
			gEvents: []*googlecalendar.Event{
				{
					Summary: "Reminder",
					Start:   edt("2025-06-15T12:00:00Z", ""),
					End:     edt("2025-06-15T12:00:00Z", ""),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []calendar.Event{
				{Title: "Reminder", Start: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC), End: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)},
			},
		},
		{
			name: "date-only event with same start and end date",
			gEvents: []*googlecalendar.Event{
				{
					Summary: "Same Day",
					Start:   edt("", "2025-06-15"),
					End:     edt("", "2025-06-15"),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []calendar.Event{
				{Title: "Same Day", Start: time.Date(2025, 6, 15, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 15, 23, 59, 59, calendar.EndOfDayNano, loc)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertGoogleEvents(tt.gEvents, tt.dayLimit, tt.today)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("length mismatch: got %d events, want %d events\n  got:  %+v\n  want: %+v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i].Title != tt.want[i].Title {
					t.Errorf("event[%d] title mismatch: got %q, want %q", i, got[i].Title, tt.want[i].Title)
				}
				if !got[i].Start.Equal(tt.want[i].Start) {
					t.Errorf("event[%d] start mismatch:\n  got:  %v\n  want: %v", i, got[i].Start, tt.want[i].Start)
				}
				if !got[i].End.Equal(tt.want[i].End) {
					t.Errorf("event[%d] end mismatch:\n  got:  %v\n  want: %v", i, got[i].End, tt.want[i].End)
				}
			}
		})
	}
}

func Test_convertGoogleCalendarEvents_nonLocalTimezone(t *testing.T) {
	loc := time.FixedZone("Asia/Dhaka", 6*3600)
	today := time.Date(2025, 6, 15, 0, 0, 0, 0, loc)

	edt := func(dateTime, date string) *googlecalendar.EventDateTime {
		return &googlecalendar.EventDateTime{DateTime: dateTime, Date: date}
	}

	gEvents := []*googlecalendar.Event{
		{
			Summary: "Holiday",
			Start:   edt("", "2025-06-15"),
			End:     edt("", "2025-06-16"),
		},
		{
			Summary: "Conference",
			Start:   edt("", "2025-06-15"),
			End:     edt("", "2025-06-17"),
		},
	}

	got, err := convertGoogleEvents(gEvents, 4, today)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []calendar.Event{
		{Title: "Holiday", Start: time.Date(2025, 6, 15, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 15, 23, 59, 59, calendar.EndOfDayNano, loc)},
		{Title: "Conference", Start: time.Date(2025, 6, 15, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 15, 23, 59, 59, calendar.EndOfDayNano, loc)},
		{Title: "Conference", Start: time.Date(2025, 6, 16, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 16, 23, 59, 59, calendar.EndOfDayNano, loc)},
	}

	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %d events, want %d events\n  got:  %+v\n  want: %+v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i].Title != want[i].Title {
			t.Errorf("event[%d] title mismatch: got %q, want %q", i, got[i].Title, want[i].Title)
		}
		if !got[i].Start.Equal(want[i].Start) {
			t.Errorf("event[%d] start mismatch:\n  got:  %v\n  want: %v", i, got[i].Start, want[i].Start)
		}
		if !got[i].End.Equal(want[i].End) {
			t.Errorf("event[%d] end mismatch:\n  got:  %v\n  want: %v", i, got[i].End, want[i].End)
		}
	}
}

func Test_parseRFC3339(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "RFC3339 without fractional seconds",
			input: "2025-06-15T10:00:00Z",
			want:  time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			name:  "RFC3339 with timezone offset",
			input: "2025-06-15T10:00:00+02:00",
			want:  time.Date(2025, 6, 15, 10, 0, 0, 0, time.FixedZone("", 2*3600)),
		},
		{
			name:  "RFC3339Nano with millisecond precision",
			input: "2025-06-15T10:00:00.123Z",
			want:  time.Date(2025, 6, 15, 10, 0, 0, 123000000, time.UTC),
		},
		{
			name:  "RFC3339Nano with microsecond precision",
			input: "2025-06-15T10:00:00.123456Z",
			want:  time.Date(2025, 6, 15, 10, 0, 0, 123456000, time.UTC),
		},
		{
			name:  "RFC3339Nano with nanosecond precision",
			input: "2025-06-15T10:00:00.123456789Z",
			want:  time.Date(2025, 6, 15, 10, 0, 0, 123456789, time.UTC),
		},
		{
			name:  "RFC3339Nano with timezone offset",
			input: "2025-06-15T10:00:00.500+05:30",
			want:  time.Date(2025, 6, 15, 10, 0, 0, 500000000, time.FixedZone("", 5*3600+30*60)),
		},
		{
			name:    "invalid timestamp",
			input:   "not-a-timestamp",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRFC3339(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got time %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("mismatch:\n  got:  %v\n  want: %v", got, tt.want)
			}
		})
	}
}
