package providers

import (
	"strings"
	"testing"

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

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid provider",
			provider: &mockProvider{
				name:        "test",
				clientID:    "client-id",
				authURL:     "https://example.com/auth",
				tokenURL:    "https://example.com/token",
				redirectURL: "http://127.0.0.1:18751/callback",
				scopes:      []string{"scope1"},
			},
			wantErr: false,
		},
		{
			name:     "nil provider",
			provider: nil,
			wantErr:  true,
			errMsg:   "provider cannot be nil",
		},
		{
			name: "empty name",
			provider: &mockProvider{
				name:        "",
				clientID:    "client-id",
				authURL:     "https://example.com/auth",
				tokenURL:    "https://example.com/token",
				redirectURL: "http://127.0.0.1:18751/callback",
			},
			wantErr: true,
			errMsg:  "provider name cannot be empty",
		},
		{
			name: "empty client ID",
			provider: &mockProvider{
				name:        "test",
				clientID:    "",
				authURL:     "https://example.com/auth",
				tokenURL:    "https://example.com/token",
				redirectURL: "http://127.0.0.1:18751/callback",
			},
			wantErr: true,
			errMsg:  "client ID cannot be empty",
		},
		{
			name: "empty auth URL",
			provider: &mockProvider{
				name:        "test",
				clientID:    "client-id",
				authURL:     "",
				tokenURL:    "https://example.com/token",
				redirectURL: "http://127.0.0.1:18751/callback",
			},
			wantErr: true,
			errMsg:  "auth URL cannot be empty",
		},
		{
			name: "empty token URL",
			provider: &mockProvider{
				name:        "test",
				clientID:    "client-id",
				authURL:     "https://example.com/auth",
				tokenURL:    "",
				redirectURL: "http://127.0.0.1:18751/callback",
			},
			wantErr: true,
			errMsg:  "token URL cannot be empty",
		},
		{
			name: "empty redirect URL",
			provider: &mockProvider{
				name:        "test",
				clientID:    "client-id",
				authURL:     "https://example.com/auth",
				tokenURL:    "https://example.com/token",
				redirectURL: "",
			},
			wantErr: true,
			errMsg:  "redirect URL cannot be empty",
		},
		{
			name: "invalid redirect URL host",
			provider: &mockProvider{
				name:        "test",
				clientID:    "client-id",
				authURL:     "https://example.com/auth",
				tokenURL:    "https://example.com/token",
				redirectURL: "http://example.com/callback",
			},
			wantErr: true,
			errMsg:  "redirect URL must use 127.0.0.1",
		},
		{
			name: "redirect URL using localhost instead of 127.0.0.1",
			provider: &mockProvider{
				name:        "test",
				clientID:    "client-id",
				authURL:     "https://example.com/auth",
				tokenURL:    "https://example.com/token",
				redirectURL: "http://localhost:18751/callback",
			},
			wantErr: true,
			errMsg:  "redirect URL must use 127.0.0.1",
		},
		{
			name: "redirect URL missing port",
			provider: &mockProvider{
				name:        "test",
				clientID:    "client-id",
				authURL:     "https://example.com/auth",
				tokenURL:    "https://example.com/token",
				redirectURL: "http://127.0.0.1/callback",
			},
			wantErr: true,
			errMsg:  "redirect URL must specify a port",
		},
		{
			name: "redirect URL using https scheme",
			provider: &mockProvider{
				name:        "test",
				clientID:    "client-id",
				authURL:     "https://example.com/auth",
				tokenURL:    "https://example.com/token",
				redirectURL: "https://127.0.0.1:18751/callback",
			},
			wantErr: true,
			errMsg:  "redirect URL must use http scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error message = %v, want containing %v", err, tt.errMsg)
				}
			}
		})
	}
}
