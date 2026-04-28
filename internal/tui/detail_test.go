package tui

import (
	"strings"
	"testing"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

func TestDetailToggleSelectedThreadCollapsesRelatedAssistants(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "first prompt"},
			{Role: "assistant", Content: "first response"},
			{Role: "assistant", Content: "second response"},
			{Role: "user", Content: "second prompt"},
			{Role: "assistant", Content: "third response"},
		},
	})

	detail.ToggleSelectedThread()
	view := detail.View()

	if strings.Contains(view, "first response") || strings.Contains(view, "second response") {
		t.Fatalf("expected selected user's assistant responses to be collapsed, got:\n%s", view)
	}
	if !strings.Contains(view, "2 assistant messages collapsed") {
		t.Fatalf("expected collapsed assistant summary, got:\n%s", view)
	}
	if !strings.Contains(view, "third response") {
		t.Fatalf("expected following user thread to remain expanded, got:\n%s", view)
	}
}

func TestDetailSelectNextUserAndCollapse(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "first prompt"},
			{Role: "assistant", Content: "first response"},
			{Role: "user", Content: "second prompt"},
			{Role: "assistant", Content: "second response"},
		},
	})

	detail.SelectNextUser()
	detail.ToggleSelectedThread()
	view := detail.View()

	if !strings.Contains(view, "first response") {
		t.Fatalf("expected first thread to remain expanded, got:\n%s", view)
	}
	if strings.Contains(view, "second response") {
		t.Fatalf("expected second thread assistant response to be collapsed, got:\n%s", view)
	}
}

func TestDetailCollapseAllThreads(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "first prompt"},
			{Role: "assistant", Content: "first response"},
			{Role: "user", Content: "second prompt"},
			{Role: "assistant", Content: "second response"},
			{Role: "assistant", Content: "third response"},
		},
	})

	detail.CollapseAllThreads()
	view := detail.View()

	if strings.Contains(view, "first response") || strings.Contains(view, "second response") || strings.Contains(view, "third response") {
		t.Fatalf("expected all assistant responses to be collapsed, got:\n%s", view)
	}
	if strings.Count(view, "assistant messages collapsed") != 2 {
		t.Fatalf("expected both user threads to show collapsed summaries, got:\n%s", view)
	}
}

func TestDetailShowsUnavailableActiveCopilotInputTokens(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		AgentType:   models.AgentCopilot,
		IsActive:    true,
		TotalTokens: models.TokenUsage{OutputTokens: 42},
	})

	view := detail.View()
	if !strings.Contains(view, "Input Tokens") || !strings.Contains(view, "-") {
		t.Fatalf("expected unavailable input token marker in detail view, got:\n%s", view)
	}
}
