package secrets

import (
	"context"
	"fmt"

	"github.com/zalando/go-keyring"
)

const serviceName = "waybar-next-events"

// KeyringStore stores secrets in the OS keyring.
type KeyringStore struct{}

// NewKeyringStore creates a keyring-backed secrets store.
func NewKeyringStore() *KeyringStore {
	return &KeyringStore{}
}

// Get returns a stored secret.
func (s *KeyringStore) Get(ctx context.Context, accountID, key string) (string, error) {
	if err := validateSecretRef(accountID, key); err != nil {
		return "", err
	}

	type result struct {
		value string
		err   error
	}

	// The underlying keyring call can block on OS integration, so run it in a
	// goroutine and race the result against ctx.Done().
	ch := make(chan result, 1)
	go func() {
		value, err := keyring.Get(serviceName, storageKey(accountID, key))
		ch <- result{value: value, err: err}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			if r.err == keyring.ErrNotFound {
				return "", ErrSecretNotFound
			}
			return "", fmt.Errorf("keyring get failed: %w", r.err)
		}
		return r.value, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Set stores a secret.
func (s *KeyringStore) Set(ctx context.Context, accountID, key, value string) error {
	if err := validateSecretRef(accountID, key); err != nil {
		return err
	}

	type result struct{ err error }

	// The underlying keyring call can block on OS integration, so run it in a
	// goroutine and race the result against ctx.Done().
	ch := make(chan result, 1)
	go func() {
		ch <- result{err: keyring.Set(serviceName, storageKey(accountID, key), value)}
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

// Delete removes a secret.
func (s *KeyringStore) Delete(ctx context.Context, accountID, key string) error {
	if err := validateSecretRef(accountID, key); err != nil {
		return err
	}

	type result struct{ err error }

	// The underlying keyring call can block on OS integration, so run it in a
	// goroutine and race the result against ctx.Done().
	ch := make(chan result, 1)
	go func() {
		ch <- result{err: keyring.Delete(serviceName, storageKey(accountID, key))}
	}()

	select {
	case r := <-ch:
		if r.err != nil && r.err != keyring.ErrNotFound {
			return fmt.Errorf("keyring delete failed: %w", r.err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var _ Store = (*KeyringStore)(nil)
