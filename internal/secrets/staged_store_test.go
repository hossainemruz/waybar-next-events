package secrets

import (
	"context"
	"errors"
	"testing"
)

func TestStagedStoreCommitAndDiscard(t *testing.T) {
	ctx := context.Background()
	backing := NewInMemoryStore()
	staged := NewStagedStore()

	if err := staged.Set(ctx, "account-1", "client_secret", "secret-value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if _, err := backing.Get(ctx, "account-1", "client_secret"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("backing Get() error = %v, want ErrSecretNotFound before commit", err)
	}

	staged.Discard()
	if _, err := staged.Get(ctx, "account-1", "client_secret"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("staged Get() after Discard error = %v, want ErrSecretNotFound", err)
	}

	if err := staged.Set(ctx, "account-1", "client_secret", "secret-value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := staged.Commit(ctx, backing); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	value, err := backing.Get(ctx, "account-1", "client_secret")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if value != "secret-value" {
		t.Fatalf("Get() = %q, want secret-value", value)
	}
}

func TestStagedStoreCommitDelete(t *testing.T) {
	ctx := context.Background()
	backing := NewInMemoryStore()
	if err := backing.Set(ctx, "account-1", "client_secret", "secret-value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	staged := NewStagedStore()
	if err := staged.Delete(ctx, "account-1", "client_secret"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if err := staged.Commit(ctx, backing); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	_, err := backing.Get(ctx, "account-1", "client_secret")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("Get() error = %v, want ErrSecretNotFound", err)
	}
}

func TestStagedStoreCommitRollsBackOnFailure(t *testing.T) {
	ctx := context.Background()
	backing := &failingStore{store: NewInMemoryStore(), setErr: errors.New("keyring unavailable")}
	if err := backing.store.Set(ctx, "account-1", "client_secret", "original"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	staged := NewStagedStore()
	if err := staged.Set(ctx, "account-1", "client_secret", "updated"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	err := staged.Commit(ctx, backing)
	if err == nil {
		t.Fatal("Commit() error = nil, want failure")
	}

	value, getErr := backing.store.Get(ctx, "account-1", "client_secret")
	if getErr != nil {
		t.Fatalf("Get() error = %v", getErr)
	}
	if value != "original" {
		t.Fatalf("Get() = %q, want original", value)
	}
}

func TestStagedStoreCommitSetAfterDelete(t *testing.T) {
	ctx := context.Background()
	backing := NewInMemoryStore()
	if err := backing.Set(ctx, "account-1", "client_secret", "original"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	staged := NewStagedStore()
	if err := staged.Delete(ctx, "account-1", "client_secret"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := staged.Set(ctx, "account-1", "client_secret", "updated"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if err := staged.Commit(ctx, backing); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	value, err := backing.Get(ctx, "account-1", "client_secret")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if value != "updated" {
		t.Fatalf("Get() = %q, want updated", value)
	}
}

func TestStagedStoreCommitMultipleKeys(t *testing.T) {
	ctx := context.Background()
	backing := NewInMemoryStore()
	if err := backing.Set(ctx, "account-1", "old_secret", "remove-me"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	staged := NewStagedStore()
	if err := staged.Set(ctx, "account-1", "client_secret", "secret-value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := staged.Set(ctx, "account-2", "api_key", "api-value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := staged.Delete(ctx, "account-1", "old_secret"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if err := staged.Commit(ctx, backing); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	value, err := backing.Get(ctx, "account-1", "client_secret")
	if err != nil || value != "secret-value" {
		t.Fatalf("Get(account-1/client_secret) = %q, err=%v", value, err)
	}

	value, err = backing.Get(ctx, "account-2", "api_key")
	if err != nil || value != "api-value" {
		t.Fatalf("Get(account-2/api_key) = %q, err=%v", value, err)
	}

	_, err = backing.Get(ctx, "account-1", "old_secret")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("Get(account-1/old_secret) error = %v, want ErrSecretNotFound", err)
	}
}

func TestSortedSecretRefsRejectsInvalidKeys(t *testing.T) {
	_, err := sortedSecretRefs(map[string]string{"invalid": "value"})
	if err == nil {
		t.Fatal("sortedSecretRefs() error = nil, want invalid key error")
	}
}

type failingStore struct {
	store     *InMemoryStore
	setErr    error
	deleteErr error
}

func (s *failingStore) Get(ctx context.Context, accountID, key string) (string, error) {
	return s.store.Get(ctx, accountID, key)
}

func (s *failingStore) Set(ctx context.Context, accountID, key, value string) error {
	if s.setErr != nil {
		return s.setErr
	}
	return s.store.Set(ctx, accountID, key, value)
}

func (s *failingStore) Delete(ctx context.Context, accountID, key string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	return s.store.Delete(ctx, accountID, key)
}
