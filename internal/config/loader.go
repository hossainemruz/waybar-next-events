package config

import (
	"encoding/json"
	"errors"
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
			return nil, &ErrConfigNotFound{Path: l.configPath}
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// LoadOrEmpty loads the configuration file, or returns an empty Config if the
// file does not exist. This is useful for commands like "account add" that
// need to work on first run when no config file exists yet.
// Returns an error only if the file exists but contains malformed JSON.
func (l *Loader) LoadOrEmpty() (*Config, error) {
	cfg, err := l.Load()
	if err != nil {
		// If the file simply doesn't exist, return an empty config (first run).
		var notFound *ErrConfigNotFound
		if errors.As(err, &notFound) {
			return &Config{}, nil
		}
		return nil, err
	}
	return cfg, nil
}

// Save marshals the Config to JSON and writes it to the configured file path.
// It creates the parent directory if it does not exist.
// The JSON is written with indentation for readability.
func (l *Loader) Save(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure the parent directory exists.
	dir := filepath.Dir(l.configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	if err := os.WriteFile(l.configPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
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
