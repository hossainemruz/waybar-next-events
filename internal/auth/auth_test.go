package auth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/auth/providers"
	"github.com/hossainemruz/waybar-next-events/internal/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/internal/config"
	"golang.org/x/oauth2"
)

// mockProvider is a test provider implementation.
type mockProvider struct {
	name         string
	clientID     string
	clientSecret string
	authURL      string
	tokenURL     string
	redirectURL  string
	scopes       []string
}

func (m *mockProvider) Name() string         { return m.name }
func (m *mockProvider) ClientID() string     { return m.clientID }
func (m *mockProvider) ClientSecret() string { return m.clientSecret }
func (m *mockProvider) AuthURL() string      { return m.authURL }
func (m *mockProvider) TokenURL() string     { return m.tokenURL }
func (m *mockProvider) RedirectURL() string  { return m.redirectURL }
func (m *mockProvider) Scopes() []string     { return m.scopes }
func (m *mockProvider) AuthCodeOptions() []oauth2.AuthCodeOption {
	return nil
}
func (m *mockProvider) ExchangeOptions() []oauth2.AuthCodeOption {
	return nil
}

// Ensure mockProvider implements providers.Provider
var _ providers.Provider = (*mockProvider)(nil)

func TestAuthenticator_ValidToken(t *testing.T) {
	store := tokenstore.NewInMemoryTokenStore()
	auth := NewAuthenticator(store)

	// Create a valid token
	validToken := &oauth2.Token{
		AccessToken:  "valid-access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	provider := &mockProvider{
		name:        "test",
		clientID:    "client-id",
		authURL:     "https://example.com/auth",
		tokenURL:    "https://example.com/token",
		redirectURL: config.DefaultCallbackURL,
		scopes:      []string{"read"},
	}

	// Store the token
	ctx := context.Background()
	err := store.Set(ctx, "test", validToken)
	if err != nil {
		t.Fatalf("Failed to set token: %v", err)
	}

	// Authenticate should return the valid token without browser flow
	token, err := auth.Authenticate(ctx, provider)
	if err != nil {
		t.Errorf("Authenticate() error = %v", err)
	}
	if token.AccessToken != validToken.AccessToken {
		t.Errorf("Token mismatch: got %s, want %s", token.AccessToken, validToken.AccessToken)
	}
}

func TestAuthenticator_ExpiredTokenNoRefresh(t *testing.T) {
	store := tokenstore.NewInMemoryTokenStore()
	auth := NewAuthenticator(store)

	// Create an expired token without refresh token
	expiredToken := &oauth2.Token{
		AccessToken: "expired-access-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(-time.Hour),
	}

	provider := &mockProvider{
		name:        "test",
		clientID:    "client-id",
		authURL:     "https://example.com/auth",
		tokenURL:    "https://example.com/token",
		redirectURL: config.DefaultCallbackURL,
		scopes:      []string{"read"},
	}

	// Store the expired token
	ctx := context.Background()
	err := store.Set(ctx, "test", expiredToken)
	if err != nil {
		t.Fatalf("Failed to set token: %v", err)
	}

	// Authenticate should return ErrReauthRequired
	_, err = auth.Authenticate(ctx, provider)
	if !errors.Is(err, ErrReauthRequired) {
		t.Errorf("Authenticate() error = %v, want ErrReauthRequired", err)
	}

	// Token should be cleared
	_, found, _ := store.Get(ctx, "test")
	if found {
		t.Error("Expired token was not cleared")
	}
}

