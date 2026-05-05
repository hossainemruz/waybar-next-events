package google

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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

	t.Run("paginates through multiple pages of events", func(t *testing.T) {
		// Page 1: 1 event with nextPageToken.
		page1 := `{
			"items": [
				{
					"summary": "Morning Standup",
					"start": {"dateTime": "2025-06-15T09:00:00Z"},
					"end":   {"dateTime": "2025-06-15T09:30:00Z"}
				}
			],
			"nextPageToken": "page2"
		}`
		// Page 2: 1 event, no nextPageToken.
		page2 := `{
			"items": [
				{
					"summary": "Afternoon Review",
					"start": {"dateTime": "2025-06-15T14:00:00Z"},
					"end":   {"dateTime": "2025-06-15T15:00:00Z"}
				}
			]
		}`

		var capturedURLs []string
		client := &http.Client{
			Transport: &paginatedTransport{
				pages: []string{page1, page2},
				urls:  &capturedURLs,
			},
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
		if events[0].Title != "Morning Standup" {
			t.Errorf("event 0 title: got %q, want %q", events[0].Title, "Morning Standup")
		}
		if events[1].Title != "Afternoon Review" {
			t.Errorf("event 1 title: got %q, want %q", events[1].Title, "Afternoon Review")
		}

		// Verify that two requests were made (first without pageToken, second with pageToken).
		if len(capturedURLs) != 2 {
			t.Fatalf("expected 2 HTTP requests, got %d", len(capturedURLs))
		}
		firstParsed, err := url.Parse(capturedURLs[0])
		if err != nil {
			t.Fatalf("failed to parse first request URL: %v", err)
		}
		if pt := firstParsed.Query().Get("pageToken"); pt != "" {
			t.Errorf("first request should not have pageToken, got %q", pt)
		}
		secondParsed, err := url.Parse(capturedURLs[1])
		if err != nil {
			t.Fatalf("failed to parse second request URL: %v", err)
		}
		if pt := secondParsed.Query().Get("pageToken"); pt != "page2" {
			t.Errorf("second request pageToken: got %q, want %q", pt, "page2")
		}
	})

	t.Run("includes TimeZone parameter in events query", func(t *testing.T) {
		body := `{"items": []}`
		var capturedURLs []string
		client := &http.Client{
			Transport: &paginatedTransport{
				pages: []string{body, body},
				urls:  &capturedURLs,
			},
		}

		// Use a specific timezone so the test is deterministic.
		loc := time.FixedZone("America/New_York", -5*3600)
		account := calendar.Account{
			ID:        "acc-1",
			Name:      "Test",
			Calendars: []calendar.CalendarRef{{ID: "primary", Name: "Primary"}},
		}
		query := calendar.EventQuery{
			Now:      time.Date(2025, 6, 15, 0, 0, 0, 0, loc),
			DayLimit: 4,
		}

		_, err := srv.FetchEvents(ctx, account, query, client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(capturedURLs) < 1 {
			t.Fatal("expected at least 1 HTTP request")
		}

		parsed, err := url.Parse(capturedURLs[0])
		if err != nil {
			t.Fatalf("failed to parse request URL: %v", err)
		}
		gotTZ := parsed.Query().Get("timeZone")
		if gotTZ != loc.String() {
			t.Errorf("timeZone query param: got %q, want %q", gotTZ, loc.String())
		}
	})

	t.Run("returns error when context cancelled between pages", func(t *testing.T) {
		// Page 1 has an event and a nextPageToken, triggering a second page request.
		page1 := `{
			"items": [
				{
					"summary": "Morning Standup",
					"start": {"dateTime": "2025-06-15T09:00:00Z"},
					"end":   {"dateTime": "2025-06-15T09:30:00Z"}
				}
			],
			"nextPageToken": "page2"
		}`

		var capturedURLs []string
		transport := &paginatedTransport{
			pages: []string{page1, page1},
			urls:  &capturedURLs,
		}

		// Cancel context after the first request to trigger ctx.Err() on second iteration.
		ctx, cancel := context.WithCancel(ctx)
		cancelling := &contextCancellingTransport{
			inner:  transport,
			cancel: cancel,
		}
		client := &http.Client{Transport: cancelling}

		account := calendar.Account{
			ID:        "acc-1",
			Name:      "Test",
			Calendars: []calendar.CalendarRef{{ID: "primary", Name: "Primary"}},
		}
		query := calendar.EventQuery{
			Now:      time.Date(2025, 6, 15, 0, 0, 0, 0, loc),
			DayLimit: 4,
		}

		_, err := srv.FetchEvents(ctx, account, query, client)
		if err == nil {
			t.Fatal("expected error from cancelled context, got nil")
		}
		if !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "context deadline") {
			t.Errorf("expected context-related error, got: %v", err)
		}
	})

	t.Run("logs warning when falling back to primary calendar", func(t *testing.T) {
		body := `{"items": []}`
		client := &http.Client{
			Transport: &mockEventsTransport{body: body, statusCode: http.StatusOK},
		}

		account := calendar.Account{
			ID:   "acc-1",
			Name: "TestUser",
			// No calendars selected — triggers fallback to "primary"
		}
		query := calendar.EventQuery{
			Now:      time.Date(2025, 6, 15, 0, 0, 0, 0, loc),
			DayLimit: 4,
		}

		// Capture slog output.
		var buf bytes.Buffer
		originalDefault := slog.Default()
		defer slog.SetDefault(originalDefault)
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))

		_, err := srv.FetchEvents(ctx, account, query, client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		logOutput := buf.String()
		if !strings.Contains(logOutput, "falling back to primary") {
			t.Errorf("expected warning log about falling back to primary, got log output:\n%s", logOutput)
		}
		if !strings.Contains(logOutput, "TestUser") {
			t.Errorf("expected warning log to include account name TestUser, got log output:\n%s", logOutput)
		}
	})

	t.Run("does not log warning when calendars are explicitly selected", func(t *testing.T) {
		body := `{"items": []}`
		client := &http.Client{
			Transport: &mockEventsTransport{body: body, statusCode: http.StatusOK},
		}

		account := calendar.Account{
			ID:        "acc-1",
			Name:      "TestUser",
			Calendars: []calendar.CalendarRef{{ID: "work", Name: "Work"}},
		}
		query := calendar.EventQuery{
			Now:      time.Date(2025, 6, 15, 0, 0, 0, 0, loc),
			DayLimit: 4,
		}

		// Capture slog output.
		var buf bytes.Buffer
		originalDefault := slog.Default()
		defer slog.SetDefault(originalDefault)
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))

		_, err := srv.FetchEvents(ctx, account, query, client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		logOutput := buf.String()
		if strings.Contains(logOutput, "falling back to primary") {
			t.Errorf("did not expect warning log about falling back to primary, but got:\n%s", logOutput)
		}
	})
}
