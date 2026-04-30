package tokenstore

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"golang.org/x/oauth2"
)

// StagedTokenStore tracks staged token state in memory and can later commit the
// final staged result to another TokenStore.
type StagedTokenStore struct {
	mu               sync.Mutex
	store            *InMemoryTokenStore
	clearedTokenKeys map[string]struct{}
}

// NewStagedTokenStore creates a staged token store.
func NewStagedTokenStore() *StagedTokenStore {
	return &StagedTokenStore{
		store:            NewInMemoryTokenStore(),
		clearedTokenKeys: make(map[string]struct{}),
	}
}

// Set stages a token write.
func (s *StagedTokenStore) Set(ctx context.Context, tokenKey string, token *oauth2.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.store.Set(ctx, tokenKey, token); err != nil {
		return err
	}

	delete(s.clearedTokenKeys, tokenKey)
	return nil
}

// Get reads the current staged token state.
func (s *StagedTokenStore) Get(ctx context.Context, tokenKey string) (*oauth2.Token, bool, error) {
	return s.store.Get(ctx, tokenKey)
}

// Clear stages a token removal.
func (s *StagedTokenStore) Clear(ctx context.Context, tokenKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.store.Clear(ctx, tokenKey); err != nil {
		return err
	}

	s.clearedTokenKeys[tokenKey] = struct{}{}
	return nil
}

// Commit applies the final staged token state to the backing store.
func (s *StagedTokenStore) Commit(ctx context.Context, backing TokenStore) error {
	s.mu.Lock()
	stagedTokens := s.store.Snapshot()
	clearedTokenKeys := make(map[string]struct{}, len(s.clearedTokenKeys))
	for tokenKey := range s.clearedTokenKeys {
		clearedTokenKeys[tokenKey] = struct{}{}
	}
	s.mu.Unlock()

	stagedTokenKeys := sortedTokenKeys(stagedTokens)
	clearedTokenKeyList := sortedClearedTokenKeys(clearedTokenKeys)
	touchedTokenKeys := mergeTokenKeys(stagedTokenKeys, clearedTokenKeyList)

	originalTokens, err := snapshotBackingTokens(ctx, backing, touchedTokenKeys)
	if err != nil {
		return err
	}

	for _, tokenKey := range stagedTokenKeys {
		if err := backing.Set(ctx, tokenKey, stagedTokens[tokenKey]); err != nil {
			return rollbackCommit(backing, originalTokens, touchedTokenKeys, err)
		}
	}

	for _, tokenKey := range clearedTokenKeyList {
		if err := backing.Clear(ctx, tokenKey); err != nil {
			return rollbackCommit(backing, originalTokens, touchedTokenKeys, err)
		}
	}

	return nil
}

type storedTokenSnapshot struct {
	token *oauth2.Token
	found bool
}

func snapshotBackingTokens(ctx context.Context, backing TokenStore, tokenKeys []string) (map[string]storedTokenSnapshot, error) {
	snapshot := make(map[string]storedTokenSnapshot, len(tokenKeys))
	for _, tokenKey := range tokenKeys {
		token, found, err := backing.Get(ctx, tokenKey)
		if err != nil {
			return nil, fmt.Errorf("failed to snapshot token for key %q: %w", tokenKey, err)
		}
		snapshot[tokenKey] = storedTokenSnapshot{
			token: cloneToken(token),
			found: found,
		}
	}

	return snapshot, nil
}

func rollbackCommit(backing TokenStore, snapshot map[string]storedTokenSnapshot, tokenKeys []string, commitErr error) error {
	rollbackCtx := context.Background()
	for _, tokenKey := range tokenKeys {
		stored := snapshot[tokenKey]
		if stored.found {
			if err := backing.Set(rollbackCtx, tokenKey, stored.token); err != nil {
				return errors.Join(commitErr, fmt.Errorf("failed to rollback token for key %q: %w", tokenKey, err))
			}
			continue
		}

		if err := backing.Clear(rollbackCtx, tokenKey); err != nil {
			return errors.Join(commitErr, fmt.Errorf("failed to rollback token clear for key %q: %w", tokenKey, err))
		}
	}

	return commitErr
}

func sortedTokenKeys(tokens map[string]*oauth2.Token) []string {
	tokenKeys := make([]string, 0, len(tokens))
	for tokenKey := range tokens {
		tokenKeys = append(tokenKeys, tokenKey)
	}
	sort.Strings(tokenKeys)
	return tokenKeys
}

func sortedClearedTokenKeys(clearedTokenKeys map[string]struct{}) []string {
	tokenKeys := make([]string, 0, len(clearedTokenKeys))
	for tokenKey := range clearedTokenKeys {
		tokenKeys = append(tokenKeys, tokenKey)
	}
	sort.Strings(tokenKeys)
	return tokenKeys
}

func mergeTokenKeys(first []string, second []string) []string {
	seen := make(map[string]struct{}, len(first)+len(second))
	tokenKeys := make([]string, 0, len(first)+len(second))
	for _, tokenKey := range first {
		if _, ok := seen[tokenKey]; ok {
			continue
		}
		seen[tokenKey] = struct{}{}
		tokenKeys = append(tokenKeys, tokenKey)
	}
	for _, tokenKey := range second {
		if _, ok := seen[tokenKey]; ok {
			continue
		}
		seen[tokenKey] = struct{}{}
		tokenKeys = append(tokenKeys, tokenKey)
	}
	return tokenKeys
}

func cloneToken(token *oauth2.Token) *oauth2.Token {
	if token == nil {
		return nil
	}

	cloned := *token
	return &cloned
}

// Ensure StagedTokenStore implements TokenStore.
var _ TokenStore = (*StagedTokenStore)(nil)
