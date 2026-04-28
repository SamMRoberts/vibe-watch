package watcher

import (
	"testing"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

func TestFilterSessionsByProject(t *testing.T) {
	projectRoot := "/Users/sam/Repo/vibe-watch"
	sessions := []*models.Session{
		{ProjectPath: projectRoot},
		{ProjectPath: projectRoot + "/cmd"},
		{ProjectPath: "/Users/sam/Repo/other"},
		{ProjectPath: "vibe-watch"},
		{ProjectPath: "-Users-sam-Repo-vibe-watch"},
		{ProjectPath: "unrelated"},
		{ProjectPath: ""},
	}

	filtered := filterSessionsByProject(sessions, projectRoot)
	if len(filtered) != 4 {
		t.Fatalf("expected 4 matching sessions, got %d", len(filtered))
	}

	for _, session := range filtered {
		switch session.ProjectPath {
		case projectRoot, projectRoot + "/cmd", "vibe-watch", "-Users-sam-Repo-vibe-watch":
		default:
			t.Fatalf("unexpected session matched project filter: %q", session.ProjectPath)
		}
	}
}

func TestFilterSessionsByProjectNoRoot(t *testing.T) {
	sessions := []*models.Session{{ProjectPath: "one"}, {ProjectPath: "two"}}
	filtered := filterSessionsByProject(sessions, "")
	if len(filtered) != len(sessions) {
		t.Fatalf("expected no filtering without a project root, got %d sessions", len(filtered))
	}
}
