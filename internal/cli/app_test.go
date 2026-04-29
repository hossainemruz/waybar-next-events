package cli

import "testing"

func TestRootCommandInitialization(t *testing.T) {
	root := New().RootCommand()
	if root == nil {
		t.Fatal("root command = nil")
	}

	if root.Name() != "waybar-next-events" {
		t.Fatalf("root command name = %q, want %q", root.Name(), "waybar-next-events")
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
