package cmd

import (
	"fmt"

	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/auth/tokenstore"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	googleservice "github.com/hossainemruz/waybar-next-events/internal/services/google"
)

func newAppRegistry() *app.Registry {
	registry, err := app.NewRegistry(googleservice.NewService())
	if err != nil {
		panic(fmt.Sprintf("failed to initialize app registry: %v", err))
	}

	return registry
}

func newAccountManager() *app.AccountManager {
	return app.NewAccountManager(
		appconfig.NewLoader(),
		newAppRegistry(),
		secrets.NewKeyringStore(),
		tokenstore.NewKeyringTokenStore(),
	)
}

func newEventFetcher() *app.EventFetcher {
	return app.NewEventFetcher(
		appconfig.NewLoader(),
		newAppRegistry(),
		secrets.NewKeyringStore(),
		tokenstore.NewKeyringTokenStore(),
	)
}
