package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_Load(t *testing.T) {
	t.Run("ValidSingleAccountConfig", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		configContent := `{
	"google": {
		"name": "Google Calendar",
		"accounts": [
			{
				"name": "Work",
				"clientId": "test-client-id",
				"clientSecret": "test-client-secret",
				"calendars": [
					{
						"name": "Work Calendar",
						"id": "work@example.com"
					}
				]
			}
		]
	}
}`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		loader := NewLoaderWithPath(configPath)
		cfg, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		googleCfg, err := cfg.GetGoogleConfig()
		if err != nil {
			t.Fatalf("GetGoogleConfig() error = %v", err)
		}

		if len(googleCfg.Accounts) != 1 {
			t.Fatalf("Accounts length = %d, want 1", len(googleCfg.Accounts))
		}
		if googleCfg.Accounts[0].ClientID != "test-client-id" {
			t.Errorf("ClientID = %s, want test-client-id", googleCfg.Accounts[0].ClientID)
		}
		if googleCfg.Accounts[0].ClientSecret != "test-client-secret" {
			t.Errorf("ClientSecret = %s, want test-client-secret", googleCfg.Accounts[0].ClientSecret)
		}
		if len(googleCfg.Accounts[0].Calendars) != 1 {
			t.Errorf("Calendars length = %d, want 1", len(googleCfg.Accounts[0].Calendars))
		}
		if googleCfg.Accounts[0].Calendars[0].Name != "Work Calendar" {
			t.Errorf("Calendar Name = %s, want Work Calendar", googleCfg.Accounts[0].Calendars[0].Name)
		}
		if googleCfg.Accounts[0].Calendars[0].ID != "work@example.com" {
			t.Errorf("Calendar ID = %s, want work@example.com", googleCfg.Accounts[0].Calendars[0].ID)
		}
	})

	t.Run("ValidMultiAccountConfig", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		configContent := `{
	"google": {
		"name": "Google Calendar",
		"accounts": [
			{
				"name": "Work",
				"clientId": "work-client-id",
				"clientSecret": "work-client-secret"
			},
			{
				"name": "Personal",
				"clientId": "personal-client-id",
				"clientSecret": "personal-client-secret",
				"calendars": [
					{
						"name": "Personal Calendar",
						"id": "personal@gmail.com"
					}
				]
			}
		]
	}
}`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		loader := NewLoaderWithPath(configPath)
		cfg, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		googleCfg, err := cfg.GetGoogleConfig()
		if err != nil {
			t.Fatalf("GetGoogleConfig() error = %v", err)
		}

		if len(googleCfg.Accounts) != 2 {
			t.Fatalf("Accounts length = %d, want 2", len(googleCfg.Accounts))
		}
		if googleCfg.Accounts[0].Name != "Work" {
			t.Errorf("Account[0] Name = %s, want Work", googleCfg.Accounts[0].Name)
		}
		if googleCfg.Accounts[1].Name != "Personal" {
			t.Errorf("Account[1] Name = %s, want Personal", googleCfg.Accounts[1].Name)
		}
	})

	t.Run("MissingFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "nonexistent.json")

		loader := NewLoaderWithPath(configPath)
		_, err := loader.Load()
		if err == nil {
			t.Error("Load() expected error for missing file, got nil")
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		configContent := `{invalid json}`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		loader := NewLoaderWithPath(configPath)
		_, err := loader.Load()
		if err == nil {
			t.Error("Load() expected error for invalid JSON, got nil")
		}
	})

	t.Run("EmptyConfig", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		configContent := `{}`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		loader := NewLoaderWithPath(configPath)
		cfg, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		_, err = cfg.GetGoogleConfig()
		if !errors.Is(err, ErrNoGoogleCalendar) {
			t.Errorf("GetGoogleConfig() error = %v, want ErrNoGoogleCalendar", err)
		}
	})

	t.Run("NoGoogleCalendar", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		configContent := `{
	"other": {
		"key": "value"
	}
}`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		loader := NewLoaderWithPath(configPath)
		cfg, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		_, err = cfg.GetGoogleConfig()
		if !errors.Is(err, ErrNoGoogleCalendar) {
			t.Errorf("GetGoogleConfig() error = %v, want ErrNoGoogleCalendar", err)
		}
	})

	t.Run("EmptyAccounts", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		configContent := `{
	"google": {
		"name": "Google Calendar",
		"accounts": []
	}
}`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		loader := NewLoaderWithPath(configPath)
		cfg, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		googleCfg, err := cfg.GetGoogleConfig()
		if err != nil {
			t.Fatalf("GetGoogleConfig() unexpected error: %v", err)
		}
		if len(googleCfg.Accounts) != 0 {
			t.Errorf("Accounts length = %d, want 0", len(googleCfg.Accounts))
		}
	})

	t.Run("AccountMissingClientID", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		configContent := `{
	"google": {
		"name": "Google Calendar",
		"accounts": [
			{
				"name": "Work",
				"clientSecret": "test-secret"
			}
		]
	}
}`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		loader := NewLoaderWithPath(configPath)
		cfg, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		_, err = cfg.GetGoogleConfig()
		if !errors.Is(err, ErrAccountMissingClientID) {
			t.Errorf("GetGoogleConfig() error = %v, want ErrAccountMissingClientID", err)
		}
	})

	t.Run("EmptyClientSecret", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		configContent := `{
	"google": {
		"name": "Google Calendar",
		"accounts": [
			{
				"name": "Work",
				"clientId": "test-client-id"
			}
		]
	}
}`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		loader := NewLoaderWithPath(configPath)
		cfg, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		googleCfg, err := cfg.GetGoogleConfig()
		if err != nil {
			t.Fatalf("GetGoogleConfig() error = %v", err)
		}

		if googleCfg.Accounts[0].ClientID != "test-client-id" {
			t.Errorf("ClientID = %s, want test-client-id", googleCfg.Accounts[0].ClientID)
		}
		// Empty clientSecret is valid for public clients
		if googleCfg.Accounts[0].ClientSecret != "" {
			t.Errorf("ClientSecret = %s, want empty", googleCfg.Accounts[0].ClientSecret)
		}
	})
}

