package main

import (
	"fmt"
	"log"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/config"
)

func main() {
	loader := config.NewLoaderWithPath("examples/config.json")
	cfg, err := loader.Load()
	if err != nil {
		log.Fatal(err)
	}

	if cfg.Google == nil || len(cfg.Google.Accounts) == 0 {
		log.Fatal("No accounts configured")
	}

	// Step 1: Select an account
	accountOptions := huh.NewOptions[string]()
	for _, ac := range cfg.Google.Accounts {
		accountOptions = append(accountOptions, huh.NewOption(ac.Name, ac.Name))
	}

	var selectedAccountName string
	accountForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select an account").
				Options(accountOptions...).
				Value(&selectedAccountName),
		),
	)

	if err := accountForm.Run(); err != nil {
		log.Fatal(err)
	}

	// Find the selected account
	var selectedAccount *config.GoogleAccount
	for i := range cfg.Google.Accounts {
		if cfg.Google.Accounts[i].Name == selectedAccountName {
			selectedAccount = &cfg.Google.Accounts[i]
			break
		}
	}

	if selectedAccount == nil {
		log.Fatal("Selected account not found")
	}

	// Step 2: Select calendars (multi-select)
	if len(selectedAccount.Calendars) == 0 {
		fmt.Printf("\nAccount: %s\n", selectedAccount.Name)
		fmt.Println("No calendars configured for this account.")
		return
	}

	calendarOptions := huh.NewOptions[string]()
	for _, cal := range selectedAccount.Calendars {
		calendarOptions = append(calendarOptions, huh.NewOption(cal.Name, cal.ID))
	}

	listHeight := len(selectedAccount.Calendars) + 1
	var selectedCalendarIDs []string
	calendarForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(fmt.Sprintf("Select calendars for account: %s", selectedAccount.Name)).
				Options(calendarOptions...).
				Value(&selectedCalendarIDs).
				Height(listHeight),
		),
	)

	if err := calendarForm.Run(); err != nil {
		log.Fatal(err)
	}

	// Step 3: Print selected information
	fmt.Println("\n========================================")
	fmt.Printf("Selected Account: %s\n", selectedAccount.Name)
	fmt.Println("----------------------------------------")
	fmt.Println("Selected Calendars:")
	for _, cal := range selectedAccount.Calendars {
		// Check if this calendar was selected
		for _, selectedID := range selectedCalendarIDs {
			if cal.ID == selectedID {
				fmt.Printf("  - %s (ID: %s)\n", cal.Name, cal.ID)
				break
			}
		}
	}
	fmt.Println("========================================")
}
