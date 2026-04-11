package auth

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/hossainemruz/waybar-next-events/pkg/auth/tokenstore"
	"golang.org/x/oauth2"
)

// persistingTokenSource wraps an oauth2.TokenSource and persists refreshed tokens.
type persistingTokenSource struct {
	base      oauth2.TokenSource
	store     tokenstore.TokenStore
	provider  string
	mu        sync.Mutex
	lastToken *oauth2.Token
}

// Token returns a valid token, refreshing if necessary.
// If the token was refreshed, it persists the new token to storage.
func (s *persistingTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	token, err := s.base.Token()
	if err != nil {
		return nil, err
	}

	// Check if token is different from what we had before
	// This indicates a refresh occurred
	if s.lastToken == nil || !tokensEqual(s.lastToken, token) {
		// Token was refreshed - persist it
		s.lastToken = token

		// Persist with timeout to avoid blocking indefinitely
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.store.Set(ctx, s.provider, token); err != nil {
			slog.Error("failed to persist refreshed token",
				"provider", s.provider,
				"error", err)
		}
	}

	return token, nil
}

// tokensEqual checks if two tokens are equal.
func tokensEqual(a, b *oauth2.Token) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.AccessToken == b.AccessToken &&
		a.RefreshToken == b.RefreshToken &&
		a.TokenType == b.TokenType &&
		a.Expiry.Equal(b.Expiry)
}
