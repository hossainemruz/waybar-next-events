package forms

import (
	"strings"
	"testing"
)

func TestDeleteConfirmFormAcceptsYes(t *testing.T) {
	confirmed := false
	out := &strings.Builder{}
	form := NewDeleteConfirmForm("Work", &confirmed).WithAccessible(true).WithInput(strings.NewReader("y\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	if !confirmed {
		t.Fatal("confirmed = false, want true")
	}
	output := out.String()
	if !strings.Contains(output, "Work") {
		t.Fatalf("output does not contain account name: %q", output)
	}
}

func TestDeleteConfirmFormDefaultsToNo(t *testing.T) {
	confirmed := false
	out := &strings.Builder{}
	form := NewDeleteConfirmForm("Work", &confirmed).WithAccessible(true).WithInput(strings.NewReader("\n")).WithOutput(out)
	if err := form.Run(); err != nil {
		t.Fatalf("form.Run() error = %v", err)
	}
	if confirmed {
		t.Fatal("confirmed = true, want false")
	}
}
