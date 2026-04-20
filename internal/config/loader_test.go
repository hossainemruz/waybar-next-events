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
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
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
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
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
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
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
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
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
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
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
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
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
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
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
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
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

func TestLoader_Save(t *testing.T) {
	t.Run("SaveAndRoundTrip", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		loader := NewLoaderWithPath(configPath)

		cfg := &Config{
			Google: &GoogleCalendar{
				Name: "Google Calendar",
				Accounts: []GoogleAccount{
					{
						Name:         "Work",
						ClientID:     "test-client-id",
						ClientSecret: "test-client-secret",
						Calendars: []Calendar{
							{Name: "Work Calendar", ID: "work@example.com"},
						},
					},
				},
			},
		}

		if err := loader.Save(cfg); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		loaded, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() after Save() error = %v", err)
		}

		got, err := loaded.GetGoogleConfig()
		if err != nil {
			t.Fatalf("GetGoogleConfig() error = %v", err)
		}

		if len(got.Accounts) != 1 {
			t.Fatalf("Accounts length = %d, want 1", len(got.Accounts))
		}
		if got.Accounts[0].Name != "Work" {
			t.Errorf("Account Name = %s, want Work", got.Accounts[0].Name)
		}
		if got.Accounts[0].ClientID != "test-client-id" {
			t.Errorf("ClientID = %s, want test-client-id", got.Accounts[0].ClientID)
		}
	})

	t.Run("CreatesParentDirectory", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "nested", "dir", "config.json")
		loader := NewLoaderWithPath(configPath)

		cfg := &Config{
			Google: &GoogleCalendar{
				Name:     "Google Calendar",
				Accounts: []GoogleAccount{},
			},
		}

		if err := loader.Save(cfg); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Error("config file was not created")
		}
	})

	t.Run("PreservesEmptyAccounts", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		loader := NewLoaderWithPath(configPath)

		cfg := &Config{
			Google: &GoogleCalendar{
				Name:     "Google Calendar",
				Accounts: []GoogleAccount{},
			},
		}

		if err := loader.Save(cfg); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		loaded, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		got, err := loaded.GetGoogleConfig()
		if err != nil {
			t.Fatalf("GetGoogleConfig() error = %v", err)
		}

		if len(got.Accounts) != 0 {
			t.Errorf("Accounts length = %d, want 0", len(got.Accounts))
		}
	})

	t.Run("OverwritesExistingFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")

		// Write initial config
		initialContent := `{"google": {"name": "Old Name", "accounts": []}}`
		if err := os.WriteFile(configPath, []byte(initialContent), 0o644); err != nil {
			t.Fatalf("Failed to write initial config: %v", err)
		}

		loader := NewLoaderWithPath(configPath)
		cfg := &Config{
			Google: &GoogleCalendar{
				Name:     "New Name",
				Accounts: []GoogleAccount{},
			},
		}

		if err := loader.Save(cfg); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		loaded, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		got, err := loaded.GetGoogleConfig()
		if err != nil {
			t.Fatalf("GetGoogleConfig() error = %v", err)
		}

		if got.Name != "New Name" {
			t.Errorf("Name = %s, want New Name", got.Name)
		}
	})

	t.Run("RejectsNilConfig", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		loader := NewLoaderWithPath(configPath)

		err := loader.Save(nil)
		if err == nil {
			t.Fatal("Save(nil) expected error, got nil")
		}

		// File should not have been created
		if _, err := os.Stat(configPath); !os.IsNotExist(err) {
			t.Error("Save(nil) should not create a file")
		}
	})

	t.Run("NormalizesNilGoogleSection", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		loader := NewLoaderWithPath(configPath)

		cfg := &Config{Google: nil}

		if err := loader.Save(cfg); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		loaded, err := loader.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		got, err := loaded.GetGoogleConfig()
		if err != nil {
			t.Fatalf("GetGoogleConfig() unexpected error: %v", err)
		}

		if got.Name != "Google Calendar" {
			t.Errorf("Google.Name = %s, want Google Calendar", got.Name)
		}
		if len(got.Accounts) != 0 {
			t.Errorf("Accounts length = %d, want 0", len(got.Accounts))
		}
	})
}

