package tokenstore

import (
	"context"
	"fmt"
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

// Set stores the token for the given token key.
func (s *InMemoryTokenStore) Set(ctx context.Context, tokenKey string, token *oauth2.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateTokenKey(tokenKey); err != nil {
		return err
	}
	if token == nil {
		return fmt.Errorf("token cannot be nil")
	}

	s.tokens[tokenKey] = cloneToken(token)
	return nil
}

// Get retrieves the token for the given token key.
func (s *InMemoryTokenStore) Get(ctx context.Context, tokenKey string) (*oauth2.Token, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := validateTokenKey(tokenKey); err != nil {
		return nil, false, err
	}

	token, ok := s.tokens[tokenKey]
	return cloneToken(token), ok, nil
}

// Clear removes the token for the given token key.
func (s *InMemoryTokenStore) Clear(ctx context.Context, tokenKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateTokenKey(tokenKey); err != nil {
		return err
	}

	delete(s.tokens, tokenKey)
	return nil
}

// Snapshot returns a clone of the current in-memory token state.
func (s *InMemoryTokenStore) Snapshot() map[string]*oauth2.Token {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]*oauth2.Token, len(s.tokens))
	for tokenKey, token := range s.tokens {
		snapshot[tokenKey] = cloneToken(token)
	}

	return snapshot
}

// Ensure InMemoryTokenStore implements TokenStore.
var _ TokenStore = (*InMemoryTokenStore)(nil)
