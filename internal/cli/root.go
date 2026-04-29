package cli

import (
	legacycmd "github.com/hossainemruz/waybar-next-events/cmd"
	"github.com/spf13/cobra"
)

func RootCommand() *cobra.Command {
	return legacycmd.RootCommand()
}
