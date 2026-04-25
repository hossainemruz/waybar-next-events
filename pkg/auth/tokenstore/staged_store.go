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
	clearedProviders map[string]struct{}
}

// NewStagedTokenStore creates a staged token store.
func NewStagedTokenStore() *StagedTokenStore {
	return &StagedTokenStore{
		store:            NewInMemoryTokenStore(),
		clearedProviders: make(map[string]struct{}),
	}
}

// Set stages a token write.
func (s *StagedTokenStore) Set(ctx context.Context, providerName string, token *oauth2.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.store.Set(ctx, providerName, token); err != nil {
		return err
	}

	delete(s.clearedProviders, providerName)
	return nil
}

// Get reads the current staged token state.
func (s *StagedTokenStore) Get(ctx context.Context, providerName string) (*oauth2.Token, bool, error) {
	return s.store.Get(ctx, providerName)
}

// Clear stages a token removal.
func (s *StagedTokenStore) Clear(ctx context.Context, providerName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.store.Clear(ctx, providerName); err != nil {
		return err
	}

	s.clearedProviders[providerName] = struct{}{}
	return nil
}

// Commit applies the final staged token state to the backing store.
func (s *StagedTokenStore) Commit(ctx context.Context, backing TokenStore) error {
	s.mu.Lock()
	stagedTokens := s.store.Snapshot()
	clearedProviders := make(map[string]struct{}, len(s.clearedProviders))
	for providerName := range s.clearedProviders {
		clearedProviders[providerName] = struct{}{}
	}
	s.mu.Unlock()

	stagedProviderNames := sortedTokenProviders(stagedTokens)
	clearedProviderNames := sortedClearedProviders(clearedProviders)
	touchedProviderNames := mergeProviderNames(stagedProviderNames, clearedProviderNames)

	originalTokens, err := snapshotBackingTokens(ctx, backing, touchedProviderNames)
	if err != nil {
		return err
	}

	for _, providerName := range stagedProviderNames {
		if err := backing.Set(ctx, providerName, stagedTokens[providerName]); err != nil {
			return rollbackCommit(backing, originalTokens, touchedProviderNames, err)
		}
	}

	for _, providerName := range clearedProviderNames {
		if err := backing.Clear(ctx, providerName); err != nil {
			return rollbackCommit(backing, originalTokens, touchedProviderNames, err)
		}
	}

	return nil
}

type storedTokenSnapshot struct {
	token *oauth2.Token
	found bool
}

func snapshotBackingTokens(ctx context.Context, backing TokenStore, providerNames []string) (map[string]storedTokenSnapshot, error) {
	snapshot := make(map[string]storedTokenSnapshot, len(providerNames))
	for _, providerName := range providerNames {
		token, found, err := backing.Get(ctx, providerName)
		if err != nil {
			return nil, fmt.Errorf("failed to snapshot token for provider %q: %w", providerName, err)
		}
		snapshot[providerName] = storedTokenSnapshot{
			token: cloneToken(token),
			found: found,
		}
	}

	return snapshot, nil
}

func rollbackCommit(backing TokenStore, snapshot map[string]storedTokenSnapshot, providerNames []string, commitErr error) error {
	rollbackCtx := context.Background()
	for _, providerName := range providerNames {
		stored := snapshot[providerName]
		if stored.found {
			if err := backing.Set(rollbackCtx, providerName, stored.token); err != nil {
				return errors.Join(commitErr, fmt.Errorf("failed to rollback token for provider %q: %w", providerName, err))
			}
			continue
		}

		if err := backing.Clear(rollbackCtx, providerName); err != nil {
			return errors.Join(commitErr, fmt.Errorf("failed to rollback token clear for provider %q: %w", providerName, err))
		}
	}

	return commitErr
}

func sortedTokenProviders(tokens map[string]*oauth2.Token) []string {
	providerNames := make([]string, 0, len(tokens))
	for providerName := range tokens {
		providerNames = append(providerNames, providerName)
	}
	sort.Strings(providerNames)
	return providerNames
}

func sortedClearedProviders(clearedProviders map[string]struct{}) []string {
	providerNames := make([]string, 0, len(clearedProviders))
	for providerName := range clearedProviders {
		providerNames = append(providerNames, providerName)
	}
	sort.Strings(providerNames)
	return providerNames
}

func mergeProviderNames(first []string, second []string) []string {
	seen := make(map[string]struct{}, len(first)+len(second))
	providerNames := make([]string, 0, len(first)+len(second))
	for _, providerName := range first {
		if _, ok := seen[providerName]; ok {
			continue
		}
		seen[providerName] = struct{}{}
		providerNames = append(providerNames, providerName)
	}
	for _, providerName := range second {
		if _, ok := seen[providerName]; ok {
			continue
		}
		seen[providerName] = struct{}{}
		providerNames = append(providerNames, providerName)
	}
	return providerNames
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
