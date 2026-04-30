package config

import (
	"bytes"
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
	// configDirPermission is the permission used for config directories.
	configDirPermission = 0o700
	// ConfigFilePermission is the permission used for config files.
	ConfigFilePermission = 0o600
)

// Loader handles loading configuration from files.
type Loader struct {
	configPath string
}

// Snapshot captures the current on-disk state of a config file so it can be
// restored later.
type Snapshot struct {
	exists bool
	data   []byte
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

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	cfg.Normalize()

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
			cfg := &Config{}
			cfg.Normalize()
			return cfg, nil
		}
		return nil, err
	}
	return cfg, nil
}

// Save marshals the Config to JSON and writes it to the configured file path.
// It creates the parent directory if it does not exist.
// The JSON is written with indentation for readability.
// Returns ErrNilConfig if cfg is nil.
func (l *Loader) Save(cfg *Config) error {
	if cfg == nil {
		return errors.New("cannot save nil config")
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	cfg.Normalize()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure the parent directory exists.
	dir := filepath.Dir(l.configPath)
	if err := os.MkdirAll(dir, configDirPermission); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	if err := os.WriteFile(l.configPath, data, ConfigFilePermission); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Snapshot returns the current config file contents.
// If the config file does not exist yet, it returns an empty snapshot.
func (l *Loader) Snapshot() (Snapshot, error) {
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{}, nil
		}
		return Snapshot{}, fmt.Errorf("failed to read config snapshot: %w", err)
	}

	return Snapshot{exists: true, data: bytes.Clone(data)}, nil
}

// RestoreSnapshot restores a snapshot previously returned by Snapshot.
func (l *Loader) RestoreSnapshot(snapshot Snapshot) error {
	if !snapshot.exists {
		if err := os.Remove(l.configPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove config file: %w", err)
		}
		return nil
	}

	dir := filepath.Dir(l.configPath)
	if err := os.MkdirAll(dir, configDirPermission); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	if err := os.WriteFile(l.configPath, snapshot.data, ConfigFilePermission); err != nil {
		return fmt.Errorf("failed to restore config file: %w", err)
	}

	return nil
}
