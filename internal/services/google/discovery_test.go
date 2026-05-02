package google

import (
	"context"
	"io"
	"net/http"
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
}
