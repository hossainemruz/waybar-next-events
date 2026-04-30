package secrets

import (
	"context"
	"sync"
)

// InMemoryStore is an in-memory secrets store for tests.
type InMemoryStore struct {
	mu      sync.RWMutex
	secrets map[string]string
}

// NewInMemoryStore creates an empty in-memory secrets store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{secrets: make(map[string]string)}
}

// Get returns a stored secret.
func (s *InMemoryStore) Get(ctx context.Context, accountID, key string) (string, error) {
	_ = ctx

	if err := validateSecretRef(accountID, key); err != nil {
		return "", err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.secrets[storageKey(accountID, key)]
	if !ok {
		return "", ErrSecretNotFound
	}

	return value, nil
}

// Set stores a secret.
func (s *InMemoryStore) Set(ctx context.Context, accountID, key, value string) error {
	_ = ctx

	if err := validateSecretRef(accountID, key); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.secrets[storageKey(accountID, key)] = value
	return nil
}

// Delete removes a secret.
func (s *InMemoryStore) Delete(ctx context.Context, accountID, key string) error {
	_ = ctx

	if err := validateSecretRef(accountID, key); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.secrets, storageKey(accountID, key))
	return nil
}

// Snapshot returns a copy of the staged state.
func (s *InMemoryStore) Snapshot() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]string, len(s.secrets))
	for key, value := range s.secrets {
		snapshot[key] = value
	}

	return snapshot
}

var _ Store = (*InMemoryStore)(nil)
