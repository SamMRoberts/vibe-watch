package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootHelp(t *testing.T) {
	command := newRootCommand()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&out)
	command.SetArgs([]string{"--help"})
	if err := command.Execute(); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	if !strings.Contains(out.String(), "vibe-watch") {
		t.Fatalf("expected help to mention vibe-watch, got %q", out.String())
	}
}

func TestStatsCommandWithSyntheticRoot(t *testing.T) {
	command := newRootCommand()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&out)
	root := filepath.Join("..", "testdata", "codex")
	command.SetArgs([]string{"--session-root", root, "stats"})
	if err := command.Execute(); err != nil {
		t.Fatalf("stats returned error: %v", err)
	}
	if !strings.Contains(out.String(), "Metrics") {
		t.Fatalf("expected metrics output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "vibe-watch") {
		t.Fatalf("expected repo aggregate in output, got %q", out.String())
	}
}

func TestInvalidDateReturnsError(t *testing.T) {
	command := newRootCommand()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&out)
	command.SetArgs([]string{"--since", "04/29/2026", "scan"})
	if err := command.Execute(); err == nil {
		t.Fatal("expected invalid date error")
	}
}

func TestTUIHelp(t *testing.T) {
	command := newRootCommand()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&out)
	command.SetArgs([]string{"tui", "--help"})
	if err := command.Execute(); err != nil {
		t.Fatalf("tui help returned error: %v", err)
	}
	if !strings.Contains(out.String(), "--interval") {
		t.Fatalf("expected interval flag in help, got %q", out.String())
	}
	if !strings.Contains(out.String(), "--event-limit") {
		t.Fatalf("expected event-limit flag in help, got %q", out.String())
	}
	if !strings.Contains(out.String(), "--once") {
		t.Fatalf("expected once flag in help, got %q", out.String())
	}
}

func TestTUIOnceWithSyntheticRoot(t *testing.T) {
	command := newRootCommand()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&out)
	command.SetArgs([]string{"--session-root", filepath.Join("..", "testdata", "codex"), "tui", "--once", "--event-limit", "2"})
	if err := command.Execute(); err != nil {
		t.Fatalf("tui once returned error: %v", err)
	}
	if !strings.Contains(out.String(), "vibe-watch") {
		t.Fatalf("expected rendered TUI output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Recent events") {
		t.Fatalf("expected recent events section, got %q", out.String())
	}
}
