package tokenstore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

const (
	// serviceName is the keyring service name for all stored tokens.
	serviceName = "waybar-next-events"
)

// KeyringTokenStore implements TokenStore using the OS keyring.
// On Linux, this uses the Secret Service API (D-Bus).
type KeyringTokenStore struct{}

// NewKeyringTokenStore creates a new keyring-based token store.
func NewKeyringTokenStore() *KeyringTokenStore {
	return &KeyringTokenStore{}
}

// Set stores the token in the keyring for the given provider.
func (s *KeyringTokenStore) Set(ctx context.Context, providerName string, token *oauth2.Token) error {
	if providerName == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if token == nil {
		return fmt.Errorf("token cannot be nil")
	}

	// Serialize token to JSON for storage
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to serialize token: %w", err)
	}

	// Store in keyring using provider name as the user/account identifier
	if err := keyring.Set(serviceName, providerName, string(data)); err != nil {
		return fmt.Errorf("keyring set failed: %w", err)
	}

	return nil
}

// Get retrieves the token from the keyring for the given provider.
func (s *KeyringTokenStore) Get(ctx context.Context, providerName string) (*oauth2.Token, bool, error) {
	if providerName == "" {
		return nil, false, fmt.Errorf("provider name cannot be empty")
	}

	data, err := keyring.Get(serviceName, providerName)
	if err != nil {
		if err == keyring.ErrNotFound {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("keyring get failed: %w", err)
	}

	var token oauth2.Token
	if err := json.Unmarshal([]byte(data), &token); err != nil {
		return nil, false, fmt.Errorf("failed to deserialize token: %w", err)
	}

	return &token, true, nil
}

// Clear removes the token from the keyring for the given provider.
func (s *KeyringTokenStore) Clear(ctx context.Context, providerName string) error {
	if providerName == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	if err := keyring.Delete(serviceName, providerName); err != nil {
		if err == keyring.ErrNotFound {
			return nil
		}
		return fmt.Errorf("keyring delete failed: %w", err)
	}

	return nil
}

// Ensure KeyringTokenStore implements TokenStore
var _ TokenStore = (*KeyringTokenStore)(nil)
