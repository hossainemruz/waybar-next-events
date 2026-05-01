package forms

import (
	"fmt"

	"charm.land/huh/v2"
)

// NewDeleteConfirmForm builds a delete confirmation form.
func NewDeleteConfirmForm(accountName string, confirmed *bool) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Delete account %q?", accountName)).
				Description("This removes the account from config and clears its stored OAuth token and secrets.").
				Affirmative("Delete").
				Negative("Cancel").
				Value(confirmed),
		),
	)
}
