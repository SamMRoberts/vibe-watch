package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/SamMRoberts/vibe-watch/internal/sessions"
	"github.com/SamMRoberts/vibe-watch/internal/watch"
)

func TestModelHandlesProgressiveBatchAndTabSelection(t *testing.T) {
	model := NewModel(Options{SessionsRoot: t.TempDir(), BatchSize: 2})
	model.width = 100
	model.height = 30
	model.resize()
	started := time.Date(2026, 4, 29, 14, 0, 0, 0, time.UTC)
	msg := batchLoadedMsg{batch: watch.Batch{
		Sessions: []sessions.Summary{{
			ID:            "rollout-a",
			Agent:         "codex",
			StartedAt:     started,
			UpdatedAt:     started,
			Status:        sessions.StatusCompleted,
			PromptPreview: "Build the dashboard",
			ToolCallCount: 2,
			Tokens: sessions.TokenSummary{
				Known: true,
				Total: sessions.TokenUsage{Known: true, TotalTokens: 42},
			},
			Activities: []sessions.Activity{{Kind: sessions.ActivityUserPrompt, Title: "User prompt", Body: "Build the dashboard"}},
		}},
		Offset: 0,
		Total:  1,
		Done:   true,
	}}
	updated, _ := model.Update(msg)
	model = updated.(Model)
	if len(model.sessions) != 1 {
		t.Fatalf("sessions = %d", len(model.sessions))
	}
	if !strings.Contains(model.View().Content, "completed: 1") {
		t.Fatalf("dashboard did not render counts:\n%s", model.View().Content)
	}

	updated, _ = model.Update(keyPress(tea.KeyTab, ""))
	model = updated.(Model)
	if model.activeTab != tabSessions {
		t.Fatalf("activeTab = %d", model.activeTab)
	}
	updated, _ = model.Update(keyPress(tea.KeyEnter, ""))
	model = updated.(Model)
	if !model.detailMode {
		t.Fatal("expected detail mode after enter")
	}
	if !strings.Contains(model.View().Content, "Build the dashboard") {
		t.Fatalf("detail view missing activity:\n%s", model.View().Content)
	}
}

func TestModelMouseWheelScrollsDetailViewport(t *testing.T) {
	model := NewModel(Options{SessionsRoot: t.TempDir()})
	model.width = 80
	model.height = 12
	model.resize()
	activities := make([]sessions.Activity, 0, 40)
	for i := 0; i < 40; i++ {
		activities = append(activities, sessions.Activity{Kind: sessions.ActivityAssistantMessage, Title: "Assistant", Body: "line"})
	}
	summary := sessions.Summary{
		ID:         "rollout-scroll",
		Agent:      "codex",
		StartedAt:  time.Date(2026, 4, 29, 14, 0, 0, 0, time.UTC),
		Status:     sessions.StatusActive,
		Activities: activities,
	}
	updated, _ := model.Update(batchLoadedMsg{batch: watch.Batch{Sessions: []sessions.Summary{summary}, Total: 1, Done: true}})
	model = updated.(Model)
	model.activeTab = tabSessions
	model.updateListItems()
	updated, _ = model.Update(keyPress(tea.KeyEnter, ""))
	model = updated.(Model)
	if !model.detailMode {
		t.Fatal("expected detail mode")
	}
	before := model.detail.YOffset()
	updated, _ = model.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	model = updated.(Model)
	after := model.detail.YOffset()
	if after <= before {
		t.Fatalf("mouse wheel did not scroll detail viewport: before=%d after=%d", before, after)
	}
}

func TestModelResizeAndThemeToggle(t *testing.T) {
	model := NewModel(Options{Theme: "dark"})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = updated.(Model)
	if model.width != 120 || model.height != 40 {
		t.Fatalf("resize not applied: %dx%d", model.width, model.height)
	}
	updated, _ = model.Update(keyPress(0, "t"))
	model = updated.(Model)
	if model.themeName != "light" {
		t.Fatalf("themeName = %q", model.themeName)
	}
}

func keyPress(code rune, text string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Text: text})
}
