package types

import (
	"time"
)

type Event struct {
	Title    string
	Start    time.Time
	End      time.Time
	Calendar string
}

type EventsGroup struct {
	Day    string
	Events []Event
}
