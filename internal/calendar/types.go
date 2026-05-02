package calendar

import "time"

// EndOfDayNano is the nanosecond component for the last nanosecond of a day.
const EndOfDayNano = 999999999

// Account represents a provider account in provider-agnostic terms.
type Account struct {
	ID        string            `json:"id"`
	Service   ServiceType       `json:"service"`
	Name      string            `json:"name"`
	Settings  map[string]string `json:"settings,omitempty"`
	Calendars []CalendarRef     `json:"calendars,omitempty"`
}

// Setting returns the stored setting value for the given key.
func (a Account) Setting(key string) string {
	if a.Settings == nil {
		return ""
	}

	return a.Settings[key]
}

// SetSetting stores a setting value on the account.
func (a *Account) SetSetting(key, value string) {
	if a.Settings == nil {
		a.Settings = make(map[string]string)
	}

	a.Settings[key] = value
}

// CalendarIDs returns the selected calendar IDs for this account.
func (a Account) CalendarIDs() []string {
	if len(a.Calendars) == 0 {
		return []string{}
	}

	ids := make([]string, len(a.Calendars))
	for i, cal := range a.Calendars {
		ids[i] = cal.ID
	}

	return ids
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

// CalendarRef represents a persisted selected calendar reference.
type CalendarRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Event represents a calendar event.
type Event struct {
	Title string
	Start time.Time
	End   time.Time
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
