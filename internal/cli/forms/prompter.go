package forms

import (
	"context"
	"fmt"
	"io"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

// Prompter implements interactive prompts using huh forms.
type Prompter struct {
	Input      io.Reader
	Output     io.Writer
	Accessible bool
}

func (p *Prompter) configure(form *huh.Form) *huh.Form {
	return ConfigureForm(form, p.Input, p.Output, p.Accessible)
}

// SelectService prompts the user to choose a calendar service.
func (p *Prompter) SelectService(ctx context.Context, services []calendar.Service) (calendar.Service, error) {
	if len(services) == 0 {
		return nil, fmt.Errorf("no services available")
	}
	var selected string
	form, err := NewServiceSelectForm(services, &selected)
	if err != nil {
		return nil, err
	}
	form = p.configure(form)
	if err := form.RunWithContext(ctx); err != nil {
		return nil, err
	}
	for _, svc := range services {
		if string(svc.Type()) == selected {
			return svc, nil
		}
	}
	return nil, fmt.Errorf("selected service %q not found", selected)
}

// PromptAccountFields prompts for account name and provider fields.
func (p *Prompter) PromptAccountFields(ctx context.Context, fields []calendar.AccountField, defaults AccountFieldsData, validateName func(string) error) (AccountFieldsData, error) {
	form, output := NewAccountFieldsForm(fields, defaults, validateName)
	form = p.configure(form)
	if err := form.RunWithContext(ctx); err != nil {
		return AccountFieldsData{}, err
	}
	return output(), nil
}

// SelectAccount prompts the user to choose an account.
func (p *Prompter) SelectAccount(ctx context.Context, accounts []calendar.Account, title string) (string, error) {
	var selected string
	form, err := NewAccountSelectForm(accounts, title, &selected)
	if err != nil {
		return "", err
	}
	form = p.configure(form)
	if err := form.RunWithContext(ctx); err != nil {
		return "", err
	}
	return selected, nil
}

// SelectCalendars prompts the user to select calendars.
func (p *Prompter) SelectCalendars(ctx context.Context, accountName string, calendars []calendar.Calendar, preselected []string) ([]calendar.CalendarRef, error) {
	selected := append([]string(nil), preselected...)
	form := p.configure(NewCalendarSelectForm(accountName, calendars, &selected))
	if err := form.RunWithContext(ctx); err != nil {
		return nil, err
	}
	return ToCalendarRefs(calendars, selected), nil
}

// ConfirmEmptyCalendars shows a note when no calendars are found.
func (p *Prompter) ConfirmEmptyCalendars(ctx context.Context, accountName string) error {
	form := p.configure(NewEmptyCalendarsNote(accountName))
	return form.RunWithContext(ctx)
}

// ConfirmDelete prompts for delete confirmation.
func (p *Prompter) ConfirmDelete(ctx context.Context, accountName string) (bool, error) {
	confirmed := false
	form := p.configure(NewDeleteConfirmForm(accountName, &confirmed))
	if err := form.RunWithContext(ctx); err != nil {
		return false, err
	}
	return confirmed, nil
}
