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
