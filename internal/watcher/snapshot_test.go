package watcher

import (
	"path/filepath"
	"testing"
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
