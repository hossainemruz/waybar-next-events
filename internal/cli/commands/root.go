package commands

import (
	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"github.com/spf13/cobra"
)

// Version is the application version. It can be overridden at build time with
// go build -ldflags "-X github.com/hossainemruz/waybar-next-events/internal/cli/commands.Version=v1.2.3".
var Version = "dev"

// AppDeps holds the shared dependencies needed by commands.
type AppDeps struct {
	Registry       *app.Registry
	SecretStore    secrets.Store
	AccountManager *app.AccountManager
	EventFetcher   *app.EventFetcher
}

// BuildRoot constructs the root command with all subcommands wired to deps.
func BuildRoot(deps *AppDeps) *cobra.Command {
	root := &cobra.Command{
		Use:     "waybar-next-events",
		Short:   "Show upcoming calendar events in Waybar",
		Long:    "A CLI tool that displays upcoming calendar events, designed to integrate with Waybar.",
		Version: Version,
	}

	root.AddCommand(buildListCmd(deps))
	root.AddCommand(buildAccountCmd(deps))

	return root
}
