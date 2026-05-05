package google

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

// mockDiscoveryTransport returns a pre-canned HTTP response for Google CalendarList requests.
type mockDiscoveryTransport struct {
	body       string
	statusCode int
}

func (m *mockDiscoveryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: m.statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Request:    req,
	}, nil
}

func TestService_DiscoverCalendars(t *testing.T) {
	ctx := context.Background()
	srv := NewService()

	t.Run("maps calendar list to domain calendars", func(t *testing.T) {
		body := `{
			"items": [
				{"id": "cal-1", "summary": "Work", "primary": true},
				{"id": "cal-2", "summary": "Personal", "primary": false}
			]
		}`
		client := &http.Client{
			Transport: &mockDiscoveryTransport{body: body, statusCode: http.StatusOK},
		}

		account := calendar.Account{ID: "acc-1", Name: "Test"}
		calendars, err := srv.DiscoverCalendars(ctx, account, client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(calendars) != 2 {
			t.Fatalf("expected 2 calendars, got %d", len(calendars))
		}

		if calendars[0].ID != "cal-1" || calendars[0].Name != "Work" || !calendars[0].Primary {
			t.Errorf("first calendar mismatch: %+v", calendars[0])
		}
		if calendars[1].ID != "cal-2" || calendars[1].Name != "Personal" || calendars[1].Primary {
			t.Errorf("second calendar mismatch: %+v", calendars[1])
		}
	})

	t.Run("empty calendar list returns empty slice", func(t *testing.T) {
		body := `{"items": []}`
		client := &http.Client{
			Transport: &mockDiscoveryTransport{body: body, statusCode: http.StatusOK},
		}

		account := calendar.Account{ID: "acc-1", Name: "Test"}
		calendars, err := srv.DiscoverCalendars(ctx, account, client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(calendars) != 0 {
			t.Fatalf("expected 0 calendars, got %d", len(calendars))
		}
	})

	t.Run("paginates through multiple pages of calendars", func(t *testing.T) {
		// Page 1 has 2 calendars and a nextPageToken.
		// Page 2 has 1 calendar and no nextPageToken.
		page1 := `{
			"items": [
				{"id": "cal-1", "summary": "Work", "primary": true},
				{"id": "cal-2", "summary": "Personal", "primary": false}
			],
			"nextPageToken": "page2"
		}`
		page2 := `{
			"items": [
				{"id": "cal-3", "summary": "Side Project", "primary": false}
			]
		}`

		client := &http.Client{
			Transport: &paginatedTransport{pages: []string{page1, page2}},
		}

		account := calendar.Account{ID: "acc-1", Name: "Test"}
		calendars, err := srv.DiscoverCalendars(ctx, account, client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(calendars) != 3 {
			t.Fatalf("expected 3 calendars, got %d", len(calendars))
		}

		if calendars[0].ID != "cal-1" {
			t.Errorf("calendar 0 ID mismatch: got %q", calendars[0].ID)
		}
		if calendars[1].ID != "cal-2" {
			t.Errorf("calendar 1 ID mismatch: got %q", calendars[1].ID)
		}
		if calendars[2].ID != "cal-3" {
			t.Errorf("calendar 2 ID mismatch: got %q", calendars[2].ID)
		}
	})

	t.Run("returns error when context cancelled between pages", func(t *testing.T) {
		// Page 1 returns a nextPageToken so the loop would attempt page 2.
		page1 := `{
			"items": [
				{"id": "cal-1", "summary": "Work", "primary": true}
			],
			"nextPageToken": "page2"
		}`

		var capturedURLs []string
		transport := &paginatedTransport{
			pages: []string{page1, page1},
			urls:  &capturedURLs,
		}

		// Cancel context after first request to trigger ctx.Err() on second iteration.
		ctx, cancel := context.WithCancel(ctx)
		cancellingTransport := &contextCancellingTransport{
			inner:  transport,
			cancel: cancel,
		}
		client := &http.Client{Transport: cancellingTransport}

		account := calendar.Account{ID: "acc-1", Name: "Test"}
		_, err := srv.DiscoverCalendars(ctx, account, client)
		if err == nil {
			t.Fatal("expected error from cancelled context, got nil")
		}
		if !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "context deadline") {
			t.Errorf("expected context-related error, got: %v", err)
		}
	})
}

// Test_pagination_sendsPageTokenInSubsequentRequests verifies that the
// DiscoverCalendars pagination implementation sends pageToken in the
// second request after receiving a nextPageToken in the first response.
func Test_pagination_sendsPageTokenInSubsequentRequests(t *testing.T) {
	page1 := `{"items": [], "nextPageToken": "page2"}`
	page2 := `{"items": []}`

	var capturedURLs []string

	client := &http.Client{
		Transport: &paginatedTransport{
			pages: []string{page1, page2},
			urls:  &capturedURLs,
		},
	}

	ctx := context.Background()
	srv := NewService()

	account := calendar.Account{ID: "acc-1", Name: "Test"}
	_, err := srv.DiscoverCalendars(ctx, account, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have made 2 requests.
	if len(capturedURLs) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(capturedURLs))
	}

	// First request should not have a pageToken.
	firstQuery := capturedURLs[0]
	if strings.Contains(firstQuery, "pageToken") {
		t.Errorf("first request should not contain pageToken, got URL: %s", firstQuery)
	}

	// Second request should include pageToken=page2.
	secondQuery := capturedURLs[1]
	parsed, err := url.Parse(secondQuery)
	if err != nil {
		t.Fatalf("failed to parse second request URL: %v", err)
	}
	if got := parsed.Query().Get("pageToken"); got != "page2" {
		t.Errorf("second request pageToken mismatch: got %q, want %q", got, "page2")
	}
}