func TestLoader_LoadOrEmpty(t *testing.T) {
	t.Run("ReturnsEmptyConfigWhenFileNotFound", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "nonexistent.json")
		loader := NewLoaderWithPath(configPath)

		cfg, err := loader.LoadOrEmpty()
		if err != nil {
			t.Fatalf("LoadOrEmpty() error = %v", err)
		}
		if cfg == nil {
			t.Fatal("LoadOrEmpty() returned nil config")
		}
		if cfg.Google != nil {
			t.Error("Expected nil Google field for empty config")
		}
	})

	t.Run("ReturnsErrorForMalformedJSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		if err := os.WriteFile(configPath, []byte(`{invalid json}`), 0o644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		loader := NewLoaderWithPath(configPath)
		_, err := loader.LoadOrEmpty()
		if err == nil {
			t.Error("LoadOrEmpty() expected error for malformed JSON, got nil")
		}
	})

	t.Run("ReturnsConfigWhenFileExists", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		configContent := `{
	"google": {
		"name": "Google Calendar",
		"accounts": [
			{
				"name": "Work",
				"clientId": "test-id",
				"clientSecret": "test-secret"
			}
		]
	}
}`
		if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		loader := NewLoaderWithPath(configPath)
		cfg, err := loader.LoadOrEmpty()
		if err != nil {
			t.Fatalf("LoadOrEmpty() error = %v", err)
		}

		got, err := cfg.GetGoogleConfig()
		if err != nil {
			t.Fatalf("GetGoogleConfig() error = %v", err)
		}
		if len(got.Accounts) != 1 {
			t.Errorf("Accounts length = %d, want 1", len(got.Accounts))
		}
	})
}

func TestGoogleCalendar_FindAccountByName(t *testing.T) {
	cal := &GoogleCalendar{
		Name: "Google Calendar",
		Accounts: []GoogleAccount{
			{Name: "Work", ClientID: "work-id", ClientSecret: "work-secret"},
			{Name: "Personal", ClientID: "personal-id", ClientSecret: "personal-secret"},
		},
	}

	t.Run("FindsExistingAccount", func(t *testing.T) {
		acc := cal.FindAccountByName("Work")
		if acc == nil {
			t.Fatal("FindAccountByName(Work) returned nil")
		}
		if acc.Name != "Work" {
			t.Errorf("Account Name = %s, want Work", acc.Name)
		}
	})

	t.Run("ReturnsNilForMissingAccount", func(t *testing.T) {
		acc := cal.FindAccountByName("NonExistent")
		if acc != nil {
			t.Errorf("FindAccountByName(NonExistent) = %+v, want nil", acc)
		}
	})
}

func TestGoogleCalendar_AccountNames(t *testing.T) {
	t.Run("ReturnsAllNames", func(t *testing.T) {
		cal := &GoogleCalendar{
			Name: "Google Calendar",
			Accounts: []GoogleAccount{
				{Name: "Work"},
				{Name: "Personal"},
			},
		}
		names := cal.AccountNames()
		if len(names) != 2 || names[0] != "Work" || names[1] != "Personal" {
			t.Errorf("AccountNames() = %v, want [Work Personal]", names)
		}
	})

	t.Run("ReturnsEmptySliceForNoAccounts", func(t *testing.T) {
		cal := &GoogleCalendar{
			Name:     "Google Calendar",
			Accounts: []GoogleAccount{},
		}
		names := cal.AccountNames()
		if len(names) != 0 {
			t.Errorf("AccountNames() = %v, want empty slice", names)
		}
	})
}

func TestConfig_EnsureGoogleInitialized(t *testing.T) {
	t.Run("InitializesNilGoogle", func(t *testing.T) {
		cfg := &Config{}
		cfg.EnsureGoogleInitialized()
		if cfg.Google == nil {
			t.Fatal("EnsureGoogleInitialized() did not initialize Google field")
		}
		if cfg.Google.Name != "Google Calendar" {
			t.Errorf("Google.Name = %s, want Google Calendar", cfg.Google.Name)
		}
		if cfg.Google.Accounts == nil {
			t.Error("Accounts should be initialized as empty slice, not nil")
		}
		if len(cfg.Google.Accounts) != 0 {
			t.Errorf("Accounts length = %d, want 0", len(cfg.Google.Accounts))
		}
	})

	t.Run("DoesNotOverwriteExistingGoogle", func(t *testing.T) {
		cfg := &Config{
			Google: &GoogleCalendar{
				Name: "Existing",
				Accounts: []GoogleAccount{
					{Name: "Work"},
				},
			},
		}
		cfg.EnsureGoogleInitialized()
		if cfg.Google.Name != "Existing" {
			t.Errorf("Google.Name = %s, want Existing", cfg.Google.Name)
		}
		if len(cfg.Google.Accounts) != 1 {
			t.Errorf("Accounts length = %d, want 1", len(cfg.Google.Accounts))
		}
	})
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
