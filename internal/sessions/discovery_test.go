package sessions

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverSortsNewestFirstAndAppliesLimit(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	older := writeSessionFile(t, root, "2026", "04", "28", "rollout-2026-04-28T10-00-00-old.jsonl", now.Add(-24*time.Hour))
	newerA := writeSessionFile(t, root, "2026", "04", "29", "rollout-2026-04-29T10-00-00-a.jsonl", now.Add(-2*time.Hour))
	newerB := writeSessionFile(t, root, "2026", "04", "29", "rollout-2026-04-29T11-00-00-b.jsonl", now.Add(-1*time.Hour))

	files, err := Discover(DiscoverOptions{Root: root, Days: 2, Limit: 2, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("len(files) = %d", len(files))
	}
	if files[0].Path != newerB || files[1].Path != newerA {
		t.Fatalf("unexpected order: %#v, older=%s", files, older)
	}
}

func TestDiscoverHonorsDaysWindowAndResolveFindsIDs(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	writeSessionFile(t, root, "2026", "04", "27", "rollout-2026-04-27T10-00-00-old.jsonl", now.Add(-48*time.Hour))
	recent := writeSessionFile(t, root, "2026", "04", "29", "rollout-2026-04-29T10-00-00-recent.jsonl", now.Add(-1*time.Hour))

	files, err := Discover(DiscoverOptions{Root: root, Days: 1, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != recent {
		t.Fatalf("unexpected files: %#v", files)
	}
	resolved, err := Resolve(root, "rollout-2026-04-29T10-00-00-recent", DiscoverOptions{Root: root, Days: 1, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if resolved != recent {
		t.Fatalf("resolved = %s, want %s", resolved, recent)
	}
}

func writeSessionFile(t *testing.T, root, year, month, day, name string, mod time.Time) string {
	t.Helper()
	dir := filepath.Join(root, year, month, day)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
	return path
}
