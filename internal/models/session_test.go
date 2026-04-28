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
