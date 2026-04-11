package main

import (
	"context"
	"log"

	"github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/pkg/auth"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/providers"
)

func main() {
	ctx := context.Background()

	// Load configuration from file
	// The config file should be at $HOME/.config/waybar-next-events/config.yaml
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Get Google calendar configuration
	googleCfg, err := cfg.GetGoogleConfig()
	if err != nil {
		log.Fatalf("Failed to get google config: %v", err)
	}

	// Create authenticator with keyring storage
	authenticator := auth.NewAuthenticator(nil) // nil uses default KeyringTokenStore

	// === Google OAuth2 Example ===
	googleProvider := providers.NewGoogle(
		googleCfg.ClientID,
		googleCfg.ClientSecret, // Empty for public clients
		[]string{"https://www.googleapis.com/auth/calendar.readonly"},
	)

	// Get an HTTP client with automatic token refresh.
	// This will handle authentication if needed (via browser flow) and
	// automatically refresh tokens when they expire.
	client, err := authenticator.HTTPClient(ctx, googleProvider)
	if err != nil {
		log.Fatalf("Failed to create HTTP client: %v", err)
	}

	// Use the client with Google Calendar API or other Google APIs
	_ = client

	// === Clear credentials if needed ===
	// err = authenticator.ClearToken(ctx, googleProvider)
	// if err != nil {
	//     log.Fatalf("Failed to clear token: %v", err)
	// }
	// fmt.Println("Google credentials cleared")
}
