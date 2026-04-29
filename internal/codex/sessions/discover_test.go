package sessions

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverFiltersDateRangeAndLimit(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "codex")
	since := mustDate(t, "2026-04-29")
	until := mustDate(t, "2026-04-29")
	files, err := Discover(ScanOptions{
		Root:  root,
		Since: &since,
		Until: &until,
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].ID != "session-one" {
		t.Fatalf("expected deterministic first file, got %q", files[0].ID)
	}
	if files[0].Date.Format(time.DateOnly) != "2026-04-29" {
		t.Fatalf("unexpected date: %s", files[0].Date.Format(time.DateOnly))
	}
}

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
