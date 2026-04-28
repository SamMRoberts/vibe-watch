package agents_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/agents"
	"github.com/SamMRoberts/vibe-watch/internal/models"
)

// writeJSONL writes a slice of maps as JSONL to a file.
func writeJSONL(t *testing.T, path string, entries []map[string]interface{}) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			t.Fatalf("encode entry: %v", err)
		}
	}
}

func TestClaudeDetectorNonExistentDir(t *testing.T) {
	// Point home somewhere without .claude
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	d := agents.NewClaudeDetector()
	sessions, err := d.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestClaudeDetectorParsesJSONL(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create ~/.claude/projects/myproject/session.jsonl
	projectDir := filepath.Join(tmp, ".claude", "projects", "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	entries := []map[string]interface{}{
		{
			"type":      "user",
			"timestamp": now.Format(time.RFC3339),
			"message": map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "text", "text": "hello"},
				},
			},
		},
		{
			"type":      "assistant",
			"timestamp": now.Add(time.Second).Format(time.RFC3339),
			"message": map[string]interface{}{
				"role": "assistant",
				"content": []map[string]interface{}{
					{"type": "text", "text": "world"},
				},
				"usage": map[string]interface{}{
					"input_tokens":  100,
					"output_tokens": 50,
				},
			},
		},
	}
	writeJSONL(t, filepath.Join(projectDir, "session.jsonl"), entries)

	d := agents.NewClaudeDetector()
	sessions, err := d.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	s := sessions[0]
	if s.AgentType != models.AgentClaude {
		t.Errorf("expected AgentClaude, got %q", s.AgentType)
	}
	if s.ProjectPath != "myproject" {
		t.Errorf("expected project 'myproject', got %q", s.ProjectPath)
	}
	if s.TotalTokens.InputTokens != 100 {
		t.Errorf("expected 100 input tokens, got %d", s.TotalTokens.InputTokens)
	}
	if s.TotalTokens.OutputTokens != 50 {
		t.Errorf("expected 50 output tokens, got %d", s.TotalTokens.OutputTokens)
	}
	if len(s.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(s.Messages))
	}
}

func TestRegistryDetectAll(t *testing.T) {
	// Ensure none of the detectors panic on an empty home dir
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	r := agents.NewRegistry()
	sessions, err := r.DetectAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty dirs → 0 sessions; no panic
	if sessions == nil {
		sessions = []*models.Session{} // nil is fine too
	}
	_ = sessions
}

func TestRegistryDetectors(t *testing.T) {
	r := agents.NewRegistry()
	dets := r.Detectors()
	if len(dets) != 4 {
		t.Errorf("expected 4 detectors, got %d", len(dets))
	}
	names := map[string]bool{}
	for _, d := range dets {
		names[d.Name()] = true
	}
	for _, want := range []string{"Claude Code", "Codex CLI", "Copilot CLI", "Amazon Q"} {
		if !names[want] {
			t.Errorf("missing detector for %q", want)
		}
	}
}
