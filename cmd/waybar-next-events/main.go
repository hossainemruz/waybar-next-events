package main

import (
	"fmt"
	"os"

	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/cli"
	googleservice "github.com/hossainemruz/waybar-next-events/internal/services/google"
)

func main() {
	registry := app.NewRegistry()
	if err := registry.Register(googleservice.NewService()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to initialize app registry: %v\n", err)
		os.Exit(1)
	}
	cli.Execute(registry)
}
