package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
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
		now: func() time.Time { return now },
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
		now: func() time.Time { return now },
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
		now: func() time.Time { return now },
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
		now: func() time.Time { return now },
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

type fakeEventFetcher struct {
	fetchFunc func(context.Context, calendar.EventQuery, int) ([]calendar.Event, error)
}

func (f *fakeEventFetcher) Fetch(ctx context.Context, query calendar.EventQuery, limit int) ([]calendar.Event, error) {
	if f.fetchFunc != nil {
		return f.fetchFunc(ctx, query, limit)
	}
	return nil, nil
}
