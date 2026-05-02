package output

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

func mustRender(t *testing.T, events []calendar.Event, now time.Time) WaybarPayload {
	t.Helper()
	b, err := Render(events, now)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	var payload WaybarPayload
	if err := json.Unmarshal(b, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return payload
}

func TestRenderEmptyEvents(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	payload := mustRender(t, nil, now)

	if payload.Text != " No more events today!" {
		t.Fatalf("Text = %q, want No more events today!", payload.Text)
	}
	if payload.Tooltip != "" {
		t.Fatalf("Tooltip = %q, want empty", payload.Tooltip)
	}
}

func TestRenderOngoingEvent(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	events := []calendar.Event{
		{Title: "Meeting", Start: now.Add(-1 * time.Hour), End: now.Add(2 * time.Hour)},
	}
	payload := mustRender(t, events, now)

	want := "󰺏 Meeting (ends in 2h)"
	if payload.Text != want {
		t.Fatalf("Text = %q, want %q", payload.Text, want)
	}
}

func TestRenderNextEvent(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	events := []calendar.Event{
		{Title: "Lunch", Start: now.Add(30 * time.Minute), End: now.Add(1 * time.Hour)},
	}
	payload := mustRender(t, events, now)

	want := "󰃰 Lunch (starts in 30m)"
	if payload.Text != want {
		t.Fatalf("Text = %q, want %q", payload.Text, want)
	}
}

func TestRenderAllDayEventSkippedForText(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	events := []calendar.Event{
		{
			Title: "Holiday",
			Start: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 1, 15, 23, 59, 59, calendar.EndOfDayNano, time.UTC),
		},
	}
	payload := mustRender(t, events, now)

	if payload.Text != " No more events today!" {
		t.Fatalf("Text = %q, want No more events today!", payload.Text)
	}

	if !strings.Contains(payload.Tooltip, "All day") {
		t.Fatalf("Tooltip should contain All day; got %q", payload.Tooltip)
	}
}

func TestRenderTooltipGrouping(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	events := []calendar.Event{
		{Title: "A", Start: now.Add(1 * time.Hour), End: now.Add(2 * time.Hour)},
		{Title: "B", Start: now.AddDate(0, 0, 1), End: now.AddDate(0, 0, 1).Add(1 * time.Hour)},
		{Title: "C", Start: now.AddDate(0, 0, 2), End: now.AddDate(0, 0, 2).Add(1 * time.Hour)},
	}
	payload := mustRender(t, events, now)

	if !strings.Contains(payload.Tooltip, "<b>Today</b>") {
		t.Fatalf("Tooltip missing Today group; got %q", payload.Tooltip)
	}
	if !strings.Contains(payload.Tooltip, "<b>Tomorrow</b>") {
		t.Fatalf("Tooltip missing Tomorrow group; got %q", payload.Tooltip)
	}
	if !strings.Contains(payload.Tooltip, "<b>Saturday</b>") {
		t.Fatalf("Tooltip missing Saturday group; got %q", payload.Tooltip)
	}

	// Verify ordering: Today before Tomorrow before Saturday
	todayIdx := strings.Index(payload.Tooltip, "<b>Today</b>")
	tomorrowIdx := strings.Index(payload.Tooltip, "<b>Tomorrow</b>")
	satIdx := strings.Index(payload.Tooltip, "<b>Saturday</b>")
	if todayIdx == -1 || tomorrowIdx == -1 || satIdx == -1 {
		t.Fatal("missing expected day labels")
	}
	if todayIdx >= tomorrowIdx || tomorrowIdx >= satIdx {
		t.Fatalf("Day order incorrect; indices Today=%d Tomorrow=%d Saturday=%d", todayIdx, tomorrowIdx, satIdx)
	}
}

func TestRenderHTMLEscaping(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	events := []calendar.Event{
		{Title: "<script>alert('xss')</script>", Start: now.Add(1 * time.Hour), End: now.Add(2 * time.Hour)},
	}
	payload := mustRender(t, events, now)

	if strings.Contains(payload.Text, "<script>") {
		t.Fatalf("Text contains unescaped HTML: %q", payload.Text)
	}
	if !strings.Contains(payload.Text, "&lt;script&gt;") {
		t.Fatalf("Text should contain escaped HTML; got %q", payload.Text)
	}
	if strings.Contains(payload.Tooltip, "<script>") {
		t.Fatalf("Tooltip contains unescaped HTML: %q", payload.Tooltip)
	}
}

func TestRenderPastEventCheckmark(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	events := []calendar.Event{
		{Title: "Done", Start: now.Add(-2 * time.Hour), End: now.Add(-1 * time.Hour)},
		{Title: "Upcoming", Start: now.Add(1 * time.Hour), End: now.Add(2 * time.Hour)},
	}
	payload := mustRender(t, events, now)

	lines := strings.Split(payload.Tooltip, "\n")
	var doneLine string
	for _, line := range lines {
		if strings.Contains(line, "Done") {
			doneLine = line
			break
		}
	}
	if !strings.HasSuffix(doneLine, " ✓") {
		t.Fatalf("Expected past event line to end with checkmark; got %q", doneLine)
	}

	var upcomingLine string
	for _, line := range lines {
		if strings.Contains(line, "Upcoming") {
			upcomingLine = line
			break
		}
	}
	if strings.HasSuffix(upcomingLine, " ✓") {
		t.Fatalf("Expected upcoming event line without checkmark; got %q", upcomingLine)
	}
}

func TestRenderMultipleOngoingPicksLatestStarted(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	events := []calendar.Event{
		{Title: "Early", Start: now.Add(-3 * time.Hour), End: now.Add(1 * time.Hour)},
		{Title: "Late", Start: now.Add(-1 * time.Hour), End: now.Add(2 * time.Hour)},
	}
	payload := mustRender(t, events, now)

	if !strings.Contains(payload.Text, "Late") {
		t.Fatalf("Text should pick the latest-started ongoing event; got %q", payload.Text)
	}
	if strings.Contains(payload.Text, "Early") {
		t.Fatalf("Text should not contain the earlier ongoing event; got %q", payload.Text)
	}
}

func TestRenderDeterministicOrdering(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	// Events must be pre-sorted by start time; renderer no longer re-sorts.
	events := []calendar.Event{
		{Title: "A", Start: now.Add(1 * time.Hour), End: now.Add(2 * time.Hour)},
		{Title: "M", Start: now.Add(2 * time.Hour), End: now.Add(3 * time.Hour)},
		{Title: "Z", Start: now.Add(3 * time.Hour), End: now.Add(4 * time.Hour)},
	}
	payload := mustRender(t, events, now)

	// In tooltip, events should be ordered A, M, Z.
	// Search for titles as line endings to avoid matching "PM".
	aIdx := strings.Index(payload.Tooltip, "    A")
	mIdx := strings.Index(payload.Tooltip, "    M")
	zIdx := strings.Index(payload.Tooltip, "    Z")
	if aIdx >= mIdx || mIdx >= zIdx {
		t.Fatalf("Expected deterministic ordering A < M < Z; indices A=%d M=%d Z=%d", aIdx, mIdx, zIdx)
	}
}

func TestRenderFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{90 * time.Minute, "1h 30m"},
		{2 * time.Hour, "2h"},
		{45 * time.Minute, "45m"},
		{0, "0m"},
		{-30 * time.Minute, "30m"},
	}

	for _, c := range cases {
		got := formatDuration(c.d)
		if got != c.want {
			t.Fatalf("formatDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}
