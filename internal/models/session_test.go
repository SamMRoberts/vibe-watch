package models_test

import (
	"testing"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

func TestSessionDuration(t *testing.T) {
	now := time.Now()
	s := &models.Session{
		StartTime:   now.Add(-10 * time.Minute),
		LastUpdated: now,
	}
	d := s.Duration()
	if d < 9*time.Minute || d > 11*time.Minute {
		t.Errorf("expected ~10m duration, got %v", d)
	}
}

func TestSessionDurationZeroStart(t *testing.T) {
	s := &models.Session{}
	if s.Duration() != 0 {
		t.Errorf("expected 0 duration for zero start time, got %v", s.Duration())
	}
}

func TestSessionMessageCount(t *testing.T) {
	s := &models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "world"},
		},
	}
	if s.MessageCount() != 2 {
		t.Errorf("expected 2 messages, got %d", s.MessageCount())
	}
}

func TestSessionTotalTokens(t *testing.T) {
	s := &models.Session{
		TotalTokens: models.TokenUsage{
			InputTokens:  100,
			OutputTokens: 200,
		},
	}
	if s.TotalInputTokens() != 100 {
		t.Errorf("expected 100 input tokens, got %d", s.TotalInputTokens())
	}
	if s.TotalOutputTokens() != 200 {
		t.Errorf("expected 200 output tokens, got %d", s.TotalOutputTokens())
	}
}

func TestMergeSessionUpdatesPreservesUnseenSessionsDuringPartialRefresh(t *testing.T) {
	unchanged := &models.Session{ID: "unchanged", AgentType: models.AgentCopilot, ProjectPath: "/repo/unchanged"}
	oldChanged := &models.Session{ID: "changed", AgentType: models.AgentCopilot, ProjectPath: "/repo/old"}
	changed := &models.Session{ID: "changed", AgentType: models.AgentCopilot, ProjectPath: "/repo/new"}
	added := &models.Session{ID: "added", AgentType: models.AgentCodex, ProjectPath: "/repo/added"}

	merged := models.MergeSessionUpdates(
		[]*models.Session{unchanged, oldChanged},
		[]*models.Session{changed, added},
		false,
	)

	if len(merged) != 3 {
		t.Fatalf("expected unchanged, changed, and added sessions, got %#v", merged)
	}
	if merged[0] != unchanged || merged[1] != changed || merged[2] != added {
		t.Fatalf("expected partial merge to update in place and preserve old sessions, got %#v", merged)
	}
}

func TestMergeSessionUpdatesRemovesMissingSessionsOnFinalRefresh(t *testing.T) {
	deleted := &models.Session{ID: "deleted", AgentType: models.AgentCopilot}
	oldKept := &models.Session{ID: "kept", AgentType: models.AgentCopilot, ProjectPath: "/repo/old"}
	kept := &models.Session{ID: "kept", AgentType: models.AgentCopilot, ProjectPath: "/repo/new"}

	merged := models.MergeSessionUpdates(
		[]*models.Session{deleted, oldKept},
		[]*models.Session{kept},
		true,
	)

	if len(merged) != 1 || merged[0] != kept {
		t.Fatalf("expected final merge to remove deleted sessions, got %#v", merged)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m30s"},
		{90 * time.Minute, "1h30m"},
	}
	for _, tc := range tests {
		got := models.FormatDuration(tc.d)
		if got != tc.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}
