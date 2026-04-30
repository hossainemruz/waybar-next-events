package tokenstore

import (
	"context"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestInMemoryTokenStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryTokenStore()

	t.Run("TokenKey", func(t *testing.T) {
		if got := TokenKey("google", "account-id"); got != "google/account-id" {
			t.Fatalf("TokenKey() = %q, want %q", got, "google/account-id")
		}
	})

	t.Run("Get_NotFound", func(t *testing.T) {
		token, found, err := store.Get(ctx, "nonexistent")
		if err != nil {
			t.Errorf("Get() error = %v", err)
		}
		if found {
			t.Error("Get() found = true, want false")
		}
		if token != nil {
			t.Error("Get() token != nil, want nil")
		}
	})

	t.Run("SetAndGet", func(t *testing.T) {
		token := &oauth2.Token{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			TokenType:    "Bearer",
			Expiry:       time.Now().Add(time.Hour),
		}

		err := store.Set(ctx, "test-provider", token)
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		retrieved, found, err := store.Get(ctx, "test-provider")
		if err != nil {
			t.Errorf("Get() error = %v", err)
		}
		if !found {
			t.Error("Get() found = false, want true")
		}
		if retrieved == nil {
			t.Fatal("Get() token = nil, want non-nil")
		}
		if retrieved.AccessToken != token.AccessToken {
			t.Errorf("Get() AccessToken = %s, want %s", retrieved.AccessToken, token.AccessToken)
		}
		if retrieved.RefreshToken != token.RefreshToken {
			t.Errorf("Get() RefreshToken = %s, want %s", retrieved.RefreshToken, token.RefreshToken)
		}
	})

	t.Run("Set_NilToken", func(t *testing.T) {
		err := store.Set(ctx, "nil-token-provider", nil)
		if err == nil {
			t.Fatal("Set() error = nil, want error")
		}
	})

	t.Run("Set_EmptyKey", func(t *testing.T) {
		err := store.Set(ctx, "", &oauth2.Token{AccessToken: "token"})
		if err == nil {
			t.Fatal("Set() error = nil, want error")
		}
	})

	t.Run("Get_EmptyKey", func(t *testing.T) {
		_, _, err := store.Get(ctx, "")
		if err == nil {
			t.Fatal("Get() error = nil, want error")
		}
	})

	t.Run("Clear_EmptyKey", func(t *testing.T) {
		err := store.Clear(ctx, "")
		if err == nil {
			t.Fatal("Clear() error = nil, want error")
		}
	})

	t.Run("Set_UpdatesExisting", func(t *testing.T) {
		token1 := &oauth2.Token{
			AccessToken: "token1",
			Expiry:      time.Now().Add(time.Hour),
		}
		token2 := &oauth2.Token{
			AccessToken: "token2",
			Expiry:      time.Now().Add(2 * time.Hour),
		}

		if err := store.Set(ctx, "update-provider", token1); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := store.Set(ctx, "update-provider", token2); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		retrieved, found, _ := store.Get(ctx, "update-provider")
		if !found {
			t.Fatal("Get() found = false after update")
		}
		if retrieved.AccessToken != "token2" {
			t.Errorf("Get() AccessToken = %s, want token2", retrieved.AccessToken)
		}
	})

	t.Run("Clear_Existing", func(t *testing.T) {
		token := &oauth2.Token{
			AccessToken: "to-be-cleared",
			Expiry:      time.Now().Add(time.Hour),
		}

		if err := store.Set(ctx, "clear-provider", token); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		err := store.Clear(ctx, "clear-provider")
		if err != nil {
			t.Errorf("Clear() error = %v", err)
		}

		_, found, _ := store.Get(ctx, "clear-provider")
		if found {
			t.Error("Get() found = true after Clear(), want false")
		}
	})

	t.Run("Clear_NotFound", func(t *testing.T) {
		// Clearing a non-existent token should not error
		err := store.Clear(ctx, "never-existed")
		if err != nil {
			t.Errorf("Clear() error = %v, want nil", err)
		}
	})

	t.Run("MultipleProviders", func(t *testing.T) {
		googleToken := &oauth2.Token{
			AccessToken: "google-token",
			Expiry:      time.Now().Add(time.Hour),
		}
		githubToken := &oauth2.Token{
			AccessToken: "github-token",
			Expiry:      time.Now().Add(time.Hour),
		}

		if err := store.Set(ctx, "google", googleToken); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if err := store.Set(ctx, "github", githubToken); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		g, found, _ := store.Get(ctx, "google")
		if !found || g.AccessToken != "google-token" {
			t.Error("Google token not stored correctly")
		}

		h, found, _ := store.Get(ctx, "github")
		if !found || h.AccessToken != "github-token" {
			t.Error("GitHub token not stored correctly")
		}

		// Ensure they're isolated
		if err := store.Clear(ctx, "google"); err != nil {
			t.Errorf("Clear() error = %v", err)
		}
		_, found, _ = store.Get(ctx, "google")
		if found {
			t.Error("Google token still exists after clear")
		}

		h, found, _ = store.Get(ctx, "github")
		if !found || h.AccessToken != "github-token" {
			t.Error("GitHub token affected by Google clear")
		}
	})
}
