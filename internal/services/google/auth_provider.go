package google

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hossainemruz/waybar-next-events/internal/auth/providers"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	googlecalendar "google.golang.org/api/calendar/v3"
)

// Provider builds a Google OAuth provider for the given account using secrets
// loaded from the secret store.
func (s *Service) Provider(ctx context.Context, account calendar.Account, secretStore secrets.Store) (providers.Provider, error) {
	if strings.TrimSpace(account.ID) == "" {
		return nil, fmt.Errorf("account ID cannot be empty")
	}

	clientID := strings.TrimSpace(account.Setting(clientIDKey))
	if clientID == "" {
		return nil, fmt.Errorf("missing required setting %s", clientIDKey)
	}

	clientSecret, err := secretStore.Get(ctx, account.ID, clientSecretKey)
	if err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			return nil, fmt.Errorf("missing stored secret %s", clientSecretKey)
		}
		return nil, fmt.Errorf("load stored secret %s: %w", clientSecretKey, err)
	}

	return providers.NewGoogle(
		account.ID,
		clientID,
		clientSecret,
		[]string{googlecalendar.CalendarReadonlyScope},
	), nil
}
