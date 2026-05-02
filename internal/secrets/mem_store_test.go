package secrets

import (
	"context"
	"errors"
	"testing"
)

func TestInMemoryStoreGetSetDelete(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	if err := store.Set(ctx, "account-1", "client_secret", "secret-value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	value, err := store.Get(ctx, "account-1", "client_secret")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if value != "secret-value" {
		t.Fatalf("Get() = %q, want secret-value", value)
	}

	if err := store.Delete(ctx, "account-1", "client_secret"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = store.Get(ctx, "account-1", "client_secret")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("Get() error = %v, want ErrSecretNotFound", err)
	}
}

func TestInMemoryStoreRejectsEmptyKeys(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	cases := []struct {
		name      string
		accountID string
		key       string
	}{
		{"empty accountID", "", "key"},
		{"empty key", "account", ""},
		{"both empty", "", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := store.Set(ctx, c.accountID, c.key, "value"); err == nil {
				t.Fatal("Set() error = nil, want error")
			}
			if _, err := store.Get(ctx, c.accountID, c.key); err == nil {
				t.Fatal("Get() error = nil, want error")
			}
			if err := store.Delete(ctx, c.accountID, c.key); err == nil {
				t.Fatal("Delete() error = nil, want error")
			}
		})
	}
}

func TestInMemoryStoreSnapshot(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_ = store.Set(ctx, "a", "k1", "v1")
	_ = store.Set(ctx, "b", "k2", "v2")

	snap := store.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("len(Snapshot) = %d, want 2", len(snap))
	}
	if snap["a/k1"] != "v1" || snap["b/k2"] != "v2" {
		t.Fatalf("Snapshot() = %v", snap)
	}

	// Snapshot should be a copy
	snap["a/k1"] = "modified"
	v, _ := store.Get(ctx, "a", "k1")
	if v != "v1" {
		t.Fatal("Snapshot was not a copy")
	}
}
