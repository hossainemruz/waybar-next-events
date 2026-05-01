package forms

import (
	"fmt"
	"strings"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

// NewServiceSelectForm builds a form for selecting a calendar service.
func NewServiceSelectForm(services []calendar.Service, selected *string) *huh.Form {
	options := make([]huh.Option[string], len(services))
	for i, svc := range services {
		options[i] = huh.NewOption(svc.DisplayName(), string(svc.Type()))
	}
	if *selected == "" && len(services) > 0 {
		*selected = string(services[0].Type())
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a calendar service").
				Options(options...).
				Value(selected),
		),
	)
}

// NewAccountSelectForm builds a form for selecting an account.
func NewAccountSelectForm(accounts []calendar.Account, title string, selected *string) *huh.Form {
	if len(accounts) == 0 {
		return huh.NewForm()
	}
	if *selected == "" {
		*selected = accounts[0].ID
	}

	options := make([]huh.Option[string], len(accounts))
	for i, account := range accounts {
		label := account.Name
		if strings.TrimSpace(label) == "" {
			label = fmt.Sprintf("Account %d", i+1)
		}
		options[i] = huh.NewOption(label, account.ID)
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Options(options...).
				Value(selected),
		),
	)
}
