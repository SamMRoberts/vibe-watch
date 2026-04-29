package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

func TestAnalyticsViewRendersPromptInsights(t *testing.T) {
	start := time.Date(2026, 4, 29, 1, 0, 0, 0, time.UTC)
	session := &models.Session{
		ID:          "session-1",
		AgentType:   models.AgentCopilot,
		ProjectPath: "/repo/vibe-watch",
		StartTime:   start,
		LastUpdated: start.Add(20 * time.Minute),
		Messages: []models.Message{
			{Role: "user", Content: "Implement analytics for prompt quality", Timestamp: start},
			{Role: "tool", Content: "read files", Timestamp: start.Add(time.Minute), Meta: models.ActivityMeta{Kind: models.ActivityKindTool, Label: "Read", Lifecycle: models.ActivityLifecycleStarted}},
			{Role: "assistant", Content: "done", Timestamp: start.Add(2 * time.Minute), Tokens: models.TokenUsage{InputTokens: 1000, OutputTokens: 500}},
			{Role: "user", Content: "Fix the failing test", Timestamp: start.Add(3 * time.Minute)},
			{Role: "tool", Content: "go test", Timestamp: start.Add(4 * time.Minute), Meta: models.ActivityMeta{Kind: models.ActivityKindTool, Label: "Bash", Lifecycle: models.ActivityLifecycleFailed}},
			{Role: "assistant", Content: "fixed", Timestamp: start.Add(5 * time.Minute), Tokens: models.TokenUsage{InputTokens: 600, OutputTokens: 200}},
		},
		TotalTokens: models.TokenUsage{InputTokens: 1600, OutputTokens: 700},
	}

	view := NewAnalyticsView(120, 120)
	view.SetSessions([]*models.Session{session})
	rendered := view.View()

	for _, want := range []string{
		"Prompt thread analytics",
		"Implementation",
		"Debugging",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected analytics view to include %q, got:\n%s", want, rendered)
		}
	}

	for _, want := range []string{
		"Tool activity summary",
		"Prompt refinement hints",
		"Bash",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected scrolled analytics view to include %q, got:\n%s", want, rendered)
		}
	}
}

func TestAnalyticsViewScrollsContent(t *testing.T) {
	view := NewAnalyticsView(120, 14)
	view.SetSessions([]*models.Session{analyticsTestSession()})

	if view.viewport.YOffset != 0 {
		t.Fatalf("expected analytics viewport to start at top, got offset %d", view.viewport.YOffset)
	}

	view.ScrollDown()
	if view.viewport.YOffset == 0 {
		t.Fatalf("expected analytics viewport to scroll down")
	}

	downOffset := view.viewport.YOffset
	view.ScrollUp()
	if view.viewport.YOffset >= downOffset {
		t.Fatalf("expected analytics viewport to scroll up from %d, got %d", downOffset, view.viewport.YOffset)
	}
}

func TestBuildPromptThreadsSummarizesUserSpans(t *testing.T) {
	start := time.Date(2026, 4, 29, 1, 0, 0, 0, time.UTC)
	session := &models.Session{
		ID:          "session-1",
		AgentType:   models.AgentClaude,
		ProjectPath: "/repo/app",
		LastUpdated: start.Add(10 * time.Minute),
		Messages: []models.Message{
			{Role: "assistant", Content: "prelude", Timestamp: start.Add(-time.Minute)},
			{Role: "user", Content: "Create a feature", Timestamp: start},
			{Role: "tool", Content: "Edit", Timestamp: start.Add(time.Minute), Tokens: models.TokenUsage{InputTokens: 10}, Meta: models.ActivityMeta{Kind: models.ActivityKindTool, Lifecycle: models.ActivityLifecycleStarted}},
			{Role: "assistant", Content: "done", Timestamp: start.Add(2 * time.Minute), Tokens: models.TokenUsage{OutputTokens: 20}},
			{Role: "user", Content: "Where is the parser?", Timestamp: start.Add(3 * time.Minute)},
			{Role: "assistant", Content: "there", Timestamp: start.Add(4 * time.Minute), Tokens: models.TokenUsage{InputTokens: 5, OutputTokens: 5}},
		},
	}

	threads := buildPromptThreads([]*models.Session{session})
	if len(threads) != 2 {
		t.Fatalf("expected 2 prompt threads, got %#v", threads)
	}
	if threads[0].Category != "Implementation" || threads[0].ToolEvents != 1 || threads[0].Tokens != 30 {
		t.Fatalf("expected first thread implementation with tool and tokens, got %#v", threads[0])
	}
	if threads[0].Duration != 2*time.Minute {
		t.Fatalf("expected first thread duration 2m, got %s", threads[0].Duration)
	}
	if threads[1].Category != "Exploration" || threads[1].Tokens != 10 {
		t.Fatalf("expected second thread exploration with tokens, got %#v", threads[1])
	}
}

func analyticsTestSession() *models.Session {
	start := time.Date(2026, 4, 29, 1, 0, 0, 0, time.UTC)
	return &models.Session{
		ID:          "session-1",
		AgentType:   models.AgentCopilot,
		ProjectPath: "/repo/vibe-watch",
		StartTime:   start,
		LastUpdated: start.Add(20 * time.Minute),
		Messages: []models.Message{
			{Role: "user", Content: "Implement analytics for prompt quality", Timestamp: start},
			{Role: "tool", Content: "read files", Timestamp: start.Add(time.Minute), Meta: models.ActivityMeta{Kind: models.ActivityKindTool, Label: "Read", Lifecycle: models.ActivityLifecycleStarted}},
			{Role: "assistant", Content: "done", Timestamp: start.Add(2 * time.Minute), Tokens: models.TokenUsage{InputTokens: 1000, OutputTokens: 500}},
			{Role: "user", Content: "Fix the failing test", Timestamp: start.Add(3 * time.Minute)},
			{Role: "tool", Content: "go test", Timestamp: start.Add(4 * time.Minute), Meta: models.ActivityMeta{Kind: models.ActivityKindTool, Label: "Bash", Lifecycle: models.ActivityLifecycleFailed}},
			{Role: "assistant", Content: "fixed", Timestamp: start.Add(5 * time.Minute), Tokens: models.TokenUsage{InputTokens: 600, OutputTokens: 200}},
		},
		TotalTokens: models.TokenUsage{InputTokens: 1600, OutputTokens: 700},
	}
}

func TestClassifyPrompt(t *testing.T) {
	tests := map[string]string{
		"Fix the failing CI build":     "Debugging",
		"Implement the analytics page": "Implementation",
		"Create a plan for rollout":    "Planning",
		"Where is parsing handled?":    "Exploration",
		"Refactor the watcher":         "Refactor",
		"Review this for security":     "Review",
		"Thanks":                       "General",
	}
	for prompt, want := range tests {
		if got := classifyPrompt(prompt); got != want {
			t.Fatalf("classifyPrompt(%q) = %q, want %q", prompt, got, want)
		}
	}
}
