package forms

import (
	"fmt"
	"strings"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

// AccountFieldsData holds account name, settings, and secrets for form input/output.
type AccountFieldsData struct {
	Name     string
	Settings map[string]string
	Secrets  map[string]string
}

// NewAccountFieldsForm builds a huh.Form from field metadata and returns the form
// plus a function that outputs the populated data after the form runs.
//
// The form uses intermediate *string bindings because Go does not allow taking
// the address of a map element, which huh.Input.Value requires.
func NewAccountFieldsForm(
	fields []calendar.AccountField,
	defaults AccountFieldsData,
	validateName func(string) error,
) (*huh.Form, func() AccountFieldsData) {
	// Bind field values to stable pointers so the form can update them.
	fieldValues := make(map[string]*string, len(fields))
	for _, field := range fields {
		s := ""
		if defaults.Settings != nil {
			if v, ok := defaults.Settings[field.Key]; ok {
				s = v
			}
		}
		if defaults.Secrets != nil {
			if v, ok := defaults.Secrets[field.Key]; ok {
				s = v
			}
		}
		fieldValues[field.Key] = &s
	}

	name := defaults.Name

	groupFields := []huh.Field{
		huh.NewInput().
			Title("Account name").
			Placeholder("Work").
			Value(&name).
			Validate(func(v string) error {
				return validateName(strings.TrimSpace(v))
			}),
	}

	for _, field := range fields {
		field := field // capture for closures
		ptr := fieldValues[field.Key]

		input := huh.NewInput().
			Title(field.Label).
			Value(ptr).
			Validate(func(v string) error {
				trimmed := strings.TrimSpace(v)
				if field.Required && trimmed == "" {
					return fmt.Errorf("%s is required", field.Label)
				}
				if field.Validate != nil {
					return field.Validate(trimmed)
				}
				return nil
			})

		if field.Secret {
			input = input.EchoMode(huh.EchoModePassword)
		}
		if field.Description != "" {
			input = input.Description(field.Description)
		}

		groupFields = append(groupFields, input)
	}

	form := huh.NewForm(huh.NewGroup(groupFields...))

	output := func() AccountFieldsData {
		data := AccountFieldsData{
			Name:     strings.TrimSpace(name),
			Settings: make(map[string]string),
			Secrets:  make(map[string]string),
		}
		for _, field := range fields {
			val := strings.TrimSpace(*fieldValues[field.Key])
			if field.Secret {
				data.Secrets[field.Key] = val
			} else {
				data.Settings[field.Key] = val
			}
		}
		return data
	}

	return form, output
}
