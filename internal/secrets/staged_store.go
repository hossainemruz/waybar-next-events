package secrets

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// StagedStore accumulates secret writes until Commit succeeds.
type StagedStore struct {
	mu          sync.Mutex
	store       *InMemoryStore
	deletedKeys map[string]struct{}
}

// NewStagedStore creates an empty staged secrets store.
func NewStagedStore() *StagedStore {
	return &StagedStore{
		store:       NewInMemoryStore(),
		deletedKeys: make(map[string]struct{}),
	}
}

// Get reads the current staged value.
func (s *StagedStore) Get(ctx context.Context, accountID, key string) (string, error) {
	storage := storageKey(accountID, key)

	s.mu.Lock()
	store := s.store
	_, deleted := s.deletedKeys[storage]
	s.mu.Unlock()
	if deleted {
		return "", ErrSecretNotFound
	}

	return store.Get(ctx, accountID, key)
}

// Set stages a secret write.
func (s *StagedStore) Set(ctx context.Context, accountID, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.store.Set(ctx, accountID, key, value); err != nil {
		return err
	}

	delete(s.deletedKeys, storageKey(accountID, key))
	return nil
}

// Delete stages a secret removal.
func (s *StagedStore) Delete(ctx context.Context, accountID, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.store.Delete(ctx, accountID, key); err != nil {
		return err
	}

	s.deletedKeys[storageKey(accountID, key)] = struct{}{}
	return nil
}

// Discard drops all staged changes.
func (s *StagedStore) Discard() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.store = NewInMemoryStore()
	s.deletedKeys = make(map[string]struct{})
}

// Commit applies staged changes to a backing store.
func (s *StagedStore) Commit(ctx context.Context, backing Store) error {
	s.mu.Lock()
	stagedValues := s.store.Snapshot()
	deletedKeys := make(map[string]struct{}, len(s.deletedKeys))
	for key := range s.deletedKeys {
		deletedKeys[key] = struct{}{}
	}
	s.mu.Unlock()

	stagedRefs, err := sortedSecretRefs(stagedValues)
	if err != nil {
		return err
	}
	deletedRefs, err := sortedDeletedSecretRefs(deletedKeys)
	if err != nil {
		return err
	}
	allRefs := mergeSecretRefs(stagedRefs, deletedRefs)

	snapshot, err := snapshotBackingSecrets(ctx, backing, allRefs)
	if err != nil {
		return err
	}

	for _, ref := range stagedRefs {
		if err := backing.Set(ctx, ref.accountID, ref.key, stagedValues[storageKey(ref.accountID, ref.key)]); err != nil {
			return rollbackSecretCommit(backing, snapshot, allRefs, err)
		}
	}

	for _, ref := range deletedRefs {
		if err := backing.Delete(ctx, ref.accountID, ref.key); err != nil {
			return rollbackSecretCommit(backing, snapshot, allRefs, err)
		}
	}

	return nil
}

type secretRef struct {
	accountID string
	key       string
}

type storedSecretSnapshot struct {
	value string
	found bool
}

func sortedSecretRefs(values map[string]string) ([]secretRef, error) {
	refs := make([]secretRef, 0, len(values))
	for storage := range values {
		accountID, key, ok := splitStorageKey(storage)
		if !ok {
			return nil, fmt.Errorf("invalid staged secret key %q", storage)
		}
		refs = append(refs, secretRef{accountID: accountID, key: key})
	}

	sort.Slice(refs, func(i, j int) bool {
		if refs[i].accountID == refs[j].accountID {
			return refs[i].key < refs[j].key
		}
		return refs[i].accountID < refs[j].accountID
	})

	return refs, nil
}

func sortedDeletedSecretRefs(deletedKeys map[string]struct{}) ([]secretRef, error) {
	refs := make([]secretRef, 0, len(deletedKeys))
	for storage := range deletedKeys {
		accountID, key, ok := splitStorageKey(storage)
		if !ok {
			return nil, fmt.Errorf("invalid staged secret key %q", storage)
		}
		refs = append(refs, secretRef{accountID: accountID, key: key})
	}

	sort.Slice(refs, func(i, j int) bool {
		if refs[i].accountID == refs[j].accountID {
			return refs[i].key < refs[j].key
		}
		return refs[i].accountID < refs[j].accountID
	})

	return refs, nil
}

func mergeSecretRefs(preferred []secretRef, fallback []secretRef) []secretRef {
	seen := make(map[string]struct{}, len(preferred)+len(fallback))
	refs := make([]secretRef, 0, len(preferred)+len(fallback))

	for _, ref := range append(preferred, fallback...) {
		storage := storageKey(ref.accountID, ref.key)
		if _, ok := seen[storage]; ok {
			continue
		}
		seen[storage] = struct{}{}
		refs = append(refs, ref)
	}

	return refs
}

func snapshotBackingSecrets(ctx context.Context, backing Store, refs []secretRef) (map[string]storedSecretSnapshot, error) {
	snapshot := make(map[string]storedSecretSnapshot, len(refs))
	for _, ref := range refs {
		value, err := backing.Get(ctx, ref.accountID, ref.key)
		if err != nil {
			if errors.Is(err, ErrSecretNotFound) {
				snapshot[storageKey(ref.accountID, ref.key)] = storedSecretSnapshot{found: false}
				continue
			}
			return nil, fmt.Errorf("failed to snapshot secret %q for account %q: %w", ref.key, ref.accountID, err)
		}

		snapshot[storageKey(ref.accountID, ref.key)] = storedSecretSnapshot{value: value, found: true}
	}

	return snapshot, nil
}

func rollbackSecretCommit(backing Store, snapshot map[string]storedSecretSnapshot, refs []secretRef, commitErr error) error {
	rollbackCtx := context.Background()
	for _, ref := range refs {
		stored := snapshot[storageKey(ref.accountID, ref.key)]
		if stored.found {
			if err := backing.Set(rollbackCtx, ref.accountID, ref.key, stored.value); err != nil {
				return errors.Join(commitErr, fmt.Errorf("failed to rollback secret %q for account %q: %w", ref.key, ref.accountID, err))
			}
			continue
		}

		if err := backing.Delete(rollbackCtx, ref.accountID, ref.key); err != nil {
			return errors.Join(commitErr, fmt.Errorf("failed to rollback secret deletion for account %q: %w", ref.accountID, err))
		}
	}

	return commitErr
}

func splitStorageKey(value string) (accountID, key string, ok bool) {
	for i := 0; i < len(value); i++ {
		if value[i] != '/' {
			continue
		}
		return value[:i], value[i+1:], true
	}

	return "", "", false
}

var _ Store = (*StagedStore)(nil)
