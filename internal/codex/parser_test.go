package codex

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/sessions"
)

func TestParseFileNormalizesCodexJSONL(t *testing.T) {
	now := time.Date(2026, 4, 29, 15, 0, 0, 0, time.UTC)
	path := filepath.Join("..", "..", "testdata", "codex", "complete.jsonl")
	summary, err := ParseFile(path, now)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ID != "complete" {
		t.Fatalf("ID = %q, want filename-derived complete", summary.ID)
	}
	if summary.Agent != "codex" {
		t.Fatalf("Agent = %q", summary.Agent)
	}
	if summary.Status != sessions.StatusCompleted {
		t.Fatalf("Status = %q", summary.Status)
	}
	if summary.PromptPreview != "Build a local TUI dashboard" {
		t.Fatalf("PromptPreview = %q", summary.PromptPreview)
	}
	if summary.ToolCallCount != 1 {
		t.Fatalf("ToolCallCount = %d", summary.ToolCallCount)
	}
	if !summary.Tokens.Known || summary.Tokens.Total.TotalTokens != 16 || summary.Tokens.Latest.TotalTokens != 5 {
		t.Fatalf("unexpected tokens: %#v", summary.Tokens)
	}
	requireActivity(t, summary, sessions.ActivityUserPrompt)
	requireActivity(t, summary, sessions.ActivityAssistantMessage)
	requireActivity(t, summary, sessions.ActivityToolCall)
	requireActivity(t, summary, sessions.ActivityToolResult)
	requireActivity(t, summary, sessions.ActivityTokenCount)
	requireActivity(t, summary, sessions.ActivityOther)
}

func TestParseFileReportsMalformedLinesAndZeroTokens(t *testing.T) {
	now := time.Date(2026, 4, 29, 15, 0, 0, 0, time.UTC)
	path := filepath.Join("..", "..", "testdata", "codex", "malformed.jsonl")
	summary, err := ParseFile(path, now)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != sessions.StatusErrored {
		t.Fatalf("Status = %q", summary.Status)
	}
	if len(summary.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %d", len(summary.Diagnostics))
	}
	if !summary.Tokens.Known || summary.Tokens.Total.TotalTokens != 0 {
		t.Fatalf("zero token values should remain known: %#v", summary.Tokens)
	}
}

func TestParseFileInfersActiveForRecentlyModifiedIncompleteSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout-2026-04-29T14-00-00-active.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"event_msg","timestamp":"2026-04-29T14:00:02Z","payload":{"type":"user_message","message":"still going"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 4, 29, 14, 10, 0, 0, time.UTC)
	if err := os.Chtimes(path, now, now.Add(-5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	summary, err := ParseFile(path, now)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != sessions.StatusActive {
		t.Fatalf("Status = %q", summary.Status)
	}
}

func requireActivity(t *testing.T, summary sessions.Summary, kind sessions.ActivityKind) {
	t.Helper()
	for _, activity := range summary.Activities {
		if activity.Kind == kind {
			return
		}
	}
	t.Fatalf("missing activity kind %s in %#v", kind, summary.Activities)
}