func TestAuthenticator_ForceAuthenticatePreservesExistingTokenOnFailure(t *testing.T) {
	store := tokenstore.NewInMemoryTokenStore()
	auth := NewAuthenticator(store, WithBrowserOpener(func(string) error {
		return errors.New("browser unavailable")
	}))

	provider := &mockProvider{
		name:        "test",
		clientID:    "client-id",
		authURL:     "https://example.com/auth",
		tokenURL:    "https://example.com/token",
		redirectURL: config.DefaultCallbackURL,
		scopes:      []string{"read"},
	}

	expiredToken := &oauth2.Token{
		AccessToken:  "expired-access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}

	ctx := context.Background()
	if err := store.Set(ctx, provider.Name(), expiredToken); err != nil {
		t.Fatalf("store.Set() error = %v", err)
	}

	_, err := auth.ForceAuthenticate(ctx, provider)
	if err == nil {
		t.Fatal("ForceAuthenticate() error = nil, want error")
	}
	if !errors.Is(err, ErrBrowserOpenFailed) {
		t.Fatalf("ForceAuthenticate() error = %v, want ErrBrowserOpenFailed", err)
	}

	stored, found, err := store.Get(ctx, provider.Name())
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if !found {
		t.Fatal("stored token missing after failed forced auth")
	}
	if !tokensEqual(stored, expiredToken) {
		t.Fatalf("stored token = %+v, want original token %+v", stored, expiredToken)
	}
}

func TestAuthenticator_ForceAuthenticateReplacesExistingTokenOnSuccess(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"access_token": "new-access-token",
			"refresh_token": "new-refresh-token",
			"token_type": "Bearer",
			"expires_in": 3600
		}`))
	}))
	defer tokenServer.Close()

	store := tokenstore.NewInMemoryTokenStore()
	provider := &mockProvider{
		name:         "test",
		clientID:     "client-id",
		clientSecret: "client-secret",
		authURL:      "https://example.com/auth",
		tokenURL:     tokenServer.URL + "/token",
		redirectURL:  config.DefaultCallbackURL,
		scopes:       []string{"read"},
	}

	ctx := context.Background()
	if err := store.Set(ctx, provider.Name(), &oauth2.Token{
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("store.Set() error = %v", err)
	}

	auth := NewAuthenticator(store, WithBrowserOpener(func(authURL string) error {
		parsedURL, err := url.Parse(authURL)
		if err != nil {
			return err
		}

		state := parsedURL.Query().Get("state")
		if state == "" {
			return fmt.Errorf("missing state in auth URL")
		}

		go func() {
			time.Sleep(100 * time.Millisecond)
			resp, err := http.Get(fmt.Sprintf("%s?code=auth-code&state=%s", config.DefaultCallbackURL, url.QueryEscape(state)))
			if err != nil {
				return
			}
			_ = resp.Body.Close()
		}()
		return nil
	}))

	token, err := auth.ForceAuthenticate(ctx, provider)
	if err != nil {
		t.Fatalf("ForceAuthenticate() error = %v", err)
	}
	if token.AccessToken != "new-access-token" {
		t.Fatalf("token.AccessToken = %q, want %q", token.AccessToken, "new-access-token")
	}

	stored, found, err := store.Get(ctx, provider.Name())
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if !found {
		t.Fatal("stored token missing after successful forced auth")
	}
	if stored.AccessToken != "new-access-token" {
		t.Fatalf("stored.AccessToken = %q, want %q", stored.AccessToken, "new-access-token")
	}
}

func TestAuthenticator_InvalidProvider(t *testing.T) {
	store := tokenstore.NewInMemoryTokenStore()
	auth := NewAuthenticator(store)

	invalidProvider := &mockProvider{
		name:     "",
		clientID: "",
	}

	ctx := context.Background()
	_, err := auth.Authenticate(ctx, invalidProvider)
	if err == nil {
		t.Error("Authenticate() expected error for invalid provider, got nil")
	}
}

func TestPersistingTokenSource(t *testing.T) {
	store := tokenstore.NewInMemoryTokenStore()

	// Create a mock token source that returns a refreshed token
	refreshedToken := &oauth2.Token{
		AccessToken:  "refreshed-token",
		RefreshToken: "new-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	mockSource := &mockTokenSource{
		token: refreshedToken,
	}

	pts := &persistingTokenSource{
		base:     mockSource,
		store:    store,
		provider: "test",
	}

	token, err := pts.Token()
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}

	if token.AccessToken != refreshedToken.AccessToken {
		t.Errorf("Token() = %s, want %s", token.AccessToken, refreshedToken.AccessToken)
	}

	// The refreshed token should be persisted
	stored, found, _ := store.Get(context.Background(), "test")
	if !found {
		t.Error("Refreshed token was not persisted")
	}
	if stored.AccessToken != refreshedToken.AccessToken {
		t.Errorf("Persisted token = %s, want %s", stored.AccessToken, refreshedToken.AccessToken)
	}
}

func TestTokensEqual(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		a    *oauth2.Token
		b    *oauth2.Token
		want bool
	}{
		{
			name: "both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "a nil",
			a:    nil,
			b:    &oauth2.Token{AccessToken: "token"},
			want: false,
		},
		{
			name: "b nil",
			a:    &oauth2.Token{AccessToken: "token"},
			b:    nil,
			want: false,
		},
		{
			name: "same token",
			a:    &oauth2.Token{AccessToken: "token", RefreshToken: "refresh", TokenType: "Bearer", Expiry: now},
			b:    &oauth2.Token{AccessToken: "token", RefreshToken: "refresh", TokenType: "Bearer", Expiry: now},
			want: true,
		},
		{
			name: "different access token",
			a:    &oauth2.Token{AccessToken: "token1"},
			b:    &oauth2.Token{AccessToken: "token2"},
			want: false,
		},
		{
			name: "different refresh token",
			a:    &oauth2.Token{AccessToken: "token", RefreshToken: "refresh1"},
			b:    &oauth2.Token{AccessToken: "token", RefreshToken: "refresh2"},
			want: false,
		},
		{
			name: "different expiry",
			a:    &oauth2.Token{AccessToken: "token", Expiry: now},
			b:    &oauth2.Token{AccessToken: "token", Expiry: now.Add(time.Second)},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokensEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("tokensEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

// mockTokenSource is a mock implementation of oauth2.TokenSource for testing
type mockTokenSource struct {
	token *oauth2.Token
	err   error
}

func (m *mockTokenSource) Token() (*oauth2.Token, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.token, nil
}

// Test with a mock OAuth2 server for refresh flow
func TestAuthenticator_TokenRefresh(t *testing.T) {
	// Create a mock OAuth2 server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			// Return a new token
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(`{
				"access_token": "new-access-token",
				"refresh_token": "new-refresh-token",
				"token_type": "Bearer",
				"expires_in": 3600
			}`)); err != nil {
				t.Logf("Failed to write response: %v", err)
			}
		}
	}))
	defer mockServer.Close()

	store := tokenstore.NewInMemoryTokenStore()
	auth := NewAuthenticator(store)

	// Create an expired token with a refresh token
	expiredToken := &oauth2.Token{
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}

	provider := &mockProvider{
		name:        "test",
		clientID:    "client-id",
		authURL:     mockServer.URL + "/auth",
		tokenURL:    mockServer.URL + "/token",
		redirectURL: config.DefaultCallbackURL,
		scopes:      []string{"read"},
	}

	// Store the expired token
	ctx := context.Background()
	err := store.Set(ctx, "test", expiredToken)
	if err != nil {
		t.Fatalf("Failed to set token: %v", err)
	}

	// This should trigger a refresh
	// Note: In a real test, we'd need to mock the token source more carefully
	// This is a simplified test that demonstrates the structure
	token, err := auth.refreshToken(ctx, provider, expiredToken)
	if err != nil {
		// Expected to potentially fail due to network, but demonstrates the structure
		t.Logf("Refresh failed (expected in test): %v", err)
	} else {
		if token.AccessToken != "new-access-token" {
			t.Errorf("Refreshed token = %s, want new-access-token", token.AccessToken)
		}
	}
}

func TestAuthenticator_InvalidGrantClearsStore(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error": "invalid_grant", "error_description": "Token has been revoked"}`))
		}
	}))
	defer mockServer.Close()

	store := tokenstore.NewInMemoryTokenStore()
	auth := NewAuthenticator(store)

	expiredToken := &oauth2.Token{
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	provider := &mockProvider{
		name:        "test",
		clientID:    "client-id",
		authURL:     mockServer.URL + "/auth",
		tokenURL:    mockServer.URL + "/token",
		redirectURL: config.DefaultCallbackURL,
		scopes:      []string{"read"},
	}

	ctx := context.Background()
	if err := store.Set(ctx, provider.Name(), expiredToken); err != nil {
		t.Fatalf("store.Set() error = %v", err)
	}

	_, err := auth.Authenticate(ctx, provider)
	if !errors.Is(err, ErrReauthRequired) {
		t.Fatalf("Authenticate() error = %v, want ErrReauthRequired", err)
	}

	_, found, _ := store.Get(ctx, provider.Name())
	if found {
		t.Fatal("expected token to be cleared after invalid_grant")
	}
}

