package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

func TestLoaderLoadAndSaveRoundTrip(t *testing.T) {
	loader := NewLoaderWithPath(filepath.Join(t.TempDir(), "config.json"))

	cfg := &Config{
		Accounts: []Account{
			{
				ID:      "acct-b",
				Service: calendar.ServiceTypeGoogle,
				Name:    "Personal",
				Settings: map[string]string{
					"client_id": "personal-client",
				},
				Calendars: []CalendarRef{{ID: "home", Name: "Home"}},
			},
			{
				ID:      "acct-a",
				Service: calendar.ServiceTypeGoogle,
				Name:    "Work",
				Settings: map[string]string{
					"client_id": "work-client",
				},
				Calendars: []CalendarRef{{ID: "team", Name: "Team"}, {ID: "primary", Name: "Primary"}},
			},
		},
	}

	if err := loader.Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded.Accounts) != 2 {
		t.Fatalf("len(loaded.Accounts) = %d, want 2", len(loaded.Accounts))
	}
	if loaded.Accounts[0].ID != "acct-a" || loaded.Accounts[1].ID != "acct-b" {
		t.Fatalf("loaded account order = [%s %s], want [acct-a acct-b]", loaded.Accounts[0].ID, loaded.Accounts[1].ID)
	}
	if got := loaded.FindAccountByID("acct-a"); got == nil || got.Name != "Work" {
		t.Fatalf("FindAccountByID(acct-a) = %+v, want Work account", got)
	}
	if got := loaded.FindAccountByName("Personal"); got == nil || got.ID != "acct-b" {
		t.Fatalf("FindAccountByName(Personal) = %+v, want acct-b", got)
	}
	if ids := loaded.FindAccountByID("acct-a").CalendarIDs(); len(ids) != 2 || ids[0] != "primary" || ids[1] != "team" {
		t.Fatalf("CalendarIDs() = %v, want [primary team]", ids)
	}
}

func TestLoaderLoadOrEmptyReturnsEmptyConfig(t *testing.T) {
	loader := NewLoaderWithPath(filepath.Join(t.TempDir(), "missing.json"))

	cfg, err := loader.LoadOrEmpty()
	if err != nil {
		t.Fatalf("LoadOrEmpty() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadOrEmpty() returned nil config")
	}
	if cfg.Accounts == nil {
		t.Fatal("Accounts = nil, want empty slice")
	}
	if len(cfg.Accounts) != 0 {
		t.Fatalf("len(cfg.Accounts) = %d, want 0", len(cfg.Accounts))
	}
}

func TestLoaderSaveGeneratesMissingAccountIDs(t *testing.T) {
	loader := NewLoaderWithPath(filepath.Join(t.TempDir(), "config.json"))

	cfg := &Config{
		Accounts: []Account{{
			Service:  calendar.ServiceTypeGoogle,
			Name:     "Work",
			Settings: map[string]string{"client_id": "client"},
		}},
	}

	if err := loader.Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Save must not mutate the caller's config.
	if cfg.Accounts[0].ID != "" {
		t.Fatal("Save() mutated caller config by generating an ID")
	}

	// The saved file must contain the generated ID.
	loaded, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Accounts[0].ID == "" {
		t.Fatal("Save() did not generate account ID in saved file")
	}
}

func TestLoaderSaveRejectsDuplicateAccountNames(t *testing.T) {
	loader := NewLoaderWithPath(filepath.Join(t.TempDir(), "config.json"))

	err := loader.Save(&Config{Accounts: []Account{
		{ID: "a", Service: calendar.ServiceTypeGoogle, Name: "Work"},
		{ID: "b", Service: calendar.ServiceTypeGoogle, Name: "Work"},
	}})
	if !errors.Is(err, ErrDuplicateAccountName) {
		t.Fatalf("Save() error = %v, want ErrDuplicateAccountName", err)
	}
}

func TestLoaderSaveRejectsDuplicateAccountIDs(t *testing.T) {
	loader := NewLoaderWithPath(filepath.Join(t.TempDir(), "config.json"))

	err := loader.Save(&Config{Accounts: []Account{
		{ID: "same-id", Service: calendar.ServiceTypeGoogle, Name: "Work"},
		{ID: "same-id", Service: calendar.ServiceType("outlook"), Name: "Mail"},
	}})
	if !errors.Is(err, ErrDuplicateAccountID) {
		t.Fatalf("Save() error = %v, want ErrDuplicateAccountID", err)
	}
}

func TestLoaderSaveIsDeterministic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	loader := NewLoaderWithPath(path)

	cfg := &Config{Accounts: []Account{{
		ID:      "acct-2",
		Service: calendar.ServiceTypeGoogle,
		Name:    "Work",
		Settings: map[string]string{
			"client_id": "client",
		},
		Calendars: []CalendarRef{{ID: "b", Name: "B"}, {ID: "a", Name: "A"}},
	}, {
		ID:      "acct-1",
		Service: calendar.ServiceType("outlook"),
		Name:    "Mail",
	}}}

	if err := loader.Save(cfg); err != nil {
		t.Fatalf("first Save() error = %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if err := loader.Save(cfg); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(first) != string(second) {
		t.Fatalf("saved JSON changed between writes\nfirst: %s\nsecond: %s", first, second)
	}
	if !strings.Contains(string(first), `"accounts"`) || strings.Contains(string(first), `"google":`) {
		t.Fatalf("saved JSON = %s, want generic accounts shape only", first)
	}
	if strings.Contains(string(first), `"client_secret"`) {
		t.Fatalf("saved JSON unexpectedly contained client_secret: %s", first)
	}
}

