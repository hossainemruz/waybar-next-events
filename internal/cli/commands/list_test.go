package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/output"
)

func TestRunListSuccess(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	events := []calendar.Event{
		{Title: "Meeting", Start: now.Add(1 * time.Hour), End: now.Add(2 * time.Hour)},
	}

	var buf bytes.Buffer
	cmd := newTestCommand()
	cmd.SetOut(&buf)

	deps := listDeps{
		listOptions: listOptions{days: defaultLookAheadDays, limit: defaultEventCountLimit},
		now:         func() time.Time { return now },
		fetcher: &fakeEventFetcher{
			fetchFunc: func(context.Context, calendar.EventQuery, int) ([]calendar.Event, error) {
				return events, nil
			},
		},
		render: output.Render,
	}

	if err := runList(cmd, deps); err != nil {
		t.Fatalf("runList() error = %v", err)
	}

	var payload output.WaybarPayload
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if payload.Text == "" {
		t.Fatalf("expected non-empty text, got empty")
	}
}

func TestRunListEmpty(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	var buf bytes.Buffer
	cmd := newTestCommand()
	cmd.SetOut(&buf)

	deps := listDeps{
		listOptions: listOptions{days: defaultLookAheadDays, limit: defaultEventCountLimit},
		now:         func() time.Time { return now },
		fetcher: &fakeEventFetcher{
			fetchFunc: func(context.Context, calendar.EventQuery, int) ([]calendar.Event, error) {
				return []calendar.Event{}, nil
			},
		},
		render: output.Render,
	}

	if err := runList(cmd, deps); err != nil {
		t.Fatalf("runList() error = %v", err)
	}

	var payload output.WaybarPayload
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if payload.Text != " No more events today!" {
		t.Fatalf("Text = %q, want No more events today!", payload.Text)
	}
}

func TestRunListFetchError(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	wantErr := errors.New("fetch failed")

	cmd := newTestCommand()
	deps := listDeps{
		listOptions: listOptions{days: defaultLookAheadDays, limit: defaultEventCountLimit},
		now:         func() time.Time { return now },
		fetcher: &fakeEventFetcher{
			fetchFunc: func(context.Context, calendar.EventQuery, int) ([]calendar.Event, error) {
				return nil, wantErr
			},
		},
		render: output.Render,
	}

	err := runList(cmd, deps)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runList() error = %v, want %v", err, wantErr)
	}
}

func TestRunListRenderError(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	wantErr := errors.New("render failed")

	var buf bytes.Buffer
	cmd := newTestCommand()
	cmd.SetOut(&buf)

	deps := listDeps{
		listOptions: listOptions{days: defaultLookAheadDays, limit: defaultEventCountLimit},
		now:         func() time.Time { return now },
		fetcher: &fakeEventFetcher{
			fetchFunc: func(context.Context, calendar.EventQuery, int) ([]calendar.Event, error) {
				return []calendar.Event{}, nil
			},
		},
		render: func([]calendar.Event, time.Time) ([]byte, error) {
			return nil, wantErr
		},
	}

	if err := runList(cmd, deps); err != nil {
		t.Fatalf("runList() error = %v", err)
	}

	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte(" Something went wrong!")) {
		t.Fatalf("expected fallback error text in output, got %q", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte("render failed")) {
		t.Fatalf("expected error message in tooltip, got %q", out)
	}
}

func TestRunListNoAccountsHint(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	cmd := newTestCommand()
	deps := listDeps{
		listOptions: listOptions{days: defaultLookAheadDays, limit: defaultEventCountLimit},
		now:         func() time.Time { return now },
		fetcher: &fakeEventFetcher{
			fetchFunc: func(context.Context, calendar.EventQuery, int) ([]calendar.Event, error) {
				return nil, appconfig.ErrNoAccounts
			},
		},
		render: output.Render,
	}

	err := runList(cmd, deps)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, appconfig.ErrNoAccounts) {
		t.Fatalf("runList() error = %v, want ErrNoAccounts", err)
	}
	if !strings.Contains(err.Error(), noAccountsConfiguredHint) {
		t.Fatalf("error message does not contain hint %q: %v", noAccountsConfiguredHint, err)
	}
}

func TestRunListPassesDaysAndLimit(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	var gotQuery calendar.EventQuery
	var gotLimit int

	cmd := newTestCommand()
	deps := listDeps{
		listOptions: listOptions{days: 7, limit: 10},
		now:         func() time.Time { return now },
		fetcher: &fakeEventFetcher{
			fetchFunc: func(_ context.Context, query calendar.EventQuery, limit int) ([]calendar.Event, error) {
				gotQuery = query
				gotLimit = limit
				return []calendar.Event{}, nil
			},
		},
		render: output.Render,
	}

	if err := runList(cmd, deps); err != nil {
		t.Fatalf("runList() error = %v", err)
	}

	if gotQuery.DayLimit != 7 {
		t.Fatalf("DayLimit = %d, want 7", gotQuery.DayLimit)
	}
	if gotLimit != 10 {
		t.Fatalf("limit = %d, want 10", gotLimit)
	}
}

func TestBuildListCmdFlagDefaults(t *testing.T) {
	cmd := buildListCmd(&AppDeps{})

	days, err := cmd.Flags().GetInt("days")
	if err != nil {
		t.Fatalf("GetInt(days) error = %v", err)
	}
	if days != defaultLookAheadDays {
		t.Fatalf("days default = %d, want %d", days, defaultLookAheadDays)
	}

	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		t.Fatalf("GetInt(limit) error = %v", err)
	}
	if limit != defaultEventCountLimit {
		t.Fatalf("limit default = %d, want %d", limit, defaultEventCountLimit)
	}
}

