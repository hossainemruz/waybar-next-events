package forms

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

func TestNewAccountFieldsFormReturnsNonNil(t *testing.T) {
	fields := []calendar.AccountField{
		{Key: "client_id", Label: "OAuth Client ID", Required: true},
	}
	form, output := NewAccountFieldsForm(fields, AccountFieldsData{}, func(string) error { return nil })
	if form == nil {
		t.Fatal("expected non-nil form")
	}
	if output == nil {
		t.Fatal("expected non-nil output function")
	}
}

func TestAccountFieldsFormValidatesName(t *testing.T) {
	fields := []calendar.AccountField{}
	validateName := func(name string) error {
		if name == "" {
			return fmt.Errorf("name is required")
		}
		if name == "Work" {
			return fmt.Errorf("name %q already exists", name)
		}
		return nil
	}

	out := &strings.Builder{}
	form, output := NewAccountFieldsForm(fields, AccountFieldsData{}, validateName)
	form = form.WithAccessible(true).WithInput(strings.NewReader("Work\nPersonal\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	result := output()
	if result.Name != "Personal" {
		t.Fatalf("result.Name = %q, want Personal", result.Name)
	}
}

func TestAccountFieldsFormRejectsEmptyName(t *testing.T) {
	fields := []calendar.AccountField{}
	validateName := func(name string) error {
		if name == "" {
			return fmt.Errorf("name is required")
		}
		return nil
	}

	out := &strings.Builder{}
	form, output := NewAccountFieldsForm(fields, AccountFieldsData{}, validateName)
	form = form.WithAccessible(true).WithInput(strings.NewReader("\nValid\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	result := output()
	if result.Name != "Valid" {
		t.Fatalf("result.Name = %q, want Valid", result.Name)
	}
}

func TestAccountFieldsFormPreservesDefaults(t *testing.T) {
	fields := []calendar.AccountField{
		{Key: "client_id", Label: "OAuth Client ID", Required: true},
		{Key: "client_secret", Label: "OAuth Client Secret", Required: true, Secret: true},
	}
	defaults := AccountFieldsData{
		Name:     "Work",
		Settings: map[string]string{"client_id": "old-id"},
		Secrets:  map[string]string{"client_secret": "old-secret"},
	}
	validateName := func(string) error { return nil }

	out := &strings.Builder{}
	form, output := NewAccountFieldsForm(fields, defaults, validateName)
	form = form.WithAccessible(true).WithInput(strings.NewReader("\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	result := output()
	if result.Name != "Work" {
		t.Fatalf("result.Name = %q, want Work", result.Name)
	}
	if result.Settings["client_id"] != "old-id" {
		t.Fatalf("result.Settings[client_id] = %q, want old-id", result.Settings["client_id"])
	}
	if result.Secrets["client_secret"] != "old-secret" {
		t.Fatalf("result.Secrets[client_secret] = %q, want old-secret", result.Secrets["client_secret"])
	}
}

func TestAccountFieldsFormTrimsName(t *testing.T) {
	fields := []calendar.AccountField{}
	validateName := func(string) error { return nil }

	out := &strings.Builder{}
	form, output := NewAccountFieldsForm(fields, AccountFieldsData{}, validateName)
	form = form.WithAccessible(true).WithInput(strings.NewReader("  Work  \n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	result := output()
	if result.Name != "Work" {
		t.Fatalf("result.Name = %q, want Work", result.Name)
	}
}
