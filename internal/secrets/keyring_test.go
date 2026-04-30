package secrets

import (
	"context"
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestKeyringStoreUsesKeyringMock(t *testing.T) {
	keyring.MockInit()
	store := NewKeyringStore()
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

func TestKeyringStorePropagatesMockErrors(t *testing.T) {
	keyring.MockInitWithError(errors.New("keyring unavailable"))
	store := NewKeyringStore()

	err := store.Set(context.Background(), "account-1", "client_secret", "secret-value")
	if err == nil || err.Error() == "" {
		t.Fatal("Set() error = nil, want keyring failure")
	}
}
