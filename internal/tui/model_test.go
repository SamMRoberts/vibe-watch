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
	if !strings.Contains(view, "session_meta") {
		t.Fatalf("expected event type in view, got %q", view)
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
		Active: &watcher.SessionDetail{
			SessionSummary: watcher.SessionSummary{ID: "session-one"},
		},
	})
	if !strings.Contains(view, "session-one") {
		t.Fatalf("expected rendered session, got %q", view)
	}
}
