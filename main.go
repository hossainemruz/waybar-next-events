package main

import (
	"fmt"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/cli"
	googleservice "github.com/hossainemruz/waybar-next-events/internal/services/google"
)

func main() {
	registry := calendar.NewRegistry()
	if err := registry.Register(googleservice.NewService()); err != nil {
		panic(fmt.Sprintf("failed to initialize app registry: %v", err))
	}
	cli.Execute(registry)
}
