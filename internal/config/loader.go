package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// ConfigDirName is the name of the configuration directory.
	ConfigDirName = "waybar-next-events"
	// ConfigFileName is the name of the configuration file.
	ConfigFileName = "config.json"
)

// Config represents the top-level configuration structure.
type Config struct {
	Google *GoogleCalendar `json:"google"`
}

// GoogleCalendar holds Google OAuth2 configuration.
type GoogleCalendar struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// Loader handles loading configuration from files.
type Loader struct {
	configPath string
}

// NewLoader creates a new config loader that reads from the default path.
func NewLoader() *Loader {
	return &Loader{
		configPath: DefaultConfigPath(),
	}
}

// NewLoaderWithPath creates a new config loader with a custom path.
// This is useful for testing.
func NewLoaderWithPath(path string) *Loader {
	return &Loader{
		configPath: path,
	}
}

// DefaultConfigPath returns the default configuration file path.
// It resolves to $HOME/.config/waybar-next-events/config.json.
func DefaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home cannot be determined
		return filepath.Join(".", ConfigDirName, ConfigFileName)
	}
	return filepath.Join(homeDir, ".config", ConfigDirName, ConfigFileName)
}

// Load reads and parses the configuration file.
// Returns an error if the file doesn't exist, is invalid JSON, or is missing required fields.
func (l *Loader) Load() (*Config, error) {
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found at %s: %w", l.configPath, err)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// GetGoogleConfig extracts the Google calendar configuration from the config.
// Returns an error if no Google calendar is configured or if clientId is missing.
func (c *Config) GetGoogleConfig() (*GoogleCalendar, error) {
	if c.Google == nil {
		return nil, fmt.Errorf("no google calendar configured")
	}
	if c.Google.ClientID == "" {
		return nil, fmt.Errorf("google calendar configuration missing required field: clientId")
	}
	return c.Google, nil
}
