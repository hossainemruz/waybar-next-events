package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
)

// fixedNow is a deterministic time value used in tests instead of time.Now().
var fixedNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

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

	events, err := fetcher.Fetch(context.Background(), calendar.EventQuery{Now: fixedNow, DayLimit: 4}, 2)
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

	_, err := fetcher.Fetch(context.Background(), calendar.EventQuery{Now: fixedNow, DayLimit: 4}, 5)
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

	events, err := fetcher.Fetch(context.Background(), calendar.EventQuery{Now: fixedNow, DayLimit: 4}, 5)
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

	_, err := fetcher.Fetch(ctx, calendar.EventQuery{Now: fixedNow, DayLimit: 4}, 5)
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
	// When context is cancelled between account iterations, partial results from
	// already-fetched accounts are discarded and the context error is returned.
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

	_, err := fetcher.Fetch(ctx, calendar.EventQuery{Now: fixedNow, DayLimit: 4}, 5)
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

	events, err := fetcher.Fetch(context.Background(), calendar.EventQuery{Now: fixedNow, DayLimit: 4}, 2)
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

func TestEventFetcherBestEffortOneAccountFails(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{
		{ID: "a", Service: calendar.ServiceTypeGoogle, Name: "A"},
		{ID: "b", Service: calendar.ServiceTypeGoogle, Name: "B"},
	})
	service := &stubAppService{
		serviceType: calendar.ServiceTypeGoogle,
		events: []calendar.Event{
			{Title: "Event-B", Start: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		},
		fetchErrs: map[string]error{
			"a": errors.New("fetch failed for account A"),
		},
	}
	registry := NewRegistry()
	if err := registry.Register(service); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	fetcher := NewEventFetcher(loader, registry, secrets.NewInMemoryStore(), tokenstore.NewInMemoryTokenStore())
	fetcher.newAuthenticator = func() Authenticator { return &stubAuthenticator{store: tokenstore.NewInMemoryTokenStore()} }

	// Capture slog output to verify per-account errors are logged.
	var buf bytes.Buffer
	originalDefault := slog.Default()
	defer slog.SetDefault(originalDefault)
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))

	events, err := fetcher.Fetch(context.Background(), calendar.EventQuery{Now: fixedNow, DayLimit: 4}, 5)
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil (partial success)", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1 (events from successful account B)", len(events))
	}
	if events[0].Title != "Event-B" {
		t.Fatalf("events[0].Title = %q, want Event-B", events[0].Title)
	}

	// Verify the failed account's error was logged.
	logOutput := buf.String()
	if !strings.Contains(logOutput, "failed to fetch events for account") {
		t.Errorf("expected error log for failed account, got log output:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "A") {
		t.Errorf("expected log to include failed account name A, got log output:\n%s", logOutput)
	}
}

func TestEventFetcherBestEffortSucceedsWithZeroEventsAndPartialFailure(t *testing.T) {
	// When one account succeeds but returns zero events and the other fails,
	// Fetch should return (empty slice, nil) — not an aggregated error,
	// because at least one account completed its pipeline successfully.
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{
		{ID: "a", Service: calendar.ServiceTypeGoogle, Name: "A"},
		{ID: "b", Service: calendar.ServiceTypeGoogle, Name: "B"},
	})
	service := &stubAppService{
		serviceType: calendar.ServiceTypeGoogle,
		events:      []calendar.Event{}, // empty — but successful
		fetchErrs: map[string]error{
			"b": errors.New("fetch failed for account B"),
		},
	}
	registry := NewRegistry()
	if err := registry.Register(service); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	fetcher := NewEventFetcher(loader, registry, secrets.NewInMemoryStore(), tokenstore.NewInMemoryTokenStore())
	fetcher.newAuthenticator = func() Authenticator { return &stubAuthenticator{store: tokenstore.NewInMemoryTokenStore()} }

	events, err := fetcher.Fetch(context.Background(), calendar.EventQuery{Now: fixedNow, DayLimit: 4}, 5)
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil (one account succeeded)", err)
	}
	if events == nil || len(events) != 0 {
		t.Fatalf("events = %+v, want empty non-nil slice", events)
	}
}

func TestEventFetcherAllAccountsFail(t *testing.T) {
	fetchErr := errors.New("fetch failed")
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{
		{ID: "a", Service: calendar.ServiceTypeGoogle, Name: "A"},
		{ID: "b", Service: calendar.ServiceTypeGoogle, Name: "B"},
	})
	service := &stubAppService{
		serviceType: calendar.ServiceTypeGoogle,
		fetchErr:    fetchErr,
	}
	registry := NewRegistry()
	if err := registry.Register(service); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	fetcher := NewEventFetcher(loader, registry, secrets.NewInMemoryStore(), tokenstore.NewInMemoryTokenStore())
	fetcher.newAuthenticator = func() Authenticator { return &stubAuthenticator{store: tokenstore.NewInMemoryTokenStore()} }

	events, err := fetcher.Fetch(context.Background(), calendar.EventQuery{Now: fixedNow, DayLimit: 4}, 5)
	if err == nil {
		t.Fatal("Fetch() error = nil, want error when all accounts fail")
	}
	if events != nil {
		t.Fatalf("events = %+v, want nil when all accounts fail", events)
	}

	// Verify the aggregated error includes information about both accounts.
	if !errors.Is(err, fetchErr) {
		t.Fatalf("Fetch() error should wrap the underlying fetch error, got: %v", err)
	}
}

func TestEventFetcherBestEffortWithProviderError(t *testing.T) {
	loader := newMemoryConfigLoaderWithAccounts([]calendar.Account{
		{ID: "a", Service: calendar.ServiceTypeGoogle, Name: "A"},
		{ID: "b", Service: calendar.ServiceTypeGoogle, Name: "B"},
	})
	service := &stubAppService{
		serviceType: calendar.ServiceTypeGoogle,
		events: []calendar.Event{
			{Title: "Event-B", Start: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		},
		providerErrs: map[string]error{
			"a": errors.New("provider creation failed for account A"),
		},
	}
	registry := NewRegistry()
	if err := registry.Register(service); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	fetcher := NewEventFetcher(loader, registry, secrets.NewInMemoryStore(), tokenstore.NewInMemoryTokenStore())
	fetcher.newAuthenticator = func() Authenticator { return &stubAuthenticator{store: tokenstore.NewInMemoryTokenStore()} }

	events, err := fetcher.Fetch(context.Background(), calendar.EventQuery{Now: fixedNow, DayLimit: 4}, 5)
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil (partial success)", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1 (events from successful account B)", len(events))
	}
}
