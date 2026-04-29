package watcher

import (
	"testing"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

func TestWatcherPublishesRefreshLifecycleMessages(t *testing.T) {
	w := &Watcher{updates: make(chan UpdateMsg, 3)}

	w.publishStatus(true)
	w.publish([]*models.Session{{ID: "partial"}}, nil, true, false)
	w.publish([]*models.Session{{ID: "final"}}, nil, false, true)

	start := <-w.Updates()
	if !start.Refreshing || start.HasSessions || start.Done {
		t.Fatalf("expected refresh start status, got %#v", start)
	}

	partial := <-w.Updates()
	if !partial.Refreshing || !partial.HasSessions || partial.Done || partial.Sessions[0].ID != "partial" {
		t.Fatalf("expected refreshing partial sessions update, got %#v", partial)
	}

	final := <-w.Updates()
	if final.Refreshing || !final.HasSessions || !final.Done || final.Sessions[0].ID != "final" {
		t.Fatalf("expected final completed sessions update, got %#v", final)
	}
}

func TestStaticWatcherPublishesCompletedSessionUpdate(t *testing.T) {
	session := &models.Session{ID: "static"}
	w := NewStatic([]*models.Session{session})
	w.Start()
	defer w.Stop()

	update := <-w.Updates()
	if update.Refreshing || !update.HasSessions || !update.Done {
		t.Fatalf("expected static watcher to publish completed data, got %#v", update)
	}
	if len(update.Sessions) != 1 || update.Sessions[0] != session {
		t.Fatalf("expected static session update, got %#v", update.Sessions)
	}
}

func TestWatcherPartialRefreshMergesWithExistingSessions(t *testing.T) {
	unchanged := &models.Session{ID: "unchanged", AgentType: models.AgentCopilot, ProjectPath: "/repo/unchanged"}
	oldChanged := &models.Session{ID: "changed", AgentType: models.AgentCopilot, ProjectPath: "/repo/old"}
	changed := &models.Session{ID: "changed", AgentType: models.AgentCopilot, ProjectPath: "/repo/new"}
	added := &models.Session{ID: "added", AgentType: models.AgentCodex, ProjectPath: "/repo/added"}
	w := &Watcher{
		sessions: []*models.Session{unchanged, oldChanged},
		updates:  make(chan UpdateMsg, 1),
	}

	w.publish([]*models.Session{changed, added}, nil, true, false)

	update := <-w.Updates()
	if !update.Refreshing || update.Done {
		t.Fatalf("expected partial refresh update, got %#v", update)
	}
	if len(update.Sessions) != 3 {
		t.Fatalf("expected unchanged, changed, and added sessions, got %#v", update.Sessions)
	}
	if update.Sessions[0] != unchanged {
		t.Fatalf("expected unchanged session to be preserved during partial refresh")
	}
	if update.Sessions[1] != changed {
		t.Fatalf("expected changed session to be replaced during partial refresh")
	}
	if update.Sessions[2] != added {
		t.Fatalf("expected new session to be appended during partial refresh")
	}
	if cached := w.Sessions(); len(cached) != 3 || cached[0] != unchanged || cached[1] != changed || cached[2] != added {
		t.Fatalf("expected watcher cache to hold merged partial refresh, got %#v", cached)
	}
}

func TestWatcherFinalRefreshRemovesDeletedSessions(t *testing.T) {
	deleted := &models.Session{ID: "deleted", AgentType: models.AgentCopilot}
	oldKept := &models.Session{ID: "kept", AgentType: models.AgentCopilot, ProjectPath: "/repo/old"}
	kept := &models.Session{ID: "kept", AgentType: models.AgentCopilot, ProjectPath: "/repo/new"}
	w := &Watcher{
		sessions: []*models.Session{deleted, oldKept},
		updates:  make(chan UpdateMsg, 1),
	}

	w.publish([]*models.Session{kept}, nil, false, true)

	update := <-w.Updates()
	if update.Refreshing || !update.Done {
		t.Fatalf("expected final refresh update, got %#v", update)
	}
	if len(update.Sessions) != 1 || update.Sessions[0] != kept {
		t.Fatalf("expected final refresh to remove deleted sessions and keep updated session, got %#v", update.Sessions)
	}
	if cached := w.Sessions(); len(cached) != 1 || cached[0] != kept {
		t.Fatalf("expected watcher cache to reconcile deletions, got %#v", cached)
	}
}