func TestConfigHelpers(t *testing.T) {
	cfg := &Config{Accounts: []Account{
		{ID: "a", Service: calendar.ServiceTypeGoogle, Name: "Work"},
		{ID: "b", Service: calendar.ServiceType("outlook"), Name: "Mail"},
		{ID: "c", Service: calendar.ServiceTypeGoogle, Name: "Personal"},
	}}

	if got := cfg.AccountNames(); len(got) != 3 || got[0] != "Work" || got[1] != "Mail" || got[2] != "Personal" {
		t.Fatalf("AccountNames() = %v", got)
	}
	googleAccounts := cfg.AccountsByService(calendar.ServiceTypeGoogle)
	if len(googleAccounts) != 2 {
		t.Fatalf("len(AccountsByService(google)) = %d, want 2", len(googleAccounts))
	}
}

func TestLoaderSnapshotAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	loader := NewLoaderWithPath(path)

	if err := os.WriteFile(path, []byte(`{"accounts":[]}`), ConfigFilePermission); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	snapshot, err := loader.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	if err := os.WriteFile(path, []byte(`{"accounts":[{"id":"a","service":"google","name":"Work"}]}`), ConfigFilePermission); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := loader.RestoreSnapshot(snapshot); err != nil {
		t.Fatalf("RestoreSnapshot() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != `{"accounts":[]}` {
		t.Fatalf("restored config = %s, want empty accounts config", data)
	}
}

func TestLoaderSaveUsesRestrictedPermissions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "config.json")
	loader := NewLoaderWithPath(path)

	if err := loader.Save(&Config{Accounts: []Account{}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat(dir) error = %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != configDirPermission {
		t.Fatalf("dir mode = %o, want %o", got, configDirPermission)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(file) error = %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != ConfigFilePermission {
		t.Fatalf("file mode = %o, want %o", got, ConfigFilePermission)
	}
}

func TestLoaderSaveDoesNotMutateCallerConfig(t *testing.T) {
	loader := NewLoaderWithPath(filepath.Join(t.TempDir(), "config.json"))

	cfg := &Config{
		Accounts: []Account{
			{
				ID:      "acct-b",
				Service: calendar.ServiceTypeGoogle,
				Name:    "Personal",
				Settings: map[string]string{
					"client_id": "personal-client",
				},
				Calendars: []CalendarRef{{ID: "home", Name: "Home"}},
			},
			{
				ID:        "acct-a",
				Service:   calendar.ServiceTypeGoogle,
				Name:      "Work",
				Settings:  nil,
				Calendars: nil,
			},
		},
	}

	// Capture pre-save state via independent manual construction (not Clone()).
	preSave := &Config{
		Accounts: []Account{
			{
				ID:      "acct-b",
				Service: calendar.ServiceTypeGoogle,
				Name:    "Personal",
				Settings: map[string]string{
					"client_id": "personal-client",
				},
				Calendars: []CalendarRef{{ID: "home", Name: "Home"}},
			},
			{
				ID:        "acct-a",
				Service:   calendar.ServiceTypeGoogle,
				Name:      "Work",
				Settings:  nil,
				Calendars: nil,
			},
		},
	}

	if err := loader.Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Assert that the caller's config pointer was not mutated.
	if len(cfg.Accounts) != len(preSave.Accounts) {
		t.Fatalf("Save() mutated caller config: len(Accounts) changed from %d to %d", len(preSave.Accounts), len(cfg.Accounts))
	}
	for i := range cfg.Accounts {
		if cfg.Accounts[i].ID != preSave.Accounts[i].ID {
			t.Fatalf("Save() mutated caller config: Accounts[%d].ID changed from %q to %q", i, preSave.Accounts[i].ID, cfg.Accounts[i].ID)
		}
		if cfg.Accounts[i].Name != preSave.Accounts[i].Name {
			t.Fatalf("Save() mutated caller config: Accounts[%d].Name changed from %q to %q", i, preSave.Accounts[i].Name, cfg.Accounts[i].Name)
		}
		if (cfg.Accounts[i].Settings == nil) != (preSave.Accounts[i].Settings == nil) {
			t.Fatalf("Save() mutated caller config: Accounts[%d].Settings nil-ness changed", i)
		}
		if (cfg.Accounts[i].Calendars == nil) != (preSave.Accounts[i].Calendars == nil) {
			t.Fatalf("Save() mutated caller config: Accounts[%d].Calendars nil-ness changed", i)
		}
	}
}

func TestLoaderRestoreSnapshotUsesRestrictedPermissions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "config.json")
	loader := NewLoaderWithPath(path)

	snapshot := Snapshot{exists: true, data: []byte(`{"accounts":[]}`)}
	if err := loader.RestoreSnapshot(snapshot); err != nil {
		t.Fatalf("RestoreSnapshot() error = %v", err)
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat(dir) error = %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != configDirPermission {
		t.Fatalf("dir mode = %o, want %o", got, configDirPermission)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(file) error = %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != ConfigFilePermission {
		t.Fatalf("file mode = %o, want %o", got, ConfigFilePermission)
	}
}

func TestDefaultConfigPathRespectsXDGConfigHome(t *testing.T) {
	xdgDir := "/tmp/xdg-test-config"
	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	got := DefaultConfigPath()
	want := filepath.Join(xdgDir, ConfigDirName, ConfigFileName)
	if got != want {
		t.Fatalf("DefaultConfigPath() = %q, want %q when XDG_CONFIG_HOME=%q", got, want, xdgDir)
	}
}

func TestDefaultConfigPathWithoutXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	got := DefaultConfigPath()
	want := filepath.Join(homeDir, ".config", ConfigDirName, ConfigFileName)
	if got != want {
		t.Fatalf("DefaultConfigPath() = %q, want %q", got, want)
	}
}
