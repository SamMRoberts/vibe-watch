package agents_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	if len(s.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(s.Messages))
	}
	if s.Messages[0].Role != "user" || s.Messages[0].Content != "hello copilot" {
		t.Errorf("unexpected user message: %#v", s.Messages[0])
	}
	if s.Messages[1].Role != "assistant" || s.Messages[1].Content != "hello human" {
		t.Errorf("unexpected assistant message: %#v", s.Messages[1])
	}
	if s.Messages[2].Role != "session" || !strings.Contains(s.Messages[2].Content, "Session shutdown") {
		t.Errorf("unexpected shutdown activity: %#v", s.Messages[2])
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

func TestCopilotDetectorParsesActivityEvents(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	sessionDir := filepath.Join(tmp, ".copilot", "session-state", "activity")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	workspace := `id: session-activity
cwd: /tmp/project
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
				"content": "first prompt",
			},
		},
		{
			"type":      "tool.execution_start",
			"timestamp": "2026-04-14T21:07:49.000Z",
			"data": map[string]interface{}{
				"toolCallId": "tool-1",
				"toolName":   "bash",
			},
		},
		{
			"type":      "tool.execution_complete",
			"timestamp": "2026-04-14T21:07:50.000Z",
			"data": map[string]interface{}{
				"toolCallId": "tool-1",
				"success":    false,
				"model":      "gpt-5.5",
				"error":      "permission denied",
				"toolTelemetry": map[string]interface{}{
					"metrics": map[string]interface{}{
						"resultLength": 12,
					},
				},
			},
		},
		{
			"type":      "subagent.completed",
			"timestamp": "2026-04-14T21:07:51.000Z",
			"data": map[string]interface{}{
				"agentDisplayName": "Reviewer",
				"model":            "claude-haiku-4.5",
				"durationMs":       1200,
				"totalTokens":      3456,
				"totalToolCalls":   7,
			},
		},
		{
			"type":      "session.model_change",
			"timestamp": "2026-04-14T21:07:52.000Z",
			"data": map[string]interface{}{
				"previousModel": "gpt-5.4",
				"newModel":      "gpt-5.5",
			},
		},
		{
			"type":      "session.mode_changed",
			"timestamp": "2026-04-14T21:07:53.000Z",
			"data": map[string]interface{}{
				"previousMode": "plan",
				"newMode":      "autopilot",
			},
		},
		{
			"type":      "session.task_complete",
			"timestamp": "2026-04-14T21:07:54.000Z",
			"data": map[string]interface{}{
				"success": true,
				"summary": "done",
			},
		},
		{
			"type":      "session.error",
			"timestamp": "2026-04-14T21:07:55.000Z",
			"data": map[string]interface{}{
				"errorType": "runtime",
				"message":   "boom",
			},
		},
		{
			"type":      "user.message",
			"timestamp": "2026-04-14T21:08:00.000Z",
			"data": map[string]interface{}{
				"content": "second prompt",
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

	messages := sessions[0].Messages
	if len(messages) != 9 {
		t.Fatalf("expected 9 messages/activity entries, got %d: %#v", len(messages), messages)
	}
	expectedRoles := []string{"user", "tool", "tool", "subagent", "session", "session", "session", "error", "user"}
	for i, role := range expectedRoles {
		if messages[i].Role != role {
			t.Fatalf("message %d role = %q, want %q", i, messages[i].Role, role)
		}
	}
	if !strings.Contains(messages[2].Content, "Tool failed: bash") ||
		!strings.Contains(messages[2].Content, "permission denied") ||
		!strings.Contains(messages[2].Content, "resultLength:12") {
		t.Fatalf("expected useful tool completion content, got %q", messages[2].Content)
	}
	if !strings.Contains(messages[3].Content, "Completed subagent: Reviewer") ||
		!strings.Contains(messages[3].Content, "tokens: 3456") {
		t.Fatalf("expected subagent telemetry, got %q", messages[3].Content)
	}
	if !strings.Contains(messages[4].Content, "gpt-5.4 -> gpt-5.5") {
		t.Fatalf("expected model change content, got %q", messages[4].Content)
	}
	if !strings.Contains(messages[6].Content, "done") {
		t.Fatalf("expected task summary, got %q", messages[6].Content)
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
	if got := sessions[0].Messages[len(sessions[0].Messages)-1].Content; !strings.Contains(got, "context tokens: current:12345") {
		t.Fatalf("expected shutdown context token summary, got %q", got)
	}
}

func TestCopilotDetectorUsesCompactionTokenUsage(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	sessionDir := filepath.Join(tmp, ".copilot", "session-state", "compaction")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	workspace := `id: session-compaction
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
			"type":      "session.compaction_complete",
			"timestamp": "2026-04-14T21:17:34.021Z",
			"data": map[string]interface{}{
				"success":             true,
				"preCompactionTokens": 219977,
				"checkpointNumber":    1,
				"compactionTokensUsed": map[string]interface{}{
					"inputTokens":      212569,
					"outputTokens":     4152,
					"cacheReadTokens":  193024,
					"cacheWriteTokens": 7,
					"model":            "gpt-5.5",
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
	if got := s.TotalTokens.InputTokens; got != 212569 {
		t.Fatalf("expected compaction input tokens, got %d", got)
	}
	if got := s.TotalTokens.OutputTokens; got != 4164 {
		t.Fatalf("expected assistant and compaction output tokens, got %d", got)
	}
	if got := s.TotalTokens.CacheReads; got != 193024 {
		t.Fatalf("expected compaction cache read tokens, got %d", got)
	}
	if got := s.TotalTokens.CacheWrites; got != 7 {
		t.Fatalf("expected compaction cache write tokens, got %d", got)
	}
	last := s.Messages[len(s.Messages)-1]
	if last.Role != "session" || last.Tokens.InputTokens != 212569 || !strings.Contains(last.Content, "cache read:193024") {
		t.Fatalf("expected compaction activity with token usage, got %#v", last)
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
