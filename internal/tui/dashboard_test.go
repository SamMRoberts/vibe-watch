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
