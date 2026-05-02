package google

import (
	"context"
	"testing"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
)

func TestService_Provider(t *testing.T) {
	srv := NewService()
	ctx := context.Background()

	t.Run("empty account id", func(t *testing.T) {
		account := calendar.Account{ID: "   "}
		store := secrets.NewInMemoryStore()
		_, err := srv.Provider(ctx, account, store)
		if err == nil {
			t.Fatal("expected error for empty account ID")
		}
	})

	t.Run("missing client_id", func(t *testing.T) {
		account := calendar.Account{
			ID:       "acc-1",
			Settings: map[string]string{},
		}
		store := secrets.NewInMemoryStore()
		if err := store.Set(ctx, account.ID, clientSecretKey, "my-secret"); err != nil {
			t.Fatalf("setup secret: %v", err)
		}
		_, err := srv.Provider(ctx, account, store)
		if err == nil {
			t.Fatal("expected error for missing client id")
		}
	})

	t.Run("missing client_secret", func(t *testing.T) {
		account := calendar.Account{
			ID:       "acc-1",
			Settings: map[string]string{clientIDKey: "my-client-id"},
		}
		store := secrets.NewInMemoryStore()
		_, err := srv.Provider(ctx, account, store)
		if err == nil {
			t.Fatal("expected error for missing client secret")
		}
	})

	t.Run("valid account returns google provider", func(t *testing.T) {
		account := calendar.Account{
			ID:       "acc-1",
			Settings: map[string]string{clientIDKey: "my-client-id"},
		}
		store := secrets.NewInMemoryStore()
		if err := store.Set(ctx, account.ID, clientSecretKey, "my-secret"); err != nil {
			t.Fatalf("setup secret: %v", err)
		}

		provider, err := srv.Provider(ctx, account, store)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if provider.ClientID() != "my-client-id" {
			t.Errorf("client ID mismatch: got %q, want %q", provider.ClientID(), "my-client-id")
		}
		if provider.ClientSecret() != "my-secret" {
			t.Errorf("client secret mismatch: got %q, want %q", provider.ClientSecret(), "my-secret")
		}
		if provider.Name() != "google/acc-1" {
			t.Errorf("name mismatch: got %q, want %q", provider.Name(), "google/acc-1")
		}
	})
}
