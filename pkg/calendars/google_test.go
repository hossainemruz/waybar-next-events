package calendars

import (
	"testing"
	"time"

	"github.com/emruz-hossain/waybar-next-events/pkg/types"
	"google.golang.org/api/calendar/v3"
)

func Test_parseEventTime(t *testing.T) {
	loc := time.Now().Location()

	tests := []struct {
		name      string
		event     calendar.Event
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			name: "both start and end have DateTime (RFC3339)",
			event: calendar.Event{
				Start: &calendar.EventDateTime{DateTime: "2025-06-15T10:00:00+02:00"},
				End:   &calendar.EventDateTime{DateTime: "2025-06-15T11:00:00+02:00"},
			},
			wantStart: time.Date(2025, 6, 15, 10, 0, 0, 0, time.FixedZone("", 2*3600)),
			wantEnd:   time.Date(2025, 6, 15, 11, 0, 0, 0, time.FixedZone("", 2*3600)),
		},
		{
			name: "both start and end have DateTime in UTC",
			event: calendar.Event{
				Start: &calendar.EventDateTime{DateTime: "2025-06-15T08:00:00Z"},
				End:   &calendar.EventDateTime{DateTime: "2025-06-15T09:30:00Z"},
			},
			wantStart: time.Date(2025, 6, 15, 8, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 6, 15, 9, 30, 0, 0, time.UTC),
		},
		{
			name: "all-day event (date only, single day)",
			event: calendar.Event{
				Start: &calendar.EventDateTime{Date: "2025-06-15"},
				End:   &calendar.EventDateTime{Date: "2025-06-16"},
			},
			// Start = start of June 15, End = end of June 15 (previous day of End.Date)
			wantStart: time.Date(2025, 6, 15, 0, 0, 0, 0, loc),
			wantEnd:   time.Date(2025, 6, 15, 23, 59, 59, types.EndOfDayNano, loc),
		},
		{
			name: "all-day event spanning multiple days",
			event: calendar.Event{
				Start: &calendar.EventDateTime{Date: "2025-06-15"},
				End:   &calendar.EventDateTime{Date: "2025-06-18"},
			},
			// Start = start of June 15, End = end of June 17
			wantStart: time.Date(2025, 6, 15, 0, 0, 0, 0, loc),
			wantEnd:   time.Date(2025, 6, 17, 23, 59, 59, types.EndOfDayNano, loc),
		},
		{
			name: "start has DateTime, end has Date only",
			event: calendar.Event{
				Start: &calendar.EventDateTime{DateTime: "2025-06-15T14:00:00Z"},
				End:   &calendar.EventDateTime{Date: "2025-06-16"},
			},
			wantStart: time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 6, 15, 23, 59, 59, types.EndOfDayNano, loc),
		},
		{
			name: "start has Date only, end has DateTime",
			event: calendar.Event{
				Start: &calendar.EventDateTime{Date: "2025-06-15"},
				End:   &calendar.EventDateTime{DateTime: "2025-06-15T18:00:00Z"},
			},
			wantStart: time.Date(2025, 6, 15, 0, 0, 0, 0, loc),
			wantEnd:   time.Date(2025, 6, 15, 18, 0, 0, 0, time.UTC),
		},
		{
			name: "invalid start DateTime format",
			event: calendar.Event{
				Start: &calendar.EventDateTime{DateTime: "not-a-date"},
				End:   &calendar.EventDateTime{DateTime: "2025-06-15T11:00:00Z"},
			},
			wantErr: true,
		},
		{
			name: "invalid end DateTime format",
			event: calendar.Event{
				Start: &calendar.EventDateTime{DateTime: "2025-06-15T10:00:00Z"},
				End:   &calendar.EventDateTime{DateTime: "not-a-date"},
			},
			wantErr: true,
		},
		{
			name: "invalid start Date format",
			event: calendar.Event{
				Start: &calendar.EventDateTime{Date: "15/06/2025"},
				End:   &calendar.EventDateTime{Date: "2025-06-16"},
			},
			wantErr: true,
		},
		{
			name: "invalid end Date format",
			event: calendar.Event{
				Start: &calendar.EventDateTime{Date: "2025-06-15"},
				End:   &calendar.EventDateTime{Date: "16-Jun-2025"},
			},
			wantErr: true,
		},
		{
			name: "empty start DateTime and Date (both zero-value)",
			event: calendar.Event{
				Start: &calendar.EventDateTime{},
				End:   &calendar.EventDateTime{DateTime: "2025-06-15T11:00:00Z"},
			},
			wantErr: true,
		},
		{
			name: "empty end DateTime and Date (both zero-value)",
			event: calendar.Event{
				Start: &calendar.EventDateTime{DateTime: "2025-06-15T10:00:00Z"},
				End:   &calendar.EventDateTime{},
			},
			wantErr: true,
		},
		{
			name: "nil Start pointer",
			event: calendar.Event{
				Start: nil,
				End:   &calendar.EventDateTime{DateTime: "2025-06-15T11:00:00Z"},
			},
			wantErr: true,
		},
		{
			name: "nil End pointer",
			event: calendar.Event{
				Start: &calendar.EventDateTime{DateTime: "2025-06-15T10:00:00Z"},
				End:   nil,
			},
			wantErr: true,
		},
		{
			name: "both Start and End nil",
			event: calendar.Event{
				Start: nil,
				End:   nil,
			},
			wantErr: true,
		},
		{
			name: "DateTime takes precedence over Date when both are set",
			event: calendar.Event{
				Start: &calendar.EventDateTime{DateTime: "2025-06-15T10:00:00Z", Date: "2025-06-20"},
				End:   &calendar.EventDateTime{DateTime: "2025-06-15T11:00:00Z", Date: "2025-06-20"},
			},
			// DateTime should be used, not Date
			wantStart: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC),
		},
		{
			name: "negative timezone offset",
			event: calendar.Event{
				Start: &calendar.EventDateTime{DateTime: "2025-06-15T10:00:00-05:00"},
				End:   &calendar.EventDateTime{DateTime: "2025-06-15T11:00:00-05:00"},
			},
			wantStart: time.Date(2025, 6, 15, 10, 0, 0, 0, time.FixedZone("", -5*3600)),
			wantEnd:   time.Date(2025, 6, 15, 11, 0, 0, 0, time.FixedZone("", -5*3600)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd, err := parseEventTime(tt.event)
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

func Test_convertGoogleCalendarEvents(t *testing.T) {
	loc := time.Now().Location()

	// Helper to create a pointer to EventDateTime
	edt := func(dateTime, date string) *calendar.EventDateTime {
		return &calendar.EventDateTime{DateTime: dateTime, Date: date}
	}

	// Fixed "today" for deterministic tests: 2025-06-15
	today := time.Date(2025, 6, 15, 0, 0, 0, 0, loc)

	tests := []struct {
		name     string
		gEvents  []*calendar.Event
		dayLimit int
		today    time.Time
		want     []types.Event
		wantErr  bool
	}{
		{
			name:     "nil input returns empty slice",
			gEvents:  nil,
			dayLimit: 4,
			today:    today,
			want:     []types.Event{},
		},
		{
			name:     "empty slice input returns empty slice",
			gEvents:  []*calendar.Event{},
			dayLimit: 4,
			today:    today,
			want:     []types.Event{},
		},
		{
			name: "single timed event",
			gEvents: []*calendar.Event{
				{
					Summary: "Standup",
					Start:   edt("2025-06-15T09:00:00Z", ""),
					End:     edt("2025-06-15T09:30:00Z", ""),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []types.Event{
				{Title: "Standup", Start: time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC), End: time.Date(2025, 6, 15, 9, 30, 0, 0, time.UTC)},
			},
		},
		{
			name: "multiple timed events",
			gEvents: []*calendar.Event{
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
			want: []types.Event{
				{Title: "Meeting A", Start: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC), End: time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC)},
				{Title: "Meeting B", Start: time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC), End: time.Date(2025, 6, 15, 15, 0, 0, 0, time.UTC)},
			},
		},
		{
			name: "single all-day event (date only)",
			gEvents: []*calendar.Event{
				{
					Summary: "Holiday",
					Start:   edt("", "2025-06-15"),
					End:     edt("", "2025-06-16"),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []types.Event{
				{Title: "Holiday", Start: time.Date(2025, 6, 15, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 15, 23, 59, 59, types.EndOfDayNano, loc)},
			},
		},
		{
			name: "event with invalid start time returns error",
			gEvents: []*calendar.Event{
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
			gEvents: []*calendar.Event{
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
			gEvents:  []*calendar.Event{},
			dayLimit: 0,
			today:    today,
			want:     []types.Event{},
		},
		{
			name: "event with empty summary gets default title",
			gEvents: []*calendar.Event{
				{
					Summary: "",
					Start:   edt("2025-06-15T10:00:00Z", ""),
					End:     edt("2025-06-15T11:00:00Z", ""),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []types.Event{
				{Title: "<Event title missing>", Start: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC), End: time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC)},
			},
		},
		// dayLimit tests with multi-day events
		{
			// today=Jun 15, dayLimit=4 covers Jun 15 - Jun 18
			// Event: Jun 15 - Jun 17 (ends before the day limit)
			name: "multi-day event ends before the day limit",
			gEvents: []*calendar.Event{
				{
					Summary: "Conference",
					Start:   edt("", "2025-06-15"),
					End:     edt("", "2025-06-17"),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []types.Event{
				{Title: "Conference", Start: time.Date(2025, 6, 15, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 15, 23, 59, 59, types.EndOfDayNano, loc)},
				{Title: "Conference", Start: time.Date(2025, 6, 16, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 16, 23, 59, 59, types.EndOfDayNano, loc)},
			},
		},
		{
			// today=Jun 15, dayLimit=3 covers Jun 15 - Jun 17
			// Event: Jun 15 - Jun 20 (ends after the day limit)
			name: "multi-day event ends after the day limit",
			gEvents: []*calendar.Event{
				{
					Summary: "Long Trip",
					Start:   edt("", "2025-06-15"),
					End:     edt("", "2025-06-20"),
				},
			},
			dayLimit: 3,
			today:    today,
			want: []types.Event{
				{Title: "Long Trip", Start: time.Date(2025, 6, 15, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 15, 23, 59, 59, types.EndOfDayNano, loc)},
				{Title: "Long Trip", Start: time.Date(2025, 6, 16, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 16, 23, 59, 59, types.EndOfDayNano, loc)},
				{Title: "Long Trip", Start: time.Date(2025, 6, 17, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 17, 23, 59, 59, types.EndOfDayNano, loc)},
			},
		},
		{
			// today=Jun 15, dayLimit=2 covers Jun 15 - Jun 16
			// Event: Jun 18 - Jun 20 (starts after the day limit window)
			name: "multi-day event starts after the day limit",
			gEvents: []*calendar.Event{
				{
					Summary: "Future Trip",
					Start:   edt("", "2025-06-18"),
					End:     edt("", "2025-06-20"),
				},
			},
			dayLimit: 2,
			today:    today,
			want:     []types.Event{},
		},
		// Multi-day event that started in the past (before today)
		{
			// today=Jun 15, dayLimit=4 covers Jun 15 - Jun 18
			// Event: Jun 13 - Jun 17 (started 2 days ago, still ongoing)
			name: "multi-day event started in the past",
			gEvents: []*calendar.Event{
				{
					Summary: "Ongoing Sprint",
					Start:   edt("", "2025-06-13"),
					End:     edt("", "2025-06-17"),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []types.Event{
				{Title: "Ongoing Sprint", Start: time.Date(2025, 6, 15, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 15, 23, 59, 59, types.EndOfDayNano, loc)},
				{Title: "Ongoing Sprint", Start: time.Date(2025, 6, 16, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 16, 23, 59, 59, types.EndOfDayNano, loc)},
			},
		},
		// Multi-day event that starts in the future (within day limit)
		{
			// today=Jun 15, dayLimit=4 covers Jun 15 - Jun 18
			// Event: Jun 17 - Jun 20 (starts in 2 days)
			name: "multi-day event starts in the future",
			gEvents: []*calendar.Event{
				{
					Summary: "Upcoming Workshop",
					Start:   edt("", "2025-06-17"),
					End:     edt("", "2025-06-20"),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []types.Event{
				{Title: "Upcoming Workshop", Start: time.Date(2025, 6, 17, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 17, 23, 59, 59, types.EndOfDayNano, loc)},
				{Title: "Upcoming Workshop", Start: time.Date(2025, 6, 18, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 18, 23, 59, 59, types.EndOfDayNano, loc)},
			},
		},
		// Multi-day timed event (DateTime, not date-only)
		{
			// today=Jun 15, dayLimit=4 covers Jun 15 - Jun 18
			// Event: Jun 15 14:00 - Jun 17 10:00 (timed, spans 2+ days)
			name: "multi-day timed event",
			gEvents: []*calendar.Event{
				{
					Summary: "Hackathon",
					Start:   edt("2025-06-15T14:00:00Z", ""),
					End:     edt("2025-06-17T10:00:00Z", ""),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []types.Event{
				{Title: "Hackathon", Start: time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC), End: time.Date(2025, 6, 15, 23, 59, 59, types.EndOfDayNano, loc)},
				{Title: "Hackathon", Start: time.Date(2025, 6, 16, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 16, 23, 59, 59, types.EndOfDayNano, loc)},
				{Title: "Hackathon", Start: time.Date(2025, 6, 17, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 17, 10, 0, 0, 0, time.UTC)},
			},
		},
		// Timed event with same start and end time
		{
			name: "timed event with same start and end time",
			gEvents: []*calendar.Event{
				{
					Summary: "Reminder",
					Start:   edt("2025-06-15T12:00:00Z", ""),
					End:     edt("2025-06-15T12:00:00Z", ""),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []types.Event{
				{Title: "Reminder", Start: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC), End: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)},
			},
		},
		// Date-only event with same start and end date
		{
			name: "date-only event with same start and end date",
			gEvents: []*calendar.Event{
				{
					Summary: "Same Day",
					Start:   edt("", "2025-06-15"),
					End:     edt("", "2025-06-15"),
				},
			},
			dayLimit: 4,
			today:    today,
			want: []types.Event{
				// When start and end dates are the same, the event is a full day on that date.
				{Title: "Same Day", Start: time.Date(2025, 6, 15, 0, 0, 0, 0, loc), End: time.Date(2025, 6, 15, 23, 59, 59, types.EndOfDayNano, loc)},
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
