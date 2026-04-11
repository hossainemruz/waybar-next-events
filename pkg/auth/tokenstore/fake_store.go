package tokenstore

import (
	"context"
	"sync"

	"golang.org/x/oauth2"
)

// FakeTokenStore is an in-memory implementation of TokenStore for testing.
// It is safe for concurrent use.
type FakeTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]*oauth2.Token
}

// NewFakeTokenStore creates a new fake token store.
func NewFakeTokenStore() *FakeTokenStore {
	return &FakeTokenStore{
		tokens: make(map[string]*oauth2.Token),
	}
}

// Set stores the token for the given provider.
func (s *FakeTokenStore) Set(ctx context.Context, providerName string, token *oauth2.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens[providerName] = token
	return nil
}

// Get retrieves the token for the given provider.
func (s *FakeTokenStore) Get(ctx context.Context, providerName string) (*oauth2.Token, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	token, ok := s.tokens[providerName]
	return token, ok, nil
}

// Clear removes the token for the given provider.
func (s *FakeTokenStore) Clear(ctx context.Context, providerName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tokens, providerName)
	return nil
}

// Ensure FakeTokenStore implements TokenStore
var _ TokenStore = (*FakeTokenStore)(nil)
