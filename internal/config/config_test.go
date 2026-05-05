package config

import (
	"errors"
	"reflect"
	"testing"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

func TestNewAccountID(t *testing.T) {
	id1, err := NewAccountID()
	if err != nil {
		t.Fatalf("NewAccountID() error = %v", err)
	}
	if id1 == "" {
		t.Fatal("NewAccountID() = empty")
	}

	id2, err := NewAccountID()
	if err != nil {
		t.Fatalf("NewAccountID() error = %v", err)
	}
	if id1 == id2 {
		t.Fatal("NewAccountID() returned duplicate IDs")
	}
}

func TestCloneDeepCopy(t *testing.T) {
	cfg := &Config{Accounts: []calendar.Account{
		{
			ID:      "a",
			Service: calendar.ServiceTypeGoogle,
			Name:    "Work",
			Settings: map[string]string{
				"client_id": "client",
			},
			Calendars: []calendar.CalendarRef{{ID: "primary", Name: "Primary"}},
		},
		{
			ID:        "b",
			Service:   calendar.ServiceType("outlook"),
			Name:      "Mail",
			Settings:  nil,
			Calendars: nil,
		},
	}}

	cloned := cfg.Clone()

	if cloned == cfg {
		t.Fatal("Clone() returned the same pointer")
	}

	if !reflect.DeepEqual(cfg, cloned) {
		t.Fatalf("Clone() did not produce an equal copy:\noriginal: %+v\ncloned:   %+v", cfg, cloned)
	}

	// Mutate clone and verify original is unaffected.
	cloned.Accounts[0].Name = "Mutated"
	cloned.Accounts[0].Settings["client_id"] = "mutated"
	cloned.Accounts[0].Calendars[0].Name = "Mutated"
	if cfg.Accounts[0].Name != "Work" {
		t.Fatalf("Clone() shallow-copied Name: original = %q", cfg.Accounts[0].Name)
	}
	if cfg.Accounts[0].Settings["client_id"] != "client" {
		t.Fatalf("Clone() shallow-copied Settings: original = %q", cfg.Accounts[0].Settings["client_id"])
	}
	if cfg.Accounts[0].Calendars[0].Name != "Primary" {
		t.Fatalf("Clone() shallow-copied Calendars: original = %q", cfg.Accounts[0].Calendars[0].Name)
	}
}

func TestNormalizeNilReceiver(t *testing.T) {
	var nilCfg *Config
	// Should not panic.
	nilCfg.Normalize()
}

func TestConfigNormalize(t *testing.T) {
	cfg := &Config{}
	cfg.Normalize()
	if cfg.Accounts == nil {
		t.Fatal("Accounts = nil after Normalize")
	}

	cfg = &Config{Accounts: []calendar.Account{
		{ID: "b", Name: "B"},
		{ID: "a", Name: "A", Settings: nil, Calendars: nil},
	}}
	cfg.Normalize()
	if cfg.Accounts[0].ID != "a" || cfg.Accounts[1].ID != "b" {
		t.Fatalf("accounts not sorted: %+v", cfg.Accounts)
	}
	for i, account := range cfg.Accounts {
		if account.Settings == nil {
			t.Fatalf("Accounts[%d].Settings = nil after Normalize", i)
		}
		if account.Calendars == nil {
			t.Fatalf("Accounts[%d].Calendars = nil after Normalize", i)
		}
	}
}

func TestConfigValidate(t *testing.T) {
	var nilCfg *Config
	if err := nilCfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	valid := &Config{Accounts: []calendar.Account{
		{ID: "a", Name: "Work"},
		{ID: "b", Name: "Personal"},
	}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	duplicateName := &Config{Accounts: []calendar.Account{
		{Name: "Work"},
		{Name: "Work"},
	}}
	if err := duplicateName.Validate(); !errors.Is(err, ErrDuplicateAccountName) {
		t.Fatalf("Validate() error = %v, want ErrDuplicateAccountName", err)
	}

	duplicateID := &Config{Accounts: []calendar.Account{
		{ID: "same", Name: "A"},
		{ID: "same", Name: "B"},
	}}
	if err := duplicateID.Validate(); !errors.Is(err, ErrDuplicateAccountID) {
		t.Fatalf("Validate() error = %v, want ErrDuplicateAccountID", err)
	}

	emptyName := &Config{Accounts: []calendar.Account{
		{ID: "a", Name: ""},
	}}
	if err := emptyName.Validate(); !errors.Is(err, ErrEmptyAccountName) {
		t.Fatalf("Validate() error = %v, want ErrEmptyAccountName", err)
	}

	whitespaceName := &Config{Accounts: []calendar.Account{
		{ID: "a", Name: "   "},
	}}
	if err := whitespaceName.Validate(); !errors.Is(err, ErrEmptyAccountName) {
		t.Fatalf("Validate() error = %v, want ErrEmptyAccountName", err)
	}
}

func TestConfigFindAccountByID(t *testing.T) {
	var nilCfg *Config
	if got := nilCfg.FindAccountByID("a"); got != nil {
		t.Fatal("FindAccountByID on nil config should return nil")
	}
}

func TestConfigFindAccountByName(t *testing.T) {
	var nilCfg *Config
	if got := nilCfg.FindAccountByName("a"); got != nil {
		t.Fatal("FindAccountByName on nil config should return nil")
	}
}

func TestConfigAccountNames(t *testing.T) {
	var nilCfg *Config
	if got := nilCfg.AccountNames(); len(got) != 0 {
		t.Fatalf("AccountNames() = %v, want empty", got)
	}
}

func TestConfigAccountsByService(t *testing.T) {
	var nilCfg *Config
	if got := nilCfg.AccountsByService(calendar.ServiceTypeGoogle); len(got) != 0 {
		t.Fatalf("AccountsByService() = %v, want empty", got)
	}
}

func TestConfigEnsureAccountIDs(t *testing.T) {
	var nilCfg *Config
	if err := nilCfg.EnsureAccountIDs(); err != nil {
		t.Fatalf("EnsureAccountIDs() error = %v, want nil", err)
	}

	cfg := &Config{Accounts: []calendar.Account{{Name: "Work"}}}
	if err := cfg.EnsureAccountIDs(); err != nil {
		t.Fatalf("EnsureAccountIDs() error = %v", err)
	}
	if cfg.Accounts[0].ID == "" {
		t.Fatal("EnsureAccountIDs() did not generate ID")
	}

	cfg2 := &Config{Accounts: []calendar.Account{
		{ID: "same", Name: "A"},
		{ID: "same", Name: "B"},
	}}
	if err := cfg2.EnsureAccountIDs(); !errors.Is(err, ErrDuplicateAccountID) {
		t.Fatalf("EnsureAccountIDs() error = %v, want ErrDuplicateAccountID", err)
	}
}
