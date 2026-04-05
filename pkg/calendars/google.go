package calendars

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/emruz-hossain/waybar-next-events/pkg/types"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const (
	tokenFile = "/home/emruz/personal/projects/waybar-next-events/token.json" // or ~/.config/yourcli/token.json
	scope     = calendar.CalendarReadonlyScope
)

// Load credentials.json
func getConfig() (*oauth2.Config, error) {
	b, err := os.ReadFile("/home/emruz/personal/projects/waybar-next-events/credentials.json")
	if err != nil {
		return nil, err
	}
	return google.ConfigFromJSON(b, scope)
}

// Token cache helpers (store refresh token securely)
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var tok oauth2.Token
	return &tok, json.NewDecoder(f).Decode(&tok)
}

func saveToken(file string, token *oauth2.Token) error {
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

// Best UX: local server + auto-open browser
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	// Start listener on random loopback port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// Temporarily set redirect (Google accepts any 127.0.0.1:<port> for Desktop apps)
	originalRedirect := config.RedirectURL
	config.RedirectURL = redirectURL
	defer func() { config.RedirectURL = originalRedirect }()

	// Channel for the auth code
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	// Simple HTTP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if code := r.URL.Query().Get("code"); code != "" {
			codeCh <- code
			fmt.Fprintln(w, "Authentication successful! You can close this tab.")
		} else {
			errCh <- fmt.Errorf("no code in callback")
			http.Error(w, "No code", http.StatusBadRequest)
		}
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	defer srv.Shutdown(context.Background())

	// Generate auth URL (access_type=offline → refresh token)
	authURL := config.AuthCodeURL("state-token",
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce, // forces consent screen on first run
	)

	fmt.Println("Opening browser for Google authentication...")
	if err := openURL(authURL); err != nil {
		fmt.Printf("Please open this URL manually:\n%s\n", authURL)
	}

	// Wait for callback or timeout
	select {
	case code := <-codeCh:
		tok, err := config.Exchange(context.Background(), code)
		if err != nil {
			return nil, err
		}
		return tok, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("authentication timeout")
	}
}

// openURL opens the specified URL in the default browser
func openURL(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin": // macOS
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return exec.Command(cmd, args...).Start()
}

// Main client getter (reuse across runs)
func getCalendarClient() (*calendar.Service, error) {
	config, err := getConfig()
	if err != nil {
		return nil, err
	}

	tokFile := tokenFile
	tok, err := tokenFromFile(tokFile)
	if err != nil || tok.Expiry.Before(time.Now()) {
		tok, err = getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		if err := saveToken(tokFile, tok); err != nil {
			log.Printf("Warning: could not save token: %v", err)
		}
	}

	ctx := context.Background()
	client := config.Client(ctx, tok) // auto-refreshes using refresh token
	return calendar.NewService(ctx, option.WithHTTPClient(client))
}

func GogoleEvent() ([]types.Event, error) {
	srv, err := getCalendarClient()
	if err != nil {
		return nil, err
	}

	dayLimit := 4
	today := time.Now()
	minDay, err := startOfDate(today.Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	maxDay, err := endOfDate(today.AddDate(0, 0, dayLimit-1).Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	// List upcoming events
	events, err := srv.Events.List("emruz.hossain@qdrant.com").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(minDay.Format(time.RFC3339)).
		TimeMax(maxDay.Format(time.RFC3339)).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, err
	}

	// Convert google events to types.Event
	return convertGoogleEvents(events.Items, dayLimit, today)
}

func convertGoogleEvents(gEvents []*calendar.Event, dayLimit int, today time.Time) ([]types.Event, error) {
	events := make([]types.Event, 0)
	for _, item := range gEvents {
		title := item.Summary
		if title == "" {
			title = "<Event title missing>"
		}

		eventStartTime, eventEndTime, err := parseEventTime(*item)
		if err != nil {
			return nil, err
		}
		// For multi-day event, add one entry per day.
		if isMultiDayEvent(eventStartTime, eventEndTime) {
			for offset := range dayLimit {
				date := today.AddDate(0, 0, offset).Format(time.DateOnly)
				dayStart, err := startOfDate(date)
				if err != nil {
					return nil, err
				}
				dayEnd, err := endOfDate(date)
				if err != nil {
					return nil, err
				}
				if !eventStartToday(eventStartTime, dayEnd) {
					continue
				}
				if eventEnded(eventEndTime, dayStart) {
					break
				}
				event := types.Event{
					Title: title,
					Start: dayStart,
					End:   dayEnd,
				}
				if eventStartTime.After(dayStart) {
					event.Start = eventStartTime
				}
				if eventEndTime.Before(dayEnd) {
					event.End = eventEndTime
				}
				events = append(events, event)
			}
		} else {
			events = append(events, types.Event{
				Title: title,
				Start: eventStartTime,
				End:   eventEndTime,
			})
		}
	}
	return events, nil
}

func startOfDate(date string) (time.Time, error) {
	return time.ParseInLocation(time.DateOnly, date, time.Now().Location())
}

func endOfDate(date string) (time.Time, error) {
	t, err := time.ParseInLocation(time.DateOnly, date, time.Now().Location())
	if err != nil {
		return t, err
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, types.EndOfDayNano, t.Location()), nil
}

func parseEventTime(e calendar.Event) (start time.Time, end time.Time, err error) {
	if e.Start == nil {
		return start, end, fmt.Errorf("event has nil Start")
	}
	if e.End == nil {
		return start, end, fmt.Errorf("event has nil End")
	}

	// Parse event start time.
	if e.Start.DateTime != "" {
		// Both date and time specified.
		start, err = time.Parse(time.RFC3339, e.Start.DateTime)
		if err != nil {
			return start, end, err
		}
	} else {
		// Only date provided but not time. In this case, we set time to start of the day.
		start, err = startOfDate(e.Start.Date)
		if err != nil {
			return start, end, err
		}
	}
	// Parse event end time.
	if e.End.DateTime != "" {
		// Both date and time specified.
		end, err = time.Parse(time.RFC3339, e.End.DateTime)
		if err != nil {
			return start, end, err
		}
	} else {
		// Only date provided but not time.
		// Google Calendar uses exclusive end dates for all-day events
		// (e.g. a single-day event on Jun 15 has End.Date = "Jun 16").
		// When start and end dates are the same, the event is a full day on that date.
		if e.Start.Date == e.End.Date {
			end, err = endOfDate(e.End.Date)
		} else {
			day, err := time.ParseInLocation(time.DateOnly, e.End.Date, time.Now().Location())
			if err != nil {
				return start, end, err
			}
			end, err = endOfDate(day.AddDate(0, 0, -1).Format(time.DateOnly))
		}
		if err != nil {
			return start, end, err
		}
	}
	return start, end, nil
}

func isMultiDayEvent(start, end time.Time) bool {
	// An event is multi-day if its duration exceeds 24 hours and 1 minute.
	return end.Sub(start) > 24*time.Hour+1*time.Minute
}

func eventStartToday(eventStartTime, dayEnd time.Time) bool {
	return eventStartTime.Before(dayEnd)
}

func eventEnded(eventEndTime, dayStart time.Time) bool {
	return eventEndTime.Add(-1 * time.Minute).Before(dayStart)
}
