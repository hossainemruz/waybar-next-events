package tokenstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestStagedTokenStoreGetReturnsStagedToken(t *testing.T) {
	ctx := context.Background()
	store := NewStagedTokenStore()
	stagedToken := &oauth2.Token{
		AccessToken: "staged-access-token",
		Expiry:      time.Now().Add(time.Hour),
	}

	if err := store.Set(ctx, "provider", stagedToken); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, found, err := store.Get(ctx, "provider")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !found {
		t.Fatal("Get() found = false, want true")
	}
	if got == nil {
		t.Fatal("Get() token = nil, want non-nil")
	}
	if got.AccessToken != stagedToken.AccessToken {
		t.Fatalf("Get() AccessToken = %q, want %q", got.AccessToken, stagedToken.AccessToken)
	}
}

func TestStagedTokenStoreCommitAppliesFinalState(t *testing.T) {
	ctx := context.Background()
	stagedStore := NewStagedTokenStore()
	backingStore := NewInMemoryTokenStore()

	if err := backingStore.Set(ctx, "clear-provider", &oauth2.Token{AccessToken: "old-clear-token"}); err != nil {
		t.Fatalf("backingStore.Set() clear-provider error = %v", err)
	}
	if err := backingStore.Set(ctx, "update-provider", &oauth2.Token{AccessToken: "old-update-token"}); err != nil {
		t.Fatalf("backingStore.Set() update-provider error = %v", err)
	}

	if err := stagedStore.Set(ctx, "update-provider", &oauth2.Token{AccessToken: "new-update-token"}); err != nil {
		t.Fatalf("Set() update-provider error = %v", err)
	}
	if err := stagedStore.Set(ctx, "set-then-clear-provider", &oauth2.Token{AccessToken: "temporary-token"}); err != nil {
		t.Fatalf("Set() set-then-clear-provider error = %v", err)
	}
	if err := stagedStore.Clear(ctx, "set-then-clear-provider"); err != nil {
		t.Fatalf("Clear() set-then-clear-provider error = %v", err)
	}
	if err := stagedStore.Clear(ctx, "clear-provider"); err != nil {
		t.Fatalf("Clear() clear-provider error = %v", err)
	}
	if err := stagedStore.Set(ctx, "new-provider", &oauth2.Token{AccessToken: "new-token"}); err != nil {
		t.Fatalf("Set() new-provider error = %v", err)
	}

	if err := stagedStore.Commit(ctx, backingStore); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	if _, found, err := backingStore.Get(ctx, "clear-provider"); err != nil {
		t.Fatalf("backingStore.Get() clear-provider error = %v", err)
	} else if found {
		t.Fatal("clear-provider still exists after Commit()")
	}

	if _, found, err := backingStore.Get(ctx, "set-then-clear-provider"); err != nil {
		t.Fatalf("backingStore.Get() set-then-clear-provider error = %v", err)
	} else if found {
		t.Fatal("set-then-clear-provider still exists after Commit()")
	}

	updated, found, err := backingStore.Get(ctx, "update-provider")
	if err != nil {
		t.Fatalf("backingStore.Get() update-provider error = %v", err)
	}
	if !found {
		t.Fatal("update-provider missing after Commit()")
	}
	if updated.AccessToken != "new-update-token" {
		t.Fatalf("update-provider AccessToken = %q, want %q", updated.AccessToken, "new-update-token")
	}

	created, found, err := backingStore.Get(ctx, "new-provider")
	if err != nil {
		t.Fatalf("backingStore.Get() new-provider error = %v", err)
	}
	if !found {
		t.Fatal("new-provider missing after Commit()")
	}
	if created.AccessToken != "new-token" {
		t.Fatalf("new-provider AccessToken = %q, want %q", created.AccessToken, "new-token")
	}
}

