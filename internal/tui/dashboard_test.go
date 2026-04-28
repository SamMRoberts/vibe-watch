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

func TestDashboardGroupsAllSessionsByDateAgentAndSession(t *testing.T) {
	dayOne := time.Date(2026, 4, 28, 10, 15, 0, 0, time.Local)
	dayTwo := time.Date(2026, 4, 27, 9, 0, 0, 0, time.Local)
	copilot := &models.Session{
		ID:          "copilot",
		AgentType:   models.AgentCopilot,
		ProjectPath: "/repo/vibe-watch",
		LastUpdated: dayOne,
	}
	claude := &models.Session{
		ID:          "claude",
		AgentType:   models.AgentClaude,
		ProjectPath: "/repo/other-project",
		LastUpdated: dayOne.Add(-time.Hour),
	}
	codex := &models.Session{
		ID:          "codex",
		AgentType:   models.AgentCodex,
		ProjectPath: "/repo/third-project",
		LastUpdated: dayTwo,
	}

	dashboard := NewDashboardView(120, 30)
	dashboard.SetSessions([]*models.Session{codex, copilot, claude}, "")

	rows := dashboard.table.Rows()
	if len(rows) != 8 {
		t.Fatalf("expected date, agent, and session rows, got %d rows: %#v", len(rows), rows)
	}
	if rows[0][0] != "Apr 28, 2026" || rows[0][1] != "2 sessions" {
		t.Fatalf("expected first date group to be Apr 28 with 2 sessions, got %#v", rows[0])
	}
	if !strings.Contains(rows[1][0], "Claude") || rows[1][1] != "1 session" {
		t.Fatalf("expected Claude agent group under Apr 28, got %#v", rows[1])
	}
	if !strings.Contains(rows[2][0], "Claude") || dashboard.rowSessions[2] != claude {
		t.Fatalf("expected Claude session row after date and agent headers, got row %#v sessions %#v", rows[2], dashboard.rowSessions[:3])
	}
	if dashboard.rowSessions[0] != nil || dashboard.rowSessions[1] != nil {
		t.Fatalf("expected date and agent headers to be non-selectable, got %#v", dashboard.rowSessions[:2])
	}
	if dashboard.SelectedSession() != claude {
		t.Fatalf("expected initial selection to skip hierarchy headers, got %#v", dashboard.SelectedSession())
	}
	if !strings.Contains(rows[3][0], "Copilot") || rows[3][1] != "1 session" {
		t.Fatalf("expected Copilot agent group under Apr 28, got %#v", rows[3])
	}
	if rows[5][0] != "Apr 27, 2026" || rows[5][1] != "1 session" {
		t.Fatalf("expected second date group to be Apr 27 with 1 session, got %#v", rows[5])
	}
	if !strings.Contains(rows[6][0], "Codex") || rows[6][1] != "1 session" {
		t.Fatalf("expected Codex agent group under Apr 27, got %#v", rows[6])
	}
	if dashboard.rowSessions[4] != copilot || dashboard.rowSessions[7] != codex {
		t.Fatalf("expected all sessions to remain selectable across repositories, got %#v", dashboard.rowSessions)
	}
	dashboard.MoveDown()
	if dashboard.SelectedSession() != copilot {
		t.Fatalf("expected moving down to skip the next agent header, got %#v", dashboard.SelectedSession())
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
