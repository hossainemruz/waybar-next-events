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
// It is designed to be extensible: additional calendar service providers
// (e.g., "outlook") can be added as new top-level fields.
type Config struct {
	Google *GoogleCalendar `json:"google"`
}

// GoogleCalendar holds Google Calendar provider configuration,
// including display metadata and a list of accounts.
type GoogleCalendar struct {
	Name     string          `json:"name"`
	Accounts []GoogleAccount `json:"accounts"`
}

// GoogleAccount represents a single Google account with its own OAuth2 credentials
// and an optional list of calendars to fetch events from.
type GoogleAccount struct {
	Name         string     `json:"name"`
	ClientID     string     `json:"clientId"`
	ClientSecret string     `json:"clientSecret"`
	Calendars    []Calendar `json:"calendars"`
}

// Calendar represents a single calendar within a Google account.
type Calendar struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// CalendarIDs returns the calendar IDs for this account.
// If no calendars are configured, it defaults to ["primary"].
func (a *GoogleAccount) CalendarIDs() []string {
	if len(a.Calendars) == 0 {
		return []string{"primary"}
	}
	ids := make([]string, len(a.Calendars))
	for i, cal := range a.Calendars {
		ids[i] = cal.ID
	}
	return ids
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

// GetGoogleConfig extracts and validates the Google calendar configuration from the config.
// Returns an error if no Google calendar is configured or if any account is missing a required clientId.
// An empty accounts list is valid and results in a GoogleCalendar with no accounts.
func (c *Config) GetGoogleConfig() (*GoogleCalendar, error) {
	// Validate google section exists
	if c.Google == nil {
		return nil, ErrNoGoogleCalendar
	}
	// Validate no account is missing clientId
	for i, acc := range c.Google.Accounts {
		if acc.ClientID == "" {
			return nil, fmt.Errorf("%w: account %q (index %d)", ErrAccountMissingClientID, acc.Name, i)
		}
	}
	return c.Google, nil
}
