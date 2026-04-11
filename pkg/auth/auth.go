package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/hossainemruz/waybar-next-events/pkg/auth/providers"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/tokenstore"
	"golang.org/x/oauth2"
)

const (
	// authTimeout is the maximum time to wait for user authorization.
	authTimeout = 5 * time.Minute
	// refreshTimeout is the maximum time to wait for token refresh.
	refreshTimeout = 30 * time.Second
)

// Common errors returned by the authenticator.
var (
	// ErrReauthRequired indicates the token is expired and cannot be refreshed.
	// The user must complete the OAuth flow again.
	ErrReauthRequired = errors.New("re-authentication required: token expired and cannot be refreshed")
	// ErrProviderDenied indicates the user or provider denied the authorization.
	ErrProviderDenied = errors.New("authorization denied by provider")
	// ErrBrowserOpenFailed indicates the browser could not be opened automatically.
	ErrBrowserOpenFailed = errors.New("failed to open browser")
)

// Authenticator coordinates OAuth2 authentication for providers.
type Authenticator struct {
	store         tokenstore.TokenStore
	browserOpener func(url string) error
}

// NewAuthenticator creates a new authenticator with the given token store.
// If store is nil, a default KeyringTokenStore is used.
func NewAuthenticator(store tokenstore.TokenStore) *Authenticator {
	if store == nil {
		store = tokenstore.NewKeyringTokenStore()
	}
	return &Authenticator{
		store:         store,
		browserOpener: defaultBrowserOpener,
	}
}

// setBrowserOpener sets a custom browser opener function.
// This is useful for testing.
func (a *Authenticator) setBrowserOpener(opener func(url string) error) {
	a.browserOpener = opener
}

// Authenticate returns a valid token for the provider.
// It will:
// 1. Check for a stored valid token
// 2. Try to refresh an expired token if a refresh token exists
// 3. Start a full OAuth flow if needed
func (a *Authenticator) Authenticate(ctx context.Context, provider providers.Provider) (*oauth2.Token, error) {
	// Validate provider configuration
	if err := providers.Validate(provider); err != nil {
		return nil, fmt.Errorf("invalid provider: %w", err)
	}

	// Try to get existing token
	token, found, err := a.store.Get(ctx, provider.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to load token: %w", err)
	}

	if found {
		// Check if token is still valid
		if token.Valid() {
			return token, nil
		}

		// Token is expired - try to refresh if we have a refresh token
		if token.RefreshToken != "" {
			// Use a dedicated context with timeout for refresh to avoid
			// cancellation from caller's context (e.g., HTTP request timeout)
			refreshCtx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
			defer cancel()
			refreshed, err := a.refreshToken(refreshCtx, provider, token)
			if err == nil {
				// Save refreshed token
				if err := a.store.Set(ctx, provider.Name(), refreshed); err != nil {
					return nil, fmt.Errorf("failed to save refreshed token: %w", err)
				}
				return refreshed, nil
			}

			// Refresh failed - clear the invalid token and require re-auth
			_ = a.store.Clear(ctx, provider.Name())
			return nil, fmt.Errorf("%w: %v", ErrReauthRequired, err)
		}

		// No refresh token available - require re-auth
		_ = a.store.Clear(ctx, provider.Name())
		return nil, ErrReauthRequired
	}

	// No token found - perform full OAuth flow
	return a.performOAuthFlow(ctx, provider)
}

// TokenSource returns an oauth2.TokenSource that automatically refreshes tokens.
// The token source will persist refreshed tokens to the store.
func (a *Authenticator) TokenSource(ctx context.Context, provider providers.Provider) (oauth2.TokenSource, error) {
	token, err := a.Authenticate(ctx, provider)
	if err != nil {
		return nil, err
	}

	config := providers.ToOAuth2Config(provider)
	baseSource := config.TokenSource(ctx, token)

	// Wrap with persisting token source
	return &persistingTokenSource{
		base:     baseSource,
		store:    a.store,
		provider: provider.Name(),
	}, nil
}

// HTTPClient returns an HTTP client with automatic token refresh.
func (a *Authenticator) HTTPClient(ctx context.Context, provider providers.Provider) (*http.Client, error) {
	tokenSource, err := a.TokenSource(ctx, provider)
	if err != nil {
		return nil, err
	}

	return oauth2.NewClient(ctx, tokenSource), nil
}

// ClearToken removes the stored token for the provider.
func (a *Authenticator) ClearToken(ctx context.Context, provider providers.Provider) error {
	if err := providers.Validate(provider); err != nil {
		return fmt.Errorf("invalid provider: %w", err)
	}

	if err := a.store.Clear(ctx, provider.Name()); err != nil {
		return fmt.Errorf("failed to clear token: %w", err)
	}

	return nil
}

// refreshToken attempts to refresh an expired token.
func (a *Authenticator) refreshToken(ctx context.Context, provider providers.Provider, token *oauth2.Token) (*oauth2.Token, error) {
	config := providers.ToOAuth2Config(provider)

	tokenSource := config.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}

	return newToken, nil
}

// performOAuthFlow performs the full OAuth2 authorization code flow with PKCE.
func (a *Authenticator) performOAuthFlow(ctx context.Context, provider providers.Provider) (*oauth2.Token, error) {
	config := providers.ToOAuth2Config(provider)

	// Generate PKCE and state
	pkce, err := GeneratePKCE()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PKCE: %w", err)
	}

	state, err := GenerateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	// Build authorization URL
	authOpts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_challenge_method", pkce.Method),
		oauth2.SetAuthURLParam("code_challenge", pkce.Challenge),
	}
	authOpts = append(authOpts, provider.AuthCodeOptions()...)

	authURL := config.AuthCodeURL(state, authOpts...)

	// Start callback server
	callbackServer, err := NewCallbackServer(state)
	if err != nil {
		return nil, fmt.Errorf("failed to start callback server: %w", err)
	}
	defer callbackServer.Shutdown()

	// Start server and wait for it to be ready
	ready := callbackServer.Start()
	<-ready // Wait for server to start accepting connections

	// Open browser
	if err := a.browserOpener(authURL); err != nil {
		return nil, fmt.Errorf("%w: %v (please open this URL manually: %s)", ErrBrowserOpenFailed, err, authURL)
	}

	// Wait for callback with timeout
	authCtx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()

	result, err := callbackServer.Wait(authCtx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, errors.New("authorization timeout: no response received within 5 minutes")
		}
		return nil, fmt.Errorf("authorization failed: %w", err)
	}

	// Check for provider error
	if result.Error != "" {
		if result.ErrorDesc != "" {
			return nil, fmt.Errorf("%w: %s - %s", ErrProviderDenied, result.Error, result.ErrorDesc)
		}
		return nil, fmt.Errorf("%w: %s", ErrProviderDenied, result.Error)
	}

	// Exchange code for token
	exchangeOpts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_verifier", pkce.Verifier),
	}
	exchangeOpts = append(exchangeOpts, provider.ExchangeOptions()...)

	token, err := config.Exchange(ctx, result.Code, exchangeOpts...)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	// Store the token
	if err := a.store.Set(ctx, provider.Name(), token); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	return token, nil
}

// defaultBrowserOpener opens the given URL in the default browser.
func defaultBrowserOpener(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}
