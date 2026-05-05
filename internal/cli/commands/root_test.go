package commands

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandRegistration(t *testing.T) {
	root := BuildRoot(&AppDeps{})
	if root == nil {
		t.Fatal("root command = nil")
	}

	if root.Name() != "waybar-next-events" {
		t.Fatalf("root command name = %q, want %q", root.Name(), "waybar-next-events")
	}

	accountCommand, _, err := root.Find([]string{"account"})
	if err != nil {
		t.Fatalf("root.Find(account) error = %v", err)
	}
	if accountCommand == nil {
		t.Fatal("account command is not registered on root command")
	}

	for _, commandName := range []string{"list", "account"} {
		command, _, err := root.Find([]string{commandName})
		if err != nil {
			t.Fatalf("root.Find(%q) error = %v", commandName, err)
		}
		if command == nil {
			t.Fatalf("root.Find(%q) returned nil command", commandName)
		}
	}
}

func TestRootCommandVersionFlag(t *testing.T) {
	root := BuildRoot(&AppDeps{})

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"--version"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("root.Execute() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, Version) {
		t.Fatalf("version output = %q, want it to contain %q", output, Version)
	}
}