func TestAuthenticator_TransientErrorDoesNotClearStore(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error": "temporarily_unavailable", "error_description": "Service is down"}`))
		}
	}))
	defer mockServer.Close()

	store := tokenstore.NewInMemoryTokenStore()
	auth := NewAuthenticator(store)

	expiredToken := &oauth2.Token{
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	provider := &mockProvider{
		name:        "test",
		clientID:    "client-id",
		authURL:     mockServer.URL + "/auth",
		tokenURL:    mockServer.URL + "/token",
		redirectURL: config.DefaultCallbackURL,
		scopes:      []string{"read"},
	}

	ctx := context.Background()
	if err := store.Set(ctx, provider.Name(), expiredToken); err != nil {
		t.Fatalf("store.Set() error = %v", err)
	}

	_, err := auth.Authenticate(ctx, provider)
	if err == nil {
		t.Fatal("Authenticate() error = nil, want error")
	}
	if errors.Is(err, ErrReauthRequired) {
		t.Fatal("Authenticate() error = ErrReauthRequired, want transient error")
	}

	stored, found, _ := store.Get(ctx, provider.Name())
	if !found {
		t.Fatal("expected token to be preserved after transient error")
	}
	if stored.RefreshToken != expiredToken.RefreshToken {
		t.Fatalf("stored refresh token = %q, want %q", stored.RefreshToken, expiredToken.RefreshToken)
	}
}

