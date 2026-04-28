package tui

import (
	"testing"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

func TestUpdateViewsRefreshesSelectedDetailSession(t *testing.T) {
	oldSession := &models.Session{
		ID:          "session-1",
		AgentType:   models.AgentCopilot,
		ProjectPath: "/repo/project",
		Messages: []models.Message{
			{Role: "assistant", Content: "old response"},
		},
		LastUpdated: time.Now().Add(-time.Minute),
	}
	updatedSession := &models.Session{
		ID:          "session-1",
		AgentType:   models.AgentCopilot,
		ProjectPath: "/repo/project",
		Messages: []models.Message{
			{Role: "assistant", Content: "new response"},
		},
		LastUpdated: time.Now(),
	}

	app := &App{
		sessions: []*models.Session{updatedSession},
		detail:   NewDetailView(100, 40),
	}
	app.detail.SetSession(oldSession)

	app.updateViews()

	if app.detail.session != updatedSession {
		t.Fatalf("expected selected detail session to be rebound to refreshed session")
	}
	if got := app.detail.session.Messages[0].Content; got != "new response" {
		t.Fatalf("expected refreshed detail content, got %q", got)
	}
}

func TestFindMatchingSessionFallsBackToLogPath(t *testing.T) {
	target := &models.Session{LogPath: "/tmp/session.jsonl"}
	updated := &models.Session{LogPath: "/tmp/session.jsonl"}

	if got := findMatchingSession(target, []*models.Session{updated}); got != updated {
		t.Fatalf("expected session to match by log path")
	}
}

func TestUpdateViewsScrollsActiveDetailSessionToBottom(t *testing.T) {
	oldSession := &models.Session{
		ID:        "session-1",
		AgentType: models.AgentCopilot,
		IsActive:  true,
		Messages:  makeDetailMessages(20),
	}
	updatedSession := &models.Session{
		ID:        "session-1",
		AgentType: models.AgentCopilot,
		IsActive:  true,
		Messages:  makeDetailMessages(40),
	}

	app := &App{
		view:     viewDetail,
		sessions: []*models.Session{updatedSession},
		detail:   NewDetailView(100, 20),
	}
	app.detail.SetSession(oldSession)
	app.detail.viewport.GotoBottom()

	app.updateViews()

	if !app.detail.viewport.AtBottom() {
		t.Fatalf("expected active detail viewport to auto-scroll to bottom")
	}
}

func TestUpdateViewsDoesNotScrollActiveDetailSessionWhenUserScrolledUp(t *testing.T) {
	oldSession := &models.Session{
		ID:        "session-1",
		AgentType: models.AgentCopilot,
		IsActive:  true,
		Messages:  makeDetailMessages(40),
	}
	updatedSession := &models.Session{
		ID:        "session-1",
		AgentType: models.AgentCopilot,
		IsActive:  true,
		Messages:  makeDetailMessages(60),
	}

	app := &App{
		view:     viewDetail,
		sessions: []*models.Session{updatedSession},
		detail:   NewDetailView(100, 20),
	}
	app.detail.SetSession(oldSession)
	app.detail.viewport.GotoBottom()
	app.detail.PageUp()
	previousOffset := app.detail.viewport.YOffset

	app.updateViews()

	if app.detail.viewport.AtBottom() {
		t.Fatalf("expected active detail viewport to preserve scrolled-up position")
	}
	if app.detail.viewport.YOffset != previousOffset {
		t.Fatalf("expected scrolled-up offset %d to be preserved, got %d", previousOffset, app.detail.viewport.YOffset)
	}
}

func makeDetailMessages(count int) []models.Message {
	messages := make([]models.Message, 0, count)
	for i := 0; i < count; i++ {
		messages = append(messages, models.Message{
			Role:    "assistant",
			Content: "line",
		})
	}
	return messages
}
