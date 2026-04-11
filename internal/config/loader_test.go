package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_Load(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		configContent := `{
	"google": {
		"clientId": "test-client-id",
		"clientSecret": "test-client-secret"
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

		if googleCfg.ClientID != "test-client-id" {
			t.Errorf("ClientID = %s, want test-client-id", googleCfg.ClientID)
		}
		if googleCfg.ClientSecret != "test-client-secret" {
			t.Errorf("ClientSecret = %s, want test-client-secret", googleCfg.ClientSecret)
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
		if err == nil {
			t.Error("GetGoogleConfig() expected error for empty config, got nil")
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
		if err == nil {
			t.Error("GetGoogleConfig() expected error when no google calendar, got nil")
		}
	})

	t.Run("GoogleCalendarMissingClientID", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		configContent := `{
	"google": {
		"clientSecret": "test-secret"
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
		if err == nil {
			t.Error("GetGoogleConfig() expected error when clientId is missing, got nil")
		}
	})

	t.Run("EmptyClientSecret", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		configContent := `{
	"google": {
		"clientId": "test-client-id"
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

		if googleCfg.ClientID != "test-client-id" {
			t.Errorf("ClientID = %s, want test-client-id", googleCfg.ClientID)
		}
		// Empty clientSecret is valid for public clients
		if googleCfg.ClientSecret != "" {
			t.Errorf("ClientSecret = %s, want empty", googleCfg.ClientSecret)
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
