package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

func TestDashboardColumnsFitStandardTerminal(t *testing.T) {
	for _, width := range []int{70, 80, 100} {
		dashboard := NewDashboardView(width, 24)
		columns := dashboard.table.Columns()

		totalWidth := 0
		for _, column := range columns {
			totalWidth += column.Width + 2
		}

		if available := tableWidth(width); totalWidth > available {
			t.Fatalf("expected dashboard columns to fit in %d cells at width %d, got %d", available, width, totalWidth)
		}
	}
}

func TestDashboardRowsPopulateDurationStatusAndLastUpdated(t *testing.T) {
	updated := time.Date(2026, 4, 28, 10, 15, 0, 0, time.Local)
	session := &models.Session{
		ID:          "session-1",
		AgentType:   models.AgentCopilot,
		ProjectPath: "/Users/example/repo/vibe-watch",
		StartTime:   updated.Add(-90 * time.Minute),
		LastUpdated: updated,
		IsActive:    true,
		Messages:    []models.Message{{Role: "user", Content: "hello"}},
		TotalTokens: models.TokenUsage{InputTokens: 1234, OutputTokens: 5678},
	}

	dashboard := NewDashboardView(80, 24)
	dashboard.SetSessions([]*models.Session{session}, "")

	rows := dashboard.table.Rows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	if got := row[5]; got != "1h30m" {
		t.Fatalf("expected duration to be populated, got %q", got)
	}
	if got := row[6]; !strings.Contains(got, "active") {
		t.Fatalf("expected status to be populated, got %q", got)
	}
	if got := row[7]; got != "10:15:00" {
		t.Fatalf("expected last updated to be populated, got %q", got)
	}
}

func TestDashboardNarrowRowsShowStateAndUpdated(t *testing.T) {
	updated := time.Date(2026, 4, 28, 10, 15, 0, 0, time.Local)
	session := &models.Session{
		ID:          "session-1",
		AgentType:   models.AgentCopilot,
		ProjectPath: "/Users/example/repo/vibe-watch",
		StartTime:   updated.Add(-10 * time.Minute),
		LastUpdated: updated,
		IsActive:    true,
	}

	dashboard := NewDashboardView(70, 24)
	dashboard.SetSessions([]*models.Session{session}, "")

	rows := dashboard.table.Rows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	if got := row[6]; got == "" || strings.Contains(got, "\x1b") {
		t.Fatalf("expected plain non-empty narrow state, got %q", got)
	}
	if got := row[7]; got == "" || strings.Contains(got, "\x1b") {
		t.Fatalf("expected plain non-empty narrow updated time, got %q", got)
	}
	if got := row[6]; got != "act" {
		t.Fatalf("expected compact active state, got %q", got)
	}
	if got := row[7]; got != "10:15" {
		t.Fatalf("expected compact updated time, got %q", got)
	}
}

func TestDashboardRowsKeepActiveStateWhenActivityFails(t *testing.T) {
	session := &models.Session{
		ID:          "session-1",
		AgentType:   models.AgentCopilot,
		ProjectPath: "/repo/vibe-watch",
		IsActive:    true,
		Messages: []models.Message{
			toolLifecycleMessage("Tool failed: bash", models.ActivityLifecycleFailed, "tool-1"),
		},
	}

	dashboard := NewDashboardView(90, 24)
	dashboard.SetSessions([]*models.Session{session}, "")

	rows := dashboard.table.Rows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if got := rows[0][6]; got != "● active" || strings.Contains(got, "\x1b") {
		t.Fatalf("expected active state despite failed activity, got %q", got)
	}
}

func TestDashboardRowsShowTerminalFailedState(t *testing.T) {
	session := &models.Session{
		ID:          "session-1",
		AgentType:   models.AgentCopilot,
		ProjectPath: "/repo/vibe-watch",
		Messages: []models.Message{
			sessionLifecycleMessage("Task incomplete", models.ActivityLifecycleFailed),
		},
	}

	dashboard := NewDashboardView(90, 24)
	dashboard.SetSessions([]*models.Session{session}, "")

	rows := dashboard.table.Rows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if got := rows[0][6]; got != "⚠ failed" || strings.Contains(got, "\x1b") {
		t.Fatalf("expected terminal failed state, got %q", got)
	}
}

func TestDashboardShowsUnavailableActiveCopilotInputTokens(t *testing.T) {
	session := &models.Session{
		AgentType:   models.AgentCopilot,
		ProjectPath: "/repo/vibe-watch",
		IsActive:    true,
		TotalTokens: models.TokenUsage{OutputTokens: 42},
	}

	dashboard := NewDashboardView(80, 24)
	dashboard.SetSessions([]*models.Session{session}, "")

	rows := dashboard.table.Rows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if got := rows[0][3]; got != "-" {
		t.Fatalf("expected unavailable input tokens marker, got %q", got)
	}
}

func TestDashboardFilterPanelUsesCommandStyling(t *testing.T) {
	dashboard := NewDashboardView(90, 24)
	dashboard.filterMode = true
	dashboard.filterInput = "copilot"
	dashboard.SetSessions([]*models.Session{{AgentType: models.AgentCopilot, ProjectPath: "/repo/vibe-watch"}}, "")

	view := dashboard.View("")
	for _, want := range []string{"Filter", "copilot", "Press enter to apply", "Session grid"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected command-style filter panel with %q, got:\n%s", want, view)
		}
	}
}

func TestAnalyticsViewFitsAdaptiveWidths(t *testing.T) {
	sessions := []*models.Session{
		{
			AgentType:   models.AgentCopilot,
			ProjectPath: "/repo/vibe-watch",
			TotalTokens: models.TokenUsage{InputTokens: 1200, OutputTokens: 300},
		},
		{
			AgentType:   models.AgentClaude,
			ProjectPath: "/repo/vibe-watch",
			TotalTokens: models.TokenUsage{InputTokens: 800, OutputTokens: 200},
		},
	}
	for _, width := range []int{70, 90, 140} {
		analytics := NewAnalyticsView(width, 30)
		analytics.SetSessions(sessions)

		view := analytics.View()
		if got := maxRenderedLineWidth(view); got > width {
			t.Fatalf("expected analytics to fit width %d, got max line width %d:\n%s", width, got, view)
		}
		if !strings.Contains(view, "Observatory analytics") || !strings.Contains(view, "Token usage by agent") {
			t.Fatalf("expected restyled analytics sections, got:\n%s", view)
		}
	}
}