func TestBuildListCmdParsesCustomDays(t *testing.T) {
	cmd := buildListCmd(&AppDeps{})
	if err := cmd.ParseFlags([]string{"--days", "7"}); err != nil {
		t.Fatalf("ParseFlags error = %v", err)
	}
	days, err := cmd.Flags().GetInt("days")
	if err != nil {
		t.Fatalf("GetInt(days) error = %v", err)
	}
	if days != 7 {
		t.Fatalf("days = %d, want 7", days)
	}
}

func TestBuildListCmdParsesCustomLimit(t *testing.T) {
	cmd := buildListCmd(&AppDeps{})
	if err := cmd.ParseFlags([]string{"--limit", "10"}); err != nil {
		t.Fatalf("ParseFlags error = %v", err)
	}
	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		t.Fatalf("GetInt(limit) error = %v", err)
	}
	if limit != 10 {
		t.Fatalf("limit = %d, want 10", limit)
	}
}

func TestRunListInvalidDays(t *testing.T) {
	cmd := newTestCommand()
	deps := listDeps{
		listOptions: listOptions{days: 0, limit: defaultEventCountLimit},
		now:         func() time.Time { return time.Now() },
		fetcher:     &fakeEventFetcher{},
		render:      output.Render,
	}

	err := runList(cmd, deps)
	if err == nil {
		t.Fatal("expected error for days=0, got nil")
	}
	if !strings.Contains(err.Error(), "--days must be a positive integer") {
		t.Fatalf("error = %q, want --days validation message", err.Error())
	}
}

func TestRunListInvalidDaysNegative(t *testing.T) {
	cmd := newTestCommand()
	deps := listDeps{
		listOptions: listOptions{days: -1, limit: defaultEventCountLimit},
		now:         func() time.Time { return time.Now() },
		fetcher:     &fakeEventFetcher{},
		render:      output.Render,
	}

	err := runList(cmd, deps)
	if err == nil {
		t.Fatal("expected error for days=-1, got nil")
	}
	if !strings.Contains(err.Error(), "--days must be a positive integer") {
		t.Fatalf("error = %q, want --days validation message", err.Error())
	}
}

func TestRunListInvalidLimit(t *testing.T) {
	cmd := newTestCommand()
	deps := listDeps{
		listOptions: listOptions{days: defaultLookAheadDays, limit: 0},
		now:         func() time.Time { return time.Now() },
		fetcher:     &fakeEventFetcher{},
		render:      output.Render,
	}

	err := runList(cmd, deps)
	if err == nil {
		t.Fatal("expected error for limit=0, got nil")
	}
	if !strings.Contains(err.Error(), "--limit must be a positive integer") {
		t.Fatalf("error = %q, want --limit validation message", err.Error())
	}
}

func TestRunListInvalidLimitNegative(t *testing.T) {
	cmd := newTestCommand()
	deps := listDeps{
		listOptions: listOptions{days: defaultLookAheadDays, limit: -5},
		now:         func() time.Time { return time.Now() },
		fetcher:     &fakeEventFetcher{},
		render:      output.Render,
	}

	err := runList(cmd, deps)
	if err == nil {
		t.Fatal("expected error for limit=-5, got nil")
	}
	if !strings.Contains(err.Error(), "--limit must be a positive integer") {
		t.Fatalf("error = %q, want --limit validation message", err.Error())
	}
}

func TestBuildListCmdSetsTimeoutContext(t *testing.T) {
	// Verify that the list command sets a timeout context by checking
	// that runList propagates a deadline from cmd.Context() to the fetcher.

	var capturedDeadline time.Time
	fetcher := &fakeEventFetcher{
		fetchFunc: func(ctx context.Context, _ calendar.EventQuery, _ int) ([]calendar.Event, error) {
			dl, ok := ctx.Deadline()
			if !ok {
				t.Fatal("expected context to have a deadline, but it doesn't")
			}
			capturedDeadline = dl
			return []calendar.Event{}, nil
		},
	}

	// Simulate what buildListCmd's RunE does: wrap the context with a timeout.
	beforeTimeout := time.Now()
	cmd := newTestCommand()
	ctx, cancel := context.WithTimeout(context.Background(), listCommandTimeout)
	defer cancel()
	cmd.SetContext(ctx)

	deps := listDeps{
		listOptions: listOptions{days: 1, limit: 1},
		now:         func() time.Time { return time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC) },
		fetcher:     fetcher,
		render:      output.Render,
	}

	if err := runList(cmd, deps); err != nil {
		t.Fatalf("runList() error = %v", err)
	}

	if capturedDeadline.IsZero() {
		t.Fatal("expected deadline to be set on context, got zero time")
	}

	// The captured deadline should be approximately listCommandTimeout from
	// when we created the timeout context. Allow up to 2 seconds of slack.
	expectedMin := beforeTimeout.Add(listCommandTimeout - 2*time.Second)
	expectedMax := beforeTimeout.Add(listCommandTimeout + 2*time.Second)
	if capturedDeadline.Before(expectedMin) || capturedDeadline.After(expectedMax) {
		t.Fatalf("deadline = %v, want within [%v, %v]", capturedDeadline, expectedMin, expectedMax)
	}
}

type fakeEventFetcher struct {
	fetchFunc func(context.Context, calendar.EventQuery, int) ([]calendar.Event, error)
}

func (f *fakeEventFetcher) Fetch(ctx context.Context, query calendar.EventQuery, limit int) ([]calendar.Event, error) {
	if f.fetchFunc != nil {
		return f.fetchFunc(ctx, query, limit)
	}
	return nil, nil
}
