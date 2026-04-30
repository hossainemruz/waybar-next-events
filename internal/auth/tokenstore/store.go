// Package tokenstore provides secure OAuth2 token storage abstractions and implementations.
package tokenstore

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
)

// TokenStore defines the interface for persisting and retrieving OAuth2 tokens.
// Implementations must be safe for concurrent use.
type TokenStore interface {
	// Set stores the token for the given token key.
	// The token key must be unique and stable for the account.
	Set(ctx context.Context, tokenKey string, token *oauth2.Token) error

	// Get retrieves the token for the given token key.
	// Returns (token, true, nil) if found.
	// Returns (nil, false, nil) if not found.
	// Returns (nil, false, err) on storage error.
	Get(ctx context.Context, tokenKey string) (*oauth2.Token, bool, error)

	// Clear removes the stored token for the given token key.
	// Returns nil if the token was successfully removed or did not exist.
	Clear(ctx context.Context, tokenKey string) error
}

// TokenKey returns the stable token storage key for an account.
func TokenKey(serviceType, accountID string) string {
	return serviceType + "/" + accountID
}

func validateTokenKey(key string) error {
	if key == "" {
		return fmt.Errorf("token key cannot be empty")
	}

	return nil
}
