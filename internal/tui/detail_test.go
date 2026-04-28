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
	if !strings.Contains(view, "2 activity entries collapsed") {
		t.Fatalf("expected collapsed activity summary, got:\n%s", view)
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
	if strings.Count(view, "activity entries collapsed") != 2 {
		t.Fatalf("expected both user threads to show collapsed summaries, got:\n%s", view)
	}
}

func TestDetailSelectNextRowNavigatesActivityRows(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "assistant", Content: "assistant response"},
			{Role: "tool", Content: "tool output"},
		},
	})

	detail.SelectNextRow()

	row, ok := detail.selectedActivityRow()
	if !ok {
		t.Fatalf("expected selected row")
	}
	if row.messageIndex != 1 {
		t.Fatalf("expected down to select assistant activity row, got message index %d", row.messageIndex)
	}
	if detail.selectedUser != 0 {
		t.Fatalf("expected selected activity to remain associated with first user prompt, got %d", detail.selectedUser)
	}
}

func TestDetailPromptJumpSelectsUserRows(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "first prompt"},
			{Role: "assistant", Content: "first response"},
			{Role: "tool", Content: "tool output"},
			{Role: "user", Content: "second prompt"},
			{Role: "assistant", Content: "second response"},
		},
	})
	detail.SelectNextRow()

	detail.SelectNextUser()

	row, ok := detail.selectedActivityRow()
	if !ok {
		t.Fatalf("expected selected row")
	}
	if row.messageIndex != 3 {
		t.Fatalf("expected prompt jump to select second user prompt, got message index %d", row.messageIndex)
	}
}

func TestDetailOpenSelectedDetailShowsSingleEvent(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		ProjectPath: "/repo/project",
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "tool", Content: "ran tests\nall passed"},
			{Role: "assistant", Content: "done"},
		},
	})
	detail.SelectNextRow()

	if !detail.OpenSelectedDetail() {
		t.Fatalf("expected selected tool activity to open")
	}
	view := detail.ThreadView()

	if !strings.Contains(view, "Focused activity 2") || !strings.Contains(view, "ran tests") {
		t.Fatalf("expected focused event detail, got:\n%s", view)
	}
	if strings.Contains(view, "prompt") || strings.Contains(view, "done") {
		t.Fatalf("expected focused event detail to exclude surrounding thread activity, got:\n%s", view)
	}
}

func TestDetailFollowModePausesAndResumes(t *testing.T) {
	detail := NewDetailView(120, 22)
	detail.SetSession(&models.Session{
		AgentType: models.AgentCopilot,
		IsActive:  true,
		Messages:  makeTestMessages(30),
	})
	if !detail.Following() {
		t.Fatalf("expected active session detail to start in follow mode")
	}

	detail.SelectPreviousRow()
	if detail.Following() {
		t.Fatalf("expected manual activity navigation to pause follow mode")
	}

	detail.ToggleFollow()
	if !detail.Following() {
		t.Fatalf("expected follow toggle to resume follow mode")
	}
	if !detail.AtBottom() {
		t.Fatalf("expected resuming follow mode to scroll to bottom")
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

func TestDetailHeaderStaysVisibleAfterScrolling(t *testing.T) {
	detail := NewDetailView(120, 22)
	detail.SetSession(&models.Session{
		AgentType:   models.AgentCopilot,
		ProjectPath: "/repo/project",
		IsActive:    true,
		TotalTokens: models.TokenUsage{InputTokens: 123, OutputTokens: 456, CacheReads: 7},
		Messages:    makeTestMessages(30),
	})

	detail.PageDown()
	view := detail.View()

	for _, want := range []string{"Copilot CLI", "/repo/project", "Input Tokens", "Output Tokens", "Cache Reads", "ACTIVE"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected fixed header to remain visible with %q, got:\n%s", want, view)
		}
	}
}

func TestDetailOpenSelectedThreadShowsRelatedActivityOnly(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		ProjectPath: "/repo/project",
		Messages: []models.Message{
			{Role: "user", Content: "first prompt"},
			{Role: "assistant", Content: "first response"},
			{Role: "assistant", Content: "first follow up"},
			{Role: "user", Content: "second prompt"},
			{Role: "assistant", Content: "second response"},
		},
	})

	if !detail.OpenSelectedThread() {
		t.Fatalf("expected selected user prompt to open")
	}
	view := detail.ThreadView()

	if !strings.Contains(view, "first prompt") ||
		!strings.Contains(view, "first response") ||
		!strings.Contains(view, "first follow up") {
		t.Fatalf("expected selected prompt thread content, got:\n%s", view)
	}
	if strings.Contains(view, "second prompt") || strings.Contains(view, "second response") {
		t.Fatalf("expected prompt detail to exclude following user thread, got:\n%s", view)
	}
}

func TestDetailPromptDetailIsVerbose(t *testing.T) {
	longContent := strings.Repeat("x", 600)
	rendered := renderVerboseMessage(1, models.Message{Role: "assistant", Content: longContent}, 80)
	if strings.Count(rendered, "x") != len(longContent) {
		t.Fatalf("expected prompt detail to render full assistant content")
	}
}

func makeTestMessages(count int) []models.Message {
	messages := make([]models.Message, 0, count)
	for i := 0; i < count; i++ {
		role := "assistant"
		if i%3 == 0 {
			role = "user"
		}
		messages = append(messages, models.Message{
			Role:    role,
			Content: "activity",
		})
	}
	return messages
}
