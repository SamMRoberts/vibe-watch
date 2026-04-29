package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/watcher"
	tea "github.com/charmbracelet/bubbletea"
)

func TestModelLoadsSnapshotAndRendersActiveSession(t *testing.T) {
	model := NewModel(Options{
		Interval: time.Hour,
		Loader: func() (watcher.Snapshot, error) {
			return watcher.Snapshot{
				Root:      "testdata/codex",
				CheckedAt: time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
				Sessions:  []watcher.SessionSummary{{ID: "session-one"}},
				Active: &watcher.SessionDetail{
					SessionSummary: watcher.SessionSummary{ID: "session-one", Events: 1, Size: 42},
					Recent: []watcher.EventSummary{{
						Line: 1,
						Type: "session_meta",
						Tool: "functions.exec_command",
					}},
				},
			}, nil
		},
	})
	msg := model.Init()()
	next, _ := model.Update(msg)
	updated := next.(Model)
	view := updated.View()
	if !strings.Contains(view, "session-one") {
		t.Fatalf("expected active session in view, got %q", view)
	}
	if !strings.Contains(view, "Sessions") {
		t.Fatalf("expected sessions tab in view, got %q", view)
	}
	next, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated = next.(Model)
	if updated.tab != tabDetail {
		t.Fatalf("expected detail tab, got %d", updated.tab)
	}
	if !strings.Contains(updated.View(), "session_meta") {
		t.Fatalf("expected event type in detail view, got %q", updated.View())
	}
}

func TestModelShowsLoaderError(t *testing.T) {
	model := NewModel(Options{
		Interval: time.Hour,
		Loader: func() (watcher.Snapshot, error) {
			return watcher.Snapshot{}, errors.New("boom")
		},
	})
	msg := model.Init()()
	next, _ := model.Update(msg)
	updated := next.(Model)
	if !strings.Contains(updated.View(), "watch error: boom") {
		t.Fatalf("expected error in view, got %q", updated.View())
	}
}

func TestModelQuitsOnQ(t *testing.T) {
	model := NewModel(Options{})
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestRenderSnapshot(t *testing.T) {
	view := RenderSnapshot(watcher.Snapshot{
		Root:      "testdata/codex",
		CheckedAt: time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
		Sessions:  []watcher.SessionSummary{{ID: "session-one", Status: "active", Agent: "Codex"}},
		Active: &watcher.SessionDetail{
			SessionSummary: watcher.SessionSummary{ID: "session-one"},
		},
	})
	if !strings.Contains(view, "session-one") {
		t.Fatalf("expected rendered session, got %q", view)
	}
}

func TestModelSelectsSessionWithArrowKeys(t *testing.T) {
	model := NewModel(Options{})
	activeDetail := watcher.SessionDetail{SessionSummary: watcher.SessionSummary{ID: "session-two"}}
	next, _ := model.Update(snapshotMsg{snapshot: watcher.Snapshot{
		Sessions: []watcher.SessionSummary{
			{ID: "session-one", Status: "idle", Agent: "Codex"},
			{ID: "session-two", Status: "active", Agent: "Codex"},
		},
		Active: &activeDetail,
		Details: map[string]watcher.SessionDetail{
			"session-one": {SessionSummary: watcher.SessionSummary{ID: "session-one"}},
			"session-two": {SessionSummary: watcher.SessionSummary{ID: "session-two"}},
		},
	}})
	updated := next.(Model)
	if updated.selected != 1 {
		t.Fatalf("expected active session selected by default, got %d", updated.selected)
	}
	next, _ = updated.Update(tea.KeyMsg{Type: tea.KeyUp})
	updated = next.(Model)
	if updated.selected != 0 {
		t.Fatalf("expected selection to move up, got %d", updated.selected)
	}
	next, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated = next.(Model)
	if updated.tab != tabDetail || updated.detailID != "session-one" {
		t.Fatalf("expected detail tab for session-one, got tab=%d detail=%q", updated.tab, updated.detailID)
	}
}