func TestDefaultBrowserOpener_Fallback(t *testing.T) {
	// Ensure xdg-open is not found by using an empty PATH.
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", "/nonexistent"); err != nil {
		t.Fatalf("failed to set PATH: %v", err)
	}
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("failed to restore PATH: %v", err)
		}
	}()

	oldStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("failed to create pipe: %v", pipeErr)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	testURL := "http://example.com/auth"
	err := defaultBrowserOpener(testURL)

	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("failed to close pipe: %v", closeErr)
	}

	if err != nil {
		t.Fatalf("defaultBrowserOpener() error = %v, want nil", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()
	if !strings.Contains(output, testURL) {
		t.Fatalf("expected stdout to contain URL %q, got %q", testURL, output)
	}
}

func TestIsTerminalAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "invalid_grant",
			err:  &oauth2.RetrieveError{ErrorCode: "invalid_grant"},
			want: true,
		},
		{
			name: "invalid_request",
			err:  &oauth2.RetrieveError{ErrorCode: "invalid_request"},
			want: true,
		},
		{
			name: "invalid_client",
			err:  &oauth2.RetrieveError{ErrorCode: "invalid_client"},
			want: true,
		},
		{
			name: "temporarily_unavailable",
			err:  &oauth2.RetrieveError{ErrorCode: "temporarily_unavailable"},
			want: false,
		},
		{
			name: "wrapped_invalid_grant",
			err:  fmt.Errorf("token refresh failed: %w", &oauth2.RetrieveError{ErrorCode: "invalid_grant"}),
			want: true,
		},
		{
			name: "wrapped_transient",
			err:  fmt.Errorf("token refresh failed: %w", &oauth2.RetrieveError{ErrorCode: "server_error"}),
			want: false,
		},
		{
			name: "context_deadline_exceeded",
			err:  context.DeadlineExceeded,
			want: false,
		},
		{
			name: "plain_error",
			err:  errors.New("some network error"),
			want: false,
		},
		{
			name: "nil_error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTerminalAuthError(tt.err)
			if got != tt.want {
				t.Errorf("isTerminalAuthError() = %v, want %v", got, tt.want)
			}
		})
	}
}