func TestStagedTokenStoreCommitRollsBackWhenSetFails(t *testing.T) {
	ctx := context.Background()
	stagedStore := NewStagedTokenStore()
	backingStore := &failingCommitTokenStore{
		TokenStore: NewInMemoryTokenStore(),
		setErrors: map[string]error{
			"new-provider": errors.New("set failed"),
		},
	}

	if err := backingStore.Set(ctx, "clear-provider", &oauth2.Token{AccessToken: "old-clear-token"}); err != nil {
		t.Fatalf("backingStore.Set() clear-provider error = %v", err)
	}
	if err := backingStore.Set(ctx, "update-provider", &oauth2.Token{AccessToken: "old-update-token"}); err != nil {
		t.Fatalf("backingStore.Set() update-provider error = %v", err)
	}

	if err := stagedStore.Clear(ctx, "clear-provider"); err != nil {
		t.Fatalf("Clear() clear-provider error = %v", err)
	}
	if err := stagedStore.Set(ctx, "update-provider", &oauth2.Token{AccessToken: "new-update-token"}); err != nil {
		t.Fatalf("Set() update-provider error = %v", err)
	}
	if err := stagedStore.Set(ctx, "new-provider", &oauth2.Token{AccessToken: "new-token"}); err != nil {
		t.Fatalf("Set() new-provider error = %v", err)
	}

	err := stagedStore.Commit(ctx, backingStore)
	if err == nil {
		t.Fatal("Commit() error = nil, want error")
	}
	if !errors.Is(err, backingStore.setErrors["new-provider"]) {
		t.Fatalf("Commit() error = %v, want set failure", err)
	}

	cleared, found, err := backingStore.Get(ctx, "clear-provider")
	if err != nil {
		t.Fatalf("backingStore.Get() clear-provider error = %v", err)
	}
	if !found {
		t.Fatal("clear-provider missing after rollback")
	}
	if cleared.AccessToken != "old-clear-token" {
		t.Fatalf("clear-provider AccessToken = %q, want %q", cleared.AccessToken, "old-clear-token")
	}

	updated, found, err := backingStore.Get(ctx, "update-provider")
	if err != nil {
		t.Fatalf("backingStore.Get() update-provider error = %v", err)
	}
	if !found {
		t.Fatal("update-provider missing after rollback")
	}
	if updated.AccessToken != "old-update-token" {
		t.Fatalf("update-provider AccessToken = %q, want %q", updated.AccessToken, "old-update-token")
	}

	if _, found, err := backingStore.Get(ctx, "new-provider"); err != nil {
		t.Fatalf("backingStore.Get() new-provider error = %v", err)
	} else if found {
		t.Fatal("new-provider should not exist after rollback")
	}
}

func TestStagedTokenStoreCommitRollsBackWhenClearFails(t *testing.T) {
	ctx := context.Background()
	stagedStore := NewStagedTokenStore()
	backingStore := &failingCommitTokenStore{
		TokenStore: NewInMemoryTokenStore(),
		clearErrors: map[string]error{
			"clear-provider": errors.New("clear failed"),
		},
	}

	if err := backingStore.Set(ctx, "clear-provider", &oauth2.Token{AccessToken: "old-clear-token"}); err != nil {
		t.Fatalf("backingStore.Set() clear-provider error = %v", err)
	}
	if err := backingStore.Set(ctx, "update-provider", &oauth2.Token{AccessToken: "old-update-token"}); err != nil {
		t.Fatalf("backingStore.Set() update-provider error = %v", err)
	}

	if err := stagedStore.Set(ctx, "update-provider", &oauth2.Token{AccessToken: "new-update-token"}); err != nil {
		t.Fatalf("Set() update-provider error = %v", err)
	}
	if err := stagedStore.Clear(ctx, "clear-provider"); err != nil {
		t.Fatalf("Clear() clear-provider error = %v", err)
	}

	err := stagedStore.Commit(ctx, backingStore)
	if err == nil {
		t.Fatal("Commit() error = nil, want error")
	}
	if !errors.Is(err, backingStore.clearErrors["clear-provider"]) {
		t.Fatalf("Commit() error = %v, want clear failure", err)
	}

	cleared, found, err := backingStore.Get(ctx, "clear-provider")
	if err != nil {
		t.Fatalf("backingStore.Get() clear-provider error = %v", err)
	}
	if !found {
		t.Fatal("clear-provider missing after rollback")
	}
	if cleared.AccessToken != "old-clear-token" {
		t.Fatalf("clear-provider AccessToken = %q, want %q", cleared.AccessToken, "old-clear-token")
	}

	updated, found, err := backingStore.Get(ctx, "update-provider")
	if err != nil {
		t.Fatalf("backingStore.Get() update-provider error = %v", err)
	}
	if !found {
		t.Fatal("update-provider missing after rollback")
	}
	if updated.AccessToken != "old-update-token" {
		t.Fatalf("update-provider AccessToken = %q, want %q", updated.AccessToken, "old-update-token")
	}
}

type failingCommitTokenStore struct {
	TokenStore
	setErrors   map[string]error
	clearErrors map[string]error
}

func (s *failingCommitTokenStore) Set(ctx context.Context, providerName string, token *oauth2.Token) error {
	if err, ok := s.setErrors[providerName]; ok {
		return err
	}
	return s.TokenStore.Set(ctx, providerName, token)
}

func (s *failingCommitTokenStore) Clear(ctx context.Context, providerName string) error {
	if err, ok := s.clearErrors[providerName]; ok {
		return err
	}
	return s.TokenStore.Clear(ctx, providerName)
}
