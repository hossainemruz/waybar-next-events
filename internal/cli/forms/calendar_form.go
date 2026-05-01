package forms

import (
	"fmt"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

const (
	minCalendarSelectionHeight = 4
	maxCalendarSelectionHeight = 10
)

// NewCalendarSelectForm builds a calendar multiselect form.
func NewCalendarSelectForm(accountName string, calendars []calendar.Calendar, selected *[]string) *huh.Form {
	options := make([]huh.Option[string], len(calendars))
	for i, cal := range calendars {
		label := cal.Name
		if cal.Primary {
			label += " (Primary)"
		}
		options[i] = huh.NewOption(label, cal.ID)
	}

	height := len(options) + 1
	if height < minCalendarSelectionHeight {
		height = minCalendarSelectionHeight
	}
	if height > maxCalendarSelectionHeight {
		height = maxCalendarSelectionHeight
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(fmt.Sprintf("Select calendars for %q", accountName)).
				Options(options...).
				Value(selected).
				Height(height),
		),
	)
}

// NewEmptyCalendarsNote builds a note form shown when no calendars are discovered.
func NewEmptyCalendarsNote(accountName string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("No calendars found").
				Description(fmt.Sprintf("No calendars were found for account %q. It will be saved with an empty calendars list.", accountName)).
				Next(true).
				NextLabel("Continue"),
		),
	)
}
