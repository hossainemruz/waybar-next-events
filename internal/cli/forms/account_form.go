package forms

import (
	"fmt"
	"strings"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

// AccountFieldsInput holds default values for an account fields form.
type AccountFieldsInput struct {
	Name     string
	Settings map[string]string
	Secrets  map[string]string
}

// AccountFieldsResult holds the output of an account fields form.
type AccountFieldsResult struct {
	Name     string
	Settings map[string]string
	Secrets  map[string]string
}

// NewAccountFieldsForm builds a huh.Form from field metadata.
// Call the returned commit function after the form runs successfully
// to populate result with trimmed values.
func NewAccountFieldsForm(
	fields []calendar.AccountField,
	defaults AccountFieldsInput,
	result *AccountFieldsResult,
	validateName func(string) error,
) (*huh.Form, func()) {
	if result.Settings == nil {
		result.Settings = make(map[string]string)
	}
	if result.Secrets == nil {
		result.Secrets = make(map[string]string)
	}

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

	result.Name = defaults.Name

	groupFields := []huh.Field{
		huh.NewInput().
			Title("Account name").
			Placeholder("Work").
			Value(&result.Name).
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

	commit := func() {
		result.Name = strings.TrimSpace(result.Name)
		for _, field := range fields {
			val := strings.TrimSpace(*fieldValues[field.Key])
			if field.Secret {
				result.Secrets[field.Key] = val
			} else {
				result.Settings[field.Key] = val
			}
		}
	}

	return form, commit
}
