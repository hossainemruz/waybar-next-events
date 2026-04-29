package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "waybar-next-events",
	Short: "Show upcoming calendar events in Waybar",
	Long:  "A CLI tool that displays upcoming calendar events, designed to integrate with Waybar.",
}

func RootCommand() *cobra.Command {
	return rootCmd
}

func Execute() {
	if err := RootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
