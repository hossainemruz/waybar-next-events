package app

import (
	"context"
	"testing"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
)

func TestEventFetcherFetchSortsAndLimitsEvents(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{
		{ID: "b", Service: calendar.ServiceTypeGoogle, Name: "B"},
		{ID: "a", Service: calendar.ServiceTypeGoogle, Name: "A"},
	})
	service := &stubAppService{
		serviceType: calendar.ServiceTypeGoogle,
		events: []calendar.Event{
			{Title: "Later", Start: time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)},
			{Title: "Sooner", Start: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		},
	}
	registry, err := NewRegistry(service)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	fetcher := NewEventFetcher(loader, registry, secrets.NewInMemoryStore(), tokenstore.NewInMemoryTokenStore())
	fetcher.newAuthenticator = func() Authenticator { return &stubAuthenticator{store: tokenstore.NewInMemoryTokenStore()} }

	events, err := fetcher.Fetch(context.Background(), calendar.EventQuery{Now: time.Now(), DayLimit: 4}, 2)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Title != "Sooner" {
		t.Fatalf("events[0].Title = %q, want Sooner", events[0].Title)
	}
}