func TestGoogleAccount_CalendarIDs(t *testing.T) {
	t.Run("CalendarsConfigured", func(t *testing.T) {
		account := GoogleAccount{
			Name:     "Work",
			ClientID: "test-id",
			Calendars: []Calendar{
				{Name: "Work Cal", ID: "work@example.com"},
				{Name: "Holidays", ID: "holidays@group.calendar.google.com"},
			},
		}
		ids := account.CalendarIDs()
		if len(ids) != 2 {
			t.Fatalf("CalendarIDs() length = %d, want 2", len(ids))
		}
		if ids[0] != "work@example.com" {
			t.Errorf("CalendarIDs()[0] = %s, want work@example.com", ids[0])
		}
		if ids[1] != "holidays@group.calendar.google.com" {
			t.Errorf("CalendarIDs()[1] = %s, want holidays@group.calendar.google.com", ids[1])
		}
	})

	t.Run("NoCalendars", func(t *testing.T) {
		account := GoogleAccount{
			Name:     "Work",
			ClientID: "test-id",
		}
		ids := account.CalendarIDs()
		if len(ids) != 1 || ids[0] != "primary" {
			t.Errorf("CalendarIDs() = %v, want [primary]", ids)
		}
	})

	t.Run("EmptyCalendars", func(t *testing.T) {
		account := GoogleAccount{
			Name:      "Work",
			ClientID:  "test-id",
			Calendars: []Calendar{},
		}
		ids := account.CalendarIDs()
		if len(ids) != 1 || ids[0] != "primary" {
			t.Errorf("CalendarIDs() = %v, want [primary]", ids)
		}
	})
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	if path == "" {
		t.Error("DefaultConfigPath() returned empty string")
	}
	// Should contain the config directory and filename
	if !contains(path, ConfigDirName) {
		t.Errorf("DefaultConfigPath() = %s, should contain %s", path, ConfigDirName)
	}
	if !contains(path, ConfigFileName) {
		t.Errorf("DefaultConfigPath() = %s, should contain %s", path, ConfigFileName)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
