package commands

import (
	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
	"github.com/spf13/cobra"
)

// AppDeps holds the shared dependencies needed by commands.
type AppDeps struct {
	Registry       *calendar.Registry
	SecretStore    secrets.Store
	AccountManager *app.AccountManager
	EventFetcher   *app.EventFetcher
}

// BuildRoot constructs the root command with all subcommands wired to deps.
func BuildRoot(deps *AppDeps) *cobra.Command {
	root := &cobra.Command{
		Use:   "waybar-next-events",
		Short: "Show upcoming calendar events in Waybar",
		Long:  "A CLI tool that displays upcoming calendar events, designed to integrate with Waybar.",
	}

	root.AddCommand(buildListCmd(deps.EventFetcher))
	root.AddCommand(buildAccountCmd(deps))

	return root
}
