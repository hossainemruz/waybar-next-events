// Package config contains persisted configuration types and constants.
package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

const (
	// DefaultCallbackPort is the default port for the OAuth2 callback server.
	// This port is used by the local HTTP server that receives OAuth2 callbacks.
	DefaultCallbackPort = "18751"

	// DefaultCallbackURL is the full OAuth2 redirect callback URL.
	// Providers must use this exact URL as their RedirectURL to ensure the
	// callback server can receive the redirect. This constant centralizes the
	// contract between provider validation and the callback server implementation.
	DefaultCallbackURL = "http://127.0.0.1:" + DefaultCallbackPort + "/callback"
)

// Config represents the top-level configuration structure.
type Config struct {
	Accounts []calendar.Account `json:"accounts"`
}

// Account is the persisted generic account model.
type Account = calendar.Account

// CalendarRef is the persisted generic calendar reference model.
type CalendarRef = calendar.CalendarRef

// NewAccountID generates a stable random account identifier.
func NewAccountID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("generate account id: %w", err)
	}

	return id.String(), nil
}

// Normalize prepares the config for deterministic persistence.
func (c *Config) Normalize() {
	if c == nil {
		return
	}

	if c.Accounts == nil {
		c.Accounts = []calendar.Account{}
		return
	}

	for i := range c.Accounts {
		normalizeAccount(&c.Accounts[i])
	}

	slices.SortFunc(c.Accounts, func(a, b calendar.Account) int {
		return strings.Compare(a.ID, b.ID)
	})
}

// Clone returns a deep copy of the Config.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	cloned := &Config{
		Accounts: make([]calendar.Account, len(c.Accounts)),
	}

	for i, account := range c.Accounts {
		cloned.Accounts[i] = calendar.Account{
			ID:      account.ID,
			Service: account.Service,
			Name:    account.Name,
		}

		if account.Settings != nil {
			cloned.Accounts[i].Settings = make(map[string]string, len(account.Settings))
			for k, v := range account.Settings {
				cloned.Accounts[i].Settings[k] = v
			}
		}

		if account.Calendars != nil {
			cloned.Accounts[i].Calendars = make([]calendar.CalendarRef, len(account.Calendars))
			copy(cloned.Accounts[i].Calendars, account.Calendars)
		}
	}

	return cloned
}

// Validate checks config invariants.
func (c *Config) Validate() error {
	if c == nil {
		return nil
	}

	seenNames := make(map[string]struct{}, len(c.Accounts))
	for i, account := range c.Accounts {
		if strings.TrimSpace(account.Name) == "" {
			return fmt.Errorf("account %d: %w", i, ErrEmptyAccountName)
		}
		if _, exists := seenNames[account.Name]; exists {
			return fmt.Errorf("%w: %q", ErrDuplicateAccountName, account.Name)
		}
		seenNames[account.Name] = struct{}{}
	}

	return c.EnsureAccountIDs()
}

// FindAccountByID returns the account with the given stable ID.
func (c *Config) FindAccountByID(id string) *calendar.Account {
	if c == nil {
		return nil
	}

	for i := range c.Accounts {
		if c.Accounts[i].ID == id {
			return &c.Accounts[i]
		}
	}

	return nil
}

// FindAccountByName returns the account with the given name.
func (c *Config) FindAccountByName(name string) *calendar.Account {
	if c == nil {
		return nil
	}

	for i := range c.Accounts {
		if c.Accounts[i].Name == name {
			return &c.Accounts[i]
		}
	}

	return nil
}

// AccountNames returns all configured account names.
func (c *Config) AccountNames() []string {
	if c == nil || len(c.Accounts) == 0 {
		return []string{}
	}

	names := make([]string, 0, len(c.Accounts))
	for _, account := range c.Accounts {
		names = append(names, account.Name)
	}

	return names
}

// AccountsByService returns all accounts configured for a service type.
func (c *Config) AccountsByService(service calendar.ServiceType) []calendar.Account {
	if c == nil {
		return []calendar.Account{}
	}

	accounts := make([]calendar.Account, 0)
	for _, account := range c.Accounts {
		if account.Service == service {
			accounts = append(accounts, account)
		}
	}

	return accounts
}

// EnsureAccountIDs validates existing account IDs and fills in missing IDs.
func (c *Config) EnsureAccountIDs() error {
	if c == nil {
		return nil
	}

	seen := make(map[string]struct{}, len(c.Accounts))
	for i := range c.Accounts {
		if strings.TrimSpace(c.Accounts[i].ID) == "" {
			id, err := NewAccountID()
			if err != nil {
				return err
			}
			c.Accounts[i].ID = id
		}

		if _, exists := seen[c.Accounts[i].ID]; exists {
			return fmt.Errorf("%w: %q", ErrDuplicateAccountID, c.Accounts[i].ID)
		}
		seen[c.Accounts[i].ID] = struct{}{}
	}

	return nil
}

func normalizeAccount(account *calendar.Account) {
	if account.Settings == nil {
		account.Settings = map[string]string{}
	}
	if account.Calendars == nil {
		account.Calendars = []calendar.CalendarRef{}
	}

	slices.SortFunc(account.Calendars, func(a, b calendar.CalendarRef) int {
		return strings.Compare(a.ID, b.ID)
	})
}
