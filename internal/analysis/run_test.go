package analysis

import (
	"path/filepath"
	"testing"

	"github.com/SamMRoberts/vibe-watch/internal/codex/sessions"
)

func TestRunAggregatesSyntheticCodexFixtures(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "codex")
	result, err := Run(sessions.ScanOptions{Root: root})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Summary.SessionsScanned != 2 {
		t.Fatalf("expected 2 sessions, got %d", result.Summary.SessionsScanned)
	}
	if result.Summary.MalformedLines != 1 {
		t.Fatalf("expected 1 malformed line, got %d", result.Summary.MalformedLines)
	}
	if result.Metrics.SessionsByDay["2026-04-29"] != 2 {
		t.Fatalf("expected 2 sessions on 2026-04-29, got %d", result.Metrics.SessionsByDay["2026-04-29"])
	}
	if result.Metrics.Models["gpt-5.4"] != 2 {
		t.Fatalf("expected model count 2, got %d", result.Metrics.Models["gpt-5.4"])
	}
	if result.Metrics.Repositories["vibe-watch"] != 2 {
		t.Fatalf("expected repo count 2, got %d", result.Metrics.Repositories["vibe-watch"])
	}
	if result.Metrics.ShellCommands != 2 {
		t.Fatalf("expected 2 shell command events, got %d", result.Metrics.ShellCommands)
	}
	if result.Metrics.FailedShellCommands != 1 {
		t.Fatalf("expected 1 failed shell command, got %d", result.Metrics.FailedShellCommands)
	}
	if result.Metrics.ApprovalEvents != 1 {
		t.Fatalf("expected 1 approval event, got %d", result.Metrics.ApprovalEvents)
	}
	if len(result.Suggestions) == 0 {
		t.Fatal("expected suggestions")
	}
}

func TestSuggestReturnsNoSessionHint(t *testing.T) {
	suggestions := Suggest(ScanSummary{}, Metrics{})
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	if suggestions[0].Severity != "info" {
		t.Fatalf("expected info suggestion, got %q", suggestions[0].Severity)
	}
}
