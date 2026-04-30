package main

import (
	"context"
	"fmt"
	"log"

	appcalendar "github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/pkg/auth"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/providers"
	"google.golang.org/api/calendar/v3"
)

func main() {
	ctx := context.Background()

	// Load configuration from file
	// The config file should be at $HOME/.config/waybar-next-events/config.json
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Get Google calendar configuration
	googleCfg := cfg.AccountsByService(appcalendar.ServiceTypeGoogle)

	// Create authenticator with keyring storage
	authenticator := auth.NewAuthenticator(nil) // nil uses default KeyringTokenStore

	// Authenticate with each configured Google account
	for i := range googleCfg {
		account := &googleCfg[i]

		fmt.Printf("=== Google Account: %s ===\n", account.Name)

		// Create Google OAuth2 provider for this account
		googleProvider := providers.NewGoogle(
			account.Setting("client_id"),
			account.Setting("client_secret"),
			[]string{calendar.CalendarReadonlyScope},
		)

		// Get an HTTP client with automatic token refresh.
		// This will handle authentication if needed (via browser flow) and
		// automatically refresh tokens when they expire.
		client, err := authenticator.HTTPClient(ctx, googleProvider)
		if err != nil {
			log.Fatalf("Failed to create HTTP client for account %q: %v", account.Name, err)
		}

		// Use the client with Google Calendar API or other Google APIs
		_ = client
		fmt.Printf("Successfully authenticated account %q\n", account.Name)

		// Print configured calendars for this account
		for _, cal := range account.Calendars {
			fmt.Printf("  Calendar: %s (%s)\n", cal.Name, cal.ID)
		}
		if len(account.Calendars) == 0 {
			fmt.Println("  No specific calendars configured; will use 'primary'")
		}

		// === Clear credentials if needed ===
		// err = authenticator.ClearToken(ctx, googleProvider)
		// if err != nil {
		//     log.Fatalf("Failed to clear token: %v", err)
		// }
		// fmt.Printf("Credentials cleared for account %q\n", account.Name)
	}
}
