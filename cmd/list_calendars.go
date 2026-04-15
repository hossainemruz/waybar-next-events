package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/pkg/auth"
	"github.com/hossainemruz/waybar-next-events/pkg/auth/providers"
	"github.com/spf13/cobra"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

var accountName string

var listCalendarsCmd = &cobra.Command{
	Use:   "list-calendars",
	Short: "List available calendars for a Google account",
	Long:  "List all calendars available in a Google account. Use --account to specify which account when multiple are configured.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load configuration
		loader := config.NewLoader()
		cfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		googleCfg, err := cfg.GetGoogleConfig()
		if err != nil {
			return fmt.Errorf("failed to get google config: %w", err)
		}

		if len(googleCfg.Accounts) == 0 {
			return fmt.Errorf("no google accounts configured")
		}

		// Find the target account
		var account *config.GoogleAccount
		for i := range googleCfg.Accounts {
			if googleCfg.Accounts[i].Name == accountName {
				account = &googleCfg.Accounts[i]
				break
			}
		}

		if account == nil {
			if accountName == "" && len(googleCfg.Accounts) == 1 {
				account = &googleCfg.Accounts[0]
			} else {
				fmt.Println("Please specify an account using --account flag. Available accounts:")
				for _, acc := range googleCfg.Accounts {
					fmt.Printf("  - %s\n", acc.Name)
				}
				os.Exit(1)
			}
		}

		// Create Google OAuth provider for the selected account
		googleProvider := providers.NewGoogle(
			account.ClientID,
			account.ClientSecret,
			[]string{calendar.CalendarReadonlyScope},
		)

		// Get authenticated HTTP client
		authenticator := auth.NewAuthenticator(nil)
		client, err := authenticator.HTTPClient(ctx, googleProvider)
		if err != nil {
			return fmt.Errorf("failed to authenticate account %q: %w", account.Name, err)
		}

		// Create calendar service
		srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			return fmt.Errorf("failed to create calendar service: %w", err)
		}

		// List all calendars
		calendarList, err := srv.CalendarList.List().Do()
		if err != nil {
			return fmt.Errorf("failed to list calendars: %w", err)
		}

		// Print results
		fmt.Printf("\nAvailable Calendars for account %q:\n", account.Name)
		fmt.Println("================================================================================")
		fmt.Printf("%-40s | %-50s | %s\n", "Calendar Name", "Calendar ID", "Type")
		fmt.Println("--------------------------------------------------------------------------------")

		for _, item := range calendarList.Items {
			calendarType := "Secondary"
			if item.Primary {
				calendarType = "Primary"
			}

			// Truncate long names for display
			name := item.Summary
			if len(name) > 38 {
				name = name[:35] + "..."
			}

			fmt.Printf("%-40s | %-50s | %s\n", name, item.Id, calendarType)
		}
		fmt.Println("================================================================================")
		fmt.Printf("\nTotal calendars: %d\n\n", len(calendarList.Items))

		fmt.Println("To use a specific calendar, add it to your config.json under the account's calendars array:")
		fmt.Println(`  { "name": "Calendar Name", "id": "CALENDAR_ID_HERE" }`)

		return nil
	},
}

func init() {
	listCalendarsCmd.Flags().StringVar(&accountName, "account", "", "Name of the Google account to list calendars for (required if multiple accounts are configured)")
	rootCmd.AddCommand(listCalendarsCmd)
}
