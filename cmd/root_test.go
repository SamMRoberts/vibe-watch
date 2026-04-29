package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootHelpAndInvalidFlags(t *testing.T) {
	helpOut, err := executeForTest(NewRootCommand(func(context.Context, Config) error { return nil }), "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(helpOut, "--sessions-root") || !strings.Contains(helpOut, "sessions") {
		t.Fatalf("help missing expected content:\n%s", helpOut)
	}

	_, err = executeForTest(NewRootCommand(func(context.Context, Config) error { return nil }), "--days", "-1")
	if err == nil || !strings.Contains(err.Error(), "--days") {
		t.Fatalf("expected invalid days error, got %v", err)
	}
}

func TestRootRunsTUIByDefault(t *testing.T) {
	called := false
	root := NewRootCommand(func(ctx context.Context, cfg Config) error {
		called = true
		if cfg.Days != 7 || cfg.Limit != 200 || cfg.Theme != "auto" {
			t.Fatalf("unexpected defaults: %#v", cfg)
		}
		return nil
	})
	if _, err := executeForTest(root, "--sessions-root", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected root command to launch TUI")
	}
}

func TestSessionsListAndInspectJSON(t *testing.T) {
	rootDir := t.TempDir()
	sessionDir := filepath.Join(rootDir, "2026", "04", "29")
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(sessionDir, "rollout-2026-04-29T14-00-00-fixture.jsonl")
	fixture, err := os.ReadFile(filepath.Join("..", "testdata", "codex", "complete.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionPath, fixture, 0o600); err != nil {
		t.Fatal(err)
	}

	listOut, err := executeForTest(NewRootCommand(func(context.Context, Config) error { return nil }), "--sessions-root", rootDir, "sessions", "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var summaries []map[string]any
	if err := json.Unmarshal([]byte(listOut), &summaries); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, listOut)
	}
	if len(summaries) != 1 || summaries[0]["id"] != "rollout-2026-04-29T14-00-00-fixture" {
		t.Fatalf("unexpected summaries: %#v", summaries)
	}

	inspectOut, err := executeForTest(NewRootCommand(func(context.Context, Config) error { return nil }), "--sessions-root", rootDir, "sessions", "inspect", "rollout-2026-04-29T14-00-00-fixture", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var summary map[string]any
	if err := json.Unmarshal([]byte(inspectOut), &summary); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, inspectOut)
	}
	if summary["status"] != "completed" {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}

func executeForTest(cmd *cobra.Command, args ...string) (string, error) {
	var out bytes.Buffer
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}
