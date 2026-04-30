package calendar

import "time"

// EndOfDayNano is the nanosecond component for the last nanosecond of a day.
const EndOfDayNano = 999999999

// Account represents a provider account in provider-agnostic terms.
type Account struct {
	ID        string            `json:"id"`
	Type      ServiceType       `json:"type"`
	Name      string            `json:"name"`
	Settings  map[string]string `json:"settings,omitempty"`
	Calendars []Calendar        `json:"calendars,omitempty"`
}

// AccountField describes one user-provided account field.
type AccountField struct {
	Key         string
	Label       string
	Description string
	Required    bool
	Secret      bool
	Validate    func(string) error
}

// Calendar represents a calendar that can be selected or queried.
type Calendar struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Primary bool   `json:"primary"`
}

// Event represents a calendar event.
type Event struct {
	Title    string
	Start    time.Time
	End      time.Time
	Calendar string
}

// IsAllDay reports whether the event spans exactly one local calendar day.
func (e Event) IsAllDay() bool {
	startOfDay := time.Date(e.Start.Year(), e.Start.Month(), e.Start.Day(), 0, 0, 0, 0, e.Start.Location())
	endOfDay := time.Date(e.End.Year(), e.End.Month(), e.End.Day(), 23, 59, 59, EndOfDayNano, e.End.Location())
	return e.Start.Equal(startOfDay) && e.End.Equal(endOfDay)
}

// EventsGroup groups events under a display label such as Today or Tomorrow.
type EventsGroup struct {
	Day    string
	Events []Event
}

// EventQuery describes the current event-fetch workflow.
type EventQuery struct {
	Now      time.Time
	DayLimit int
}
