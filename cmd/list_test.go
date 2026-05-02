package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/output"
	"github.com/spf13/cobra"
)

func TestRunListSuccess(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	events := []calendar.Event{
		{Title: "Meeting", Start: now.Add(1 * time.Hour), End: now.Add(2 * time.Hour)},
	}

	var buf bytes.Buffer
	cmd := newTestCommand()
	cmd.SetOut(&buf)

	deps := listDependencies{
		now: func() time.Time { return now },
		fetchEvents: func(cmd *cobra.Command, query calendar.EventQuery, limit int) ([]calendar.Event, error) {
			return events, nil
		},
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

	deps := listDependencies{
		now: func() time.Time { return now },
		fetchEvents: func(cmd *cobra.Command, query calendar.EventQuery, limit int) ([]calendar.Event, error) {
			return []calendar.Event{}, nil
		},
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
	deps := listDependencies{
		now: func() time.Time { return now },
		fetchEvents: func(cmd *cobra.Command, query calendar.EventQuery, limit int) ([]calendar.Event, error) {
			return nil, wantErr
		},
	}

	err := runList(cmd, deps)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runList() error = %v, want %v", err, wantErr)
	}
}

func TestRunListRenderError(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	var buf bytes.Buffer
	cmd := newTestCommand()
	cmd.SetOut(&buf)

	deps := listDependencies{
		now: func() time.Time { return now },
		fetchEvents: func(cmd *cobra.Command, query calendar.EventQuery, limit int) ([]calendar.Event, error) {
			return []calendar.Event{}, nil
		},
	}

	if err := runList(cmd, deps); err != nil {
		t.Fatalf("runList() error = %v", err)
	}

	out := buf.String()
	if out == "" {
		t.Fatal("expected output for empty events, got none")
	}
}
