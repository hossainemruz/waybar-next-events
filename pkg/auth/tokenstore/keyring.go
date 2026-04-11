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
// The context can be used to cancel the operation if the keyring service is unresponsive.
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

	// Wrap keyring call in a goroutine to support context cancellation
	type result struct {
		err error
	}
	ch := make(chan result, 1)
	go func() {
		ch <- result{keyring.Set(serviceName, providerName, string(data))}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			return fmt.Errorf("keyring set failed: %w", r.err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Get retrieves the token from the keyring for the given provider.
// The context can be used to cancel the operation if the keyring service is unresponsive.
func (s *KeyringTokenStore) Get(ctx context.Context, providerName string) (*oauth2.Token, bool, error) {
	if providerName == "" {
		return nil, false, fmt.Errorf("provider name cannot be empty")
	}

	// Wrap keyring call in a goroutine to support context cancellation
	type result struct {
		data string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		data, err := keyring.Get(serviceName, providerName)
		ch <- result{data, err}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			if r.err == keyring.ErrNotFound {
				return nil, false, nil
			}
			return nil, false, fmt.Errorf("keyring get failed: %w", r.err)
		}

		var token oauth2.Token
		if err := json.Unmarshal([]byte(r.data), &token); err != nil {
			return nil, false, fmt.Errorf("failed to deserialize token: %w", err)
		}

		return &token, true, nil
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
}

// Clear removes the token from the keyring for the given provider.
// The context can be used to cancel the operation if the keyring service is unresponsive.
func (s *KeyringTokenStore) Clear(ctx context.Context, providerName string) error {
	if providerName == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	// Wrap keyring call in a goroutine to support context cancellation
	type result struct {
		err error
	}
	ch := make(chan result, 1)
	go func() {
		ch <- result{keyring.Delete(serviceName, providerName)}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			if r.err == keyring.ErrNotFound {
				return nil
			}
			return fmt.Errorf("keyring delete failed: %w", r.err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Ensure KeyringTokenStore implements TokenStore
var _ TokenStore = (*KeyringTokenStore)(nil)
