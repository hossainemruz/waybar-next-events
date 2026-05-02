package app

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/config"
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
			{Title: "Latest", Start: time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC)},
			{Title: "Later", Start: time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)},
			{Title: "Sooner", Start: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		},
	}
	registry := NewRegistry()
	if err := registry.Register(service); err != nil {
		t.Fatalf("Register() error = %v", err)
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
	if events[1].Title != "Sooner" {
		t.Fatalf("events[1].Title = %q, want duplicated Sooner from second account due to per-account fetch stub", events[1].Title)
	}
}

func TestEventFetcherFetchReturnsNoAccountsError(t *testing.T) {
	fetcher := NewEventFetcher(newMemoryConfigLoaderWithAccounts(nil), NewRegistry(), secrets.NewInMemoryStore(), tokenstore.NewInMemoryTokenStore())

	_, err := fetcher.Fetch(context.Background(), calendar.EventQuery{Now: time.Now(), DayLimit: 4}, 5)
	if !errors.Is(err, config.ErrNoAccounts) {
		t.Fatalf("Fetch() error = %v, want ErrNoAccounts", err)
	}
}

func TestEventFetcherFetchReturnsEmptyEventsSlice(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "a", Service: calendar.ServiceTypeGoogle, Name: "A"}})
	service := &stubAppService{serviceType: calendar.ServiceTypeGoogle, events: []calendar.Event{}}
	registry := NewRegistry()
	if err := registry.Register(service); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	fetcher := NewEventFetcher(loader, registry, secrets.NewInMemoryStore(), tokenstore.NewInMemoryTokenStore())
	fetcher.newAuthenticator = func() Authenticator { return &stubAuthenticator{store: tokenstore.NewInMemoryTokenStore()} }

	events, err := fetcher.Fetch(context.Background(), calendar.EventQuery{Now: time.Now(), DayLimit: 4}, 5)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if events == nil || len(events) != 0 {
		t.Fatalf("events = %+v, want empty slice", events)
	}
}

func TestEventFetcherFetchReturnsErrorOnAlreadyCancelledContext(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{
		{ID: "a", Service: calendar.ServiceTypeGoogle, Name: "A"},
	})
	service := &stubAppService{serviceType: calendar.ServiceTypeGoogle, events: []calendar.Event{}}
	registry := NewRegistry()
	if err := registry.Register(service); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	fetcher := NewEventFetcher(loader, registry, secrets.NewInMemoryStore(), tokenstore.NewInMemoryTokenStore())
	fetcher.newAuthenticator = func() Authenticator { return &stubAuthenticator{store: tokenstore.NewInMemoryTokenStore()} }

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fetcher.Fetch(ctx, calendar.EventQuery{Now: time.Now(), DayLimit: 4}, 5)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Fetch() error = %v, want context.Canceled", err)
	}
}

// cancellingStubService wraps a stubAppService and cancels the context after the first FetchEvents call.
type cancellingStubService struct {
	*stubAppService
	cancel context.CancelFunc
	calls  int
}

func (s *cancellingStubService) FetchEvents(ctx context.Context, account calendar.Account, query calendar.EventQuery, client *http.Client) ([]calendar.Event, error) {
	s.calls++
	if s.calls == 1 {
		s.cancel()
	}
	return s.stubAppService.FetchEvents(ctx, account, query, client)
}

func TestEventFetcherFetchReturnsErrorWhenCancelledBetweenAccounts(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{
		{ID: "a", Service: calendar.ServiceTypeGoogle, Name: "A"},
		{ID: "b", Service: calendar.ServiceTypeGoogle, Name: "B"},
	})
	base := &stubAppService{
		serviceType: calendar.ServiceTypeGoogle,
		events: []calendar.Event{
			{Title: "First", Start: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	service := &cancellingStubService{stubAppService: base, cancel: cancel}
	registry := NewRegistry()
	if err := registry.Register(service); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	fetcher := NewEventFetcher(loader, registry, secrets.NewInMemoryStore(), tokenstore.NewInMemoryTokenStore())
	fetcher.newAuthenticator = func() Authenticator { return &stubAuthenticator{store: tokenstore.NewInMemoryTokenStore()} }

	_, err := fetcher.Fetch(ctx, calendar.EventQuery{Now: time.Now(), DayLimit: 4}, 5)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Fetch() error = %v, want context.Canceled", err)
	}
}

func TestEventFetcherFetchAppliesLimitWhenMoreEventsExist(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{{ID: "a", Service: calendar.ServiceTypeGoogle, Name: "A"}})
	service := &stubAppService{
		serviceType: calendar.ServiceTypeGoogle,
		events: []calendar.Event{
			{Title: "One", Start: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
			{Title: "Two", Start: time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)},
			{Title: "Three", Start: time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC)},
		},
	}
	registry := NewRegistry()
	if err := registry.Register(service); err != nil {
		t.Fatalf("Register() error = %v", err)
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
	if events[0].Title != "One" || events[1].Title != "Two" {
		t.Fatalf("events = %+v, want first two sorted events", events)
	}
}
