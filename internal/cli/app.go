package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type App struct{}

func New() *App {
	return &App{}
}

func (a *App) RootCommand() *cobra.Command {
	return RootCommand()
}

func (a *App) Run() error {
	return a.RootCommand().Execute()
}

func Execute() {
	if err := New().Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
