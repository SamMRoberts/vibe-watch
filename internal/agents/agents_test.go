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

func TestCopilotDetectorParsesSessionState(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	sessionDir := filepath.Join(tmp, ".copilot", "session-state", "session-123")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	workspace := `id: session-123
cwd: /tmp/project
git_root: /tmp/project
summary: Test Project
created_at: 2026-04-14T21:05:20.436Z
updated_at: 2026-04-14T21:08:03.055Z
`
	if err := os.WriteFile(filepath.Join(sessionDir, "workspace.yaml"), []byte(workspace), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []map[string]interface{}{
		{
			"type":      "user.message",
			"timestamp": "2026-04-14T21:07:48.374Z",
			"data": map[string]interface{}{
				"content": "hello copilot",
			},
		},
		{
			"type":      "assistant.message",
			"timestamp": "2026-04-14T21:07:54.053Z",
			"data": map[string]interface{}{
				"content":      "hello human",
				"outputTokens": 12,
			},
		},
		{
			"type":      "session.shutdown",
			"timestamp": "2026-04-14T21:17:34.021Z",
			"data": map[string]interface{}{
				"modelMetrics": map[string]interface{}{
					"claude-sonnet-4.6": map[string]interface{}{
						"usage": map[string]interface{}{
							"inputTokens":      100,
							"outputTokens":     50,
							"cacheReadTokens":  20,
							"cacheWriteTokens": 10,
						},
					},
				},
			},
		},
	}
	writeJSONL(t, filepath.Join(sessionDir, "events.jsonl"), entries)

	d := agents.NewCopilotDetector()
	sessions, err := d.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	s := sessions[0]
	if s.ID != "session-123" {
		t.Errorf("expected session ID from workspace, got %q", s.ID)
	}
	if s.AgentType != models.AgentCopilot {
		t.Errorf("expected AgentCopilot, got %q", s.AgentType)
	}
	if s.ProjectPath != "/tmp/project" {
		t.Errorf("expected project path from git_root, got %q", s.ProjectPath)
	}
	if len(s.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(s.Messages))
	}
	if s.Messages[0].Role != "user" || s.Messages[0].Content != "hello copilot" {
		t.Errorf("unexpected user message: %#v", s.Messages[0])
	}
	if s.Messages[1].Role != "assistant" || s.Messages[1].Content != "hello human" {
		t.Errorf("unexpected assistant message: %#v", s.Messages[1])
	}
	if s.TotalTokens.InputTokens != 100 {
		t.Errorf("expected 100 input tokens, got %d", s.TotalTokens.InputTokens)
	}
	if s.TotalTokens.OutputTokens != 50 {
		t.Errorf("expected 50 output tokens, got %d", s.TotalTokens.OutputTokens)
	}
	if s.TotalTokens.CacheReads != 20 {
		t.Errorf("expected 20 cache reads, got %d", s.TotalTokens.CacheReads)
	}
	if s.TotalTokens.CacheWrites != 10 {
		t.Errorf("expected 10 cache writes, got %d", s.TotalTokens.CacheWrites)
	}
}

func TestCopilotDetectorUsesShutdownCurrentTokensFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	sessionDir := filepath.Join(tmp, ".copilot", "session-state", "abc")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	workspace := `id: session-abc
cwd: /tmp/project
created_at: 2026-04-14T21:05:20.436Z
updated_at: 2026-04-14T21:08:03.055Z
`
	if err := os.WriteFile(filepath.Join(sessionDir, "workspace.yaml"), []byte(workspace), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []map[string]interface{}{
		{
			"type":      "assistant.message",
			"timestamp": "2026-04-14T21:07:54.053Z",
			"data": map[string]interface{}{
				"content":      "hello human",
				"outputTokens": 12,
			},
		},
		{
			"type":      "session.shutdown",
			"timestamp": "2026-04-14T21:17:34.021Z",
			"data": map[string]interface{}{
				"currentTokens": 12345,
			},
		},
	}
	writeJSONL(t, filepath.Join(sessionDir, "events.jsonl"), entries)

	d := agents.NewCopilotDetector()
	sessions, err := d.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if got := sessions[0].TotalTokens.InputTokens; got != 12345 {
		t.Fatalf("expected currentTokens fallback for input tokens, got %d", got)
	}
	if got := sessions[0].TotalTokens.OutputTokens; got != 12 {
		t.Fatalf("expected assistant output tokens to be preserved, got %d", got)
	}
}

func TestCopilotDetectorNonExistentDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	d := agents.NewCopilotDetector()
	sessions, err := d.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
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
	if len(dets) != 5 {
		t.Errorf("expected 5 detectors, got %d", len(dets))
	}
	names := map[string]bool{}
	for _, d := range dets {
		names[d.Name()] = true
	}
	for _, want := range []string{"Claude Code", "Codex CLI", "Copilot CLI", "Copilot Chat", "Amazon Q"} {
		if !names[want] {
			t.Errorf("missing detector for %q", want)
		}
	}
}
