package types

import (
	"time"
)

// EndOfDayNano is the nanosecond component for the last nanosecond of a day (23:59:59.999999999).
const EndOfDayNano = 999999999

type Event struct {
	Title    string
	Start    time.Time
	End      time.Time
	Calendar string
}

func (e *Event) IsAllDay() bool {
	startOfDay := time.Date(e.Start.Year(), e.Start.Month(), e.Start.Day(), 0, 0, 0, 0, e.Start.Location())
	endOfDay := time.Date(e.End.Year(), e.End.Month(), e.End.Day(), 23, 59, 59, EndOfDayNano, e.End.Location())
	return e.Start.Equal(startOfDay) && e.End.Equal(endOfDay)
}

type EventsGroup struct {
	Day    string
	Events []Event
}
