package cmd

import (
	"fmt"

	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	googleservice "github.com/hossainemruz/waybar-next-events/internal/services/google"
)

func newAppRegistry() *calendar.Registry {
	registry := calendar.NewRegistry()
	if err := registry.Register(googleservice.NewService()); err != nil {
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
