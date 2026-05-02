package google

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

// mockEventsTransport returns a pre-canned HTTP response for Google Events requests.
type mockEventsTransport struct {
	body       string
	statusCode int
}

func (m *mockEventsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: m.statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Request:    req,
	}, nil
}

func TestService_FetchEvents(t *testing.T) {
	ctx := context.Background()
	srv := NewService()
	loc := time.Now().Location()

	t.Run("fetches and converts events from selected calendars", func(t *testing.T) {
		body := `{
			"items": [
				{
					"summary": "Team Standup",
					"start": {"dateTime": "2025-06-15T09:00:00Z"},
					"end":   {"dateTime": "2025-06-15T09:30:00Z"}
				},
				{
					"summary": "All-Day Review",
					"start": {"date": "2025-06-15"},
					"end":   {"date": "2025-06-16"}
				}
			]
		}`
		client := &http.Client{
			Transport: &mockEventsTransport{body: body, statusCode: http.StatusOK},
		}

		account := calendar.Account{
			ID:        "acc-1",
			Name:      "Test",
			Calendars: []calendar.CalendarRef{{ID: "primary", Name: "Primary"}},
		}
		query := calendar.EventQuery{
			Now:      time.Date(2025, 6, 15, 0, 0, 0, 0, loc),
			DayLimit: 4,
		}

		events, err := srv.FetchEvents(ctx, account, query, client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
		}

		if events[0].Title != "Team Standup" {
			t.Errorf("first event title mismatch: got %q, want %q", events[0].Title, "Team Standup")
		}
		if events[1].Title != "All-Day Review" {
			t.Errorf("second event title mismatch: got %q, want %q", events[1].Title, "All-Day Review")
		}
	})

	t.Run("falls back to primary calendar when none selected", func(t *testing.T) {
		body := `{"items": []}`
		client := &http.Client{
			Transport: &mockEventsTransport{body: body, statusCode: http.StatusOK},
		}

		account := calendar.Account{
			ID:   "acc-1",
			Name: "Test",
			// No calendars selected
		}
		query := calendar.EventQuery{
			Now:      time.Date(2025, 6, 15, 0, 0, 0, 0, loc),
			DayLimit: 4,
		}

		events, err := srv.FetchEvents(ctx, account, query, client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(events) != 0 {
			t.Fatalf("expected 0 events, got %d", len(events))
		}
	})

	t.Run("fallback primary calendar error includes contextual hint", func(t *testing.T) {
		body := `{"error": {"code": 404, "message": "Not Found"}}`
		client := &http.Client{
			Transport: &mockEventsTransport{body: body, statusCode: http.StatusNotFound},
		}

		account := calendar.Account{
			ID:   "acc-1",
			Name: "Test",
			// No calendars selected
		}
		query := calendar.EventQuery{
			Now:      time.Date(2025, 6, 15, 0, 0, 0, 0, loc),
			DayLimit: 4,
		}

		_, err := srv.FetchEvents(ctx, account, query, client)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		want := "no calendars selected for account"
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error message does not contain %q:\n  got: %v", want, err)
		}
	})
}
