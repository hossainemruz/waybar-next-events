package tokenstore

import (
	"context"
	"sync"

	"golang.org/x/oauth2"
)

// InMemoryTokenStore is an in-memory implementation of TokenStore.
// It is safe for concurrent use.
type InMemoryTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]*oauth2.Token
}

// NewInMemoryTokenStore creates a new in-memory token store.
func NewInMemoryTokenStore() *InMemoryTokenStore {
	return &InMemoryTokenStore{
		tokens: make(map[string]*oauth2.Token),
	}
}

// Set stores the token for the given provider.
func (s *InMemoryTokenStore) Set(ctx context.Context, providerName string, token *oauth2.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens[providerName] = cloneToken(token)
	return nil
}

// Get retrieves the token for the given provider.
func (s *InMemoryTokenStore) Get(ctx context.Context, providerName string) (*oauth2.Token, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	token, ok := s.tokens[providerName]
	return cloneToken(token), ok, nil
}

// Clear removes the token for the given provider.
func (s *InMemoryTokenStore) Clear(ctx context.Context, providerName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tokens, providerName)
	return nil
}

// Snapshot returns a clone of the current in-memory token state.
func (s *InMemoryTokenStore) Snapshot() map[string]*oauth2.Token {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]*oauth2.Token, len(s.tokens))
	for providerName, token := range s.tokens {
		snapshot[providerName] = cloneToken(token)
	}

	return snapshot
}

// Ensure InMemoryTokenStore implements TokenStore.
var _ TokenStore = (*InMemoryTokenStore)(nil)
