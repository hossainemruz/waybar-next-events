// Package tokenstore provides secure OAuth2 token storage abstractions and implementations.
package tokenstore

import (
	"context"
	"errors"

	"golang.org/x/oauth2"
)

// TokenStore defines the interface for persisting and retrieving OAuth2 tokens.
// Implementations must be safe for concurrent use.
type TokenStore interface {
	// Set stores the token for the given provider name.
	// The provider name must be unique and stable for the provider.
	Set(ctx context.Context, providerName string, token *oauth2.Token) error

	// Get retrieves the token for the given provider name.
	// Returns (token, true, nil) if found.
	// Returns (nil, false, nil) if not found.
	// Returns (nil, false, err) on storage error.
	Get(ctx context.Context, providerName string) (*oauth2.Token, bool, error)

	// Clear removes the stored token for the given provider name.
	// Returns nil if the token was successfully removed or did not exist.
	Clear(ctx context.Context, providerName string) error
}

// Common errors returned by TokenStore implementations.
var (
	// ErrTokenNotFound indicates no token exists for the provider.
	ErrTokenNotFound = errors.New("token not found")
)
