package cli

import (
	"testing"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

func TestRootCommandInitialization(t *testing.T) {
	root := New(nil).RootCommand()
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

func TestAppWithRegistry(t *testing.T) {
	registry := calendar.NewRegistry()
	app := New(registry)
	if app.Registry != registry {
		t.Fatal("expected App to use provided registry")
	}
	if app.AccountManager == nil {
		t.Fatal("expected AccountManager to be initialized")
	}
	if app.EventFetcher == nil {
		t.Fatal("expected EventFetcher to be initialized")
	}
}
