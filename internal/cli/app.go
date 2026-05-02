package cli

import (
	"fmt"
	"os"

	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/internal/cli/commands"
	"github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"github.com/spf13/cobra"
)

// App holds shared CLI dependencies.
type App struct {
	Registry       *app.Registry
	Loader         *config.Loader
	SecretStore    secrets.Store
	TokenStore     tokenstore.TokenStore
	AccountManager *app.AccountManager
	EventFetcher   *app.EventFetcher
}

// New creates an App with the given registry.
func New(registry *app.Registry) *App {
	if registry == nil {
		registry = app.NewRegistry()
	}
	loader := config.NewLoader()
	secretStore := secrets.NewKeyringStore()
	tokenStore := tokenstore.NewKeyringTokenStore()

	return &App{
		Registry:       registry,
		Loader:         loader,
		SecretStore:    secretStore,
		TokenStore:     tokenStore,
		AccountManager: app.NewAccountManager(loader, registry, secretStore, tokenStore),
		EventFetcher:   app.NewEventFetcher(loader, registry, secretStore, tokenStore),
	}
}

// RootCommand returns the root cobra command.
func (a *App) RootCommand() *cobra.Command {
	return commands.BuildRoot(&commands.AppDeps{
		Registry:       a.Registry,
		SecretStore:    a.SecretStore,
		AccountManager: a.AccountManager,
		EventFetcher:   a.EventFetcher,
	})
}

// Run executes the root command.
func (a *App) Run() error {
	return a.RootCommand().Execute()
}

// Execute creates a new App and runs it.
func Execute(registry *app.Registry) {
	if err := New(registry).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
