package watcher

import (
	"path/filepath"
	"testing"

	"github.com/SamMRoberts/vibe-watch/internal/codex/events"
)

func TestCaptureSummarizesActiveSessionWithoutRawContent(t *testing.T) {
	snapshot, err := Capture(Options{
		Root:       filepath.Join("..", "..", "testdata", "codex"),
		EventLimit: 2,
	})
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	if len(snapshot.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(snapshot.Sessions))
	}
	if snapshot.Active == nil {
		t.Fatal("expected active session")
	}
	if snapshot.Active.Agent != "Codex" {
		t.Fatalf("expected Codex agent, got %q", snapshot.Active.Agent)
	}
	if snapshot.Active.Status != "active" {
		t.Fatalf("expected active status, got %q", snapshot.Active.Status)
	}
	if snapshot.Active.RepoPath == "" {
		t.Fatal("expected repo path metadata")
	}
	if snapshot.Active.LastEventType == "" {
		t.Fatal("expected last event type metadata")
	}
	if _, ok := snapshot.Details[snapshot.Active.ID]; !ok {
		t.Fatalf("expected details for active session %q", snapshot.Active.ID)
	}
	if len(snapshot.Active.Content) == 0 {
		t.Fatal("expected bounded detail content")
	}
	if len(snapshot.Active.Recent) != 2 {
		t.Fatalf("expected 2 recent events, got %d", len(snapshot.Active.Recent))
	}
	for _, event := range snapshot.Active.Recent {
		if event.Type == "" {
			t.Fatalf("expected event type for line %d", event.Line)
		}
	}
}

func TestCaptureEmptyRoot(t *testing.T) {
	dir := t.TempDir()
	snapshot, err := Capture(Options{Root: dir})
	if err != nil {
		t.Fatalf("Capture returned error: %v", err)
	}
	if len(snapshot.Sessions) != 0 {
		t.Fatalf("expected no sessions, got %d", len(snapshot.Sessions))
	}
	if snapshot.Active != nil {
		t.Fatal("expected no active session")
	}
}

func TestSummarizeExtractsDetailContent(t *testing.T) {
	cases := []struct {
		name      string
		raw       map[string]any
		wantKind  string
		wantActor string
		wantText  string
	}{
		{
			name: "user prompt",
			raw: map[string]any{
				"type":    "message",
				"payload": map[string]any{"role": "user", "content": "Build the monitor view."},
			},
			wantKind:  "user prompt",
			wantActor: "user",
			wantText:  "Build the monitor view.",
		},
		{
			name: "assistant",
			raw: map[string]any{
				"type":    "message",
				"payload": map[string]any{"role": "assistant", "content": "I will update the TUI."},
			},
			wantKind:  "assistant",
			wantActor: "assistant",
			wantText:  "I will update the TUI.",
		},
		{
			name: "reasoning",
			raw: map[string]any{
				"type":    "reasoning",
				"payload": map[string]any{"summary": "Need bounded content extraction."},
			},
			wantKind:  "reasoning",
			wantActor: "agent",
			wantText:  "Need bounded content extraction.",
		},
		{
			name: "tool call",
			raw: map[string]any{
				"type":           "tool_call",
				"recipient_name": "functions.exec_command",
				"payload":        map[string]any{"cmd": "go test ./..."},
			},
			wantKind:  "tool call",
			wantActor: "tool",
			wantText:  "functions.exec_command go test ./...",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary := Summarize(events.Event{Line: 1, Raw: tc.raw})
			if summary.Kind != tc.wantKind {
				t.Fatalf("expected kind %q, got %q", tc.wantKind, summary.Kind)
			}
			if summary.Actor != tc.wantActor {
				t.Fatalf("expected actor %q, got %q", tc.wantActor, summary.Actor)
			}
			if summary.Text != tc.wantText {
				t.Fatalf("expected text %q, got %q", tc.wantText, summary.Text)
			}
		})
	}
}
