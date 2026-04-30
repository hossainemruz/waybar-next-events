package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
)

func writeTestConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), appconfig.ConfigFilePermission); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}

func writeGenericConfig(t *testing.T, accounts []appconfig.Account) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	data, err := json.Marshal(&appconfig.Config{Accounts: accounts})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, data, appconfig.ConfigFilePermission); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	return data
}

func assertConfigUnchanged(t *testing.T, configPath string, original []byte) {
	t.Helper()
	after := readFile(t, configPath)
	if string(after) != string(original) {
		t.Fatalf("config changed unexpectedly\n got: %s\nwant: %s", string(after), string(original))
	}
}
