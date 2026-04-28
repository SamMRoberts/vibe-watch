package models

import (
	"fmt"
	"time"
)

type AgentType string

const (
	AgentClaude      AgentType = "Claude Code"
	AgentCodex       AgentType = "Codex CLI"
	AgentCopilot     AgentType = "Copilot CLI"
	AgentCopilotChat AgentType = "Copilot Chat"
	AgentAmazonQ     AgentType = "Amazon Q"
)

type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	CacheReads   int
	CacheWrites  int
}

type Message struct {
	Role      string
	Content   string
	Timestamp time.Time
	Tokens    TokenUsage
}

type Session struct {
	ID          string
	AgentType   AgentType
	ProjectPath string
	StartTime   time.Time
	LastUpdated time.Time
	Messages    []Message
	TotalTokens TokenUsage
	IsActive    bool
	LogPath     string
	CostUSD     float64
}

func (s *Session) TotalInputTokens() int {
	return s.TotalTokens.InputTokens
}

func (s *Session) TotalOutputTokens() int {
	return s.TotalTokens.OutputTokens
}

func (s *Session) EstimatedCost() float64 {
	if s.CostUSD > 0 {
		return s.CostUSD
	}
	inputCost := float64(s.TotalTokens.InputTokens) * 3.0 / 1_000_000
	outputCost := float64(s.TotalTokens.OutputTokens) * 15.0 / 1_000_000
	return inputCost + outputCost
}

func (s *Session) Duration() time.Duration {
	if s.StartTime.IsZero() {
		return 0
	}
	end := s.LastUpdated
	if end.IsZero() {
		end = time.Now()
	}
	return end.Sub(s.StartTime)
}

func (s *Session) MessageCount() int {
	return len(s.Messages)
}

func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
