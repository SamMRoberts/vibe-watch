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

const (
	ActivityKindTool     = "tool"
	ActivityKindSubagent = "subagent"
	ActivityKindSession  = "session"

	ActivityLifecycleRequested = "requested"
	ActivityLifecycleStarted   = "started"
	ActivityLifecycleCompleted = "completed"
	ActivityLifecycleFailed    = "failed"
)

type ActivityMeta struct {
	Kind          string
	Lifecycle     string
	ID            string
	ParentID      string
	InteractionID string
	Label         string
	EventID       string
	EventParentID string
	RawParentID   string
}

type Message struct {
	Role      string
	Content   string
	Timestamp time.Time
	Tokens    TokenUsage
	Meta      ActivityMeta
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
}

func (s *Session) TotalInputTokens() int {
	return s.TotalTokens.InputTokens
}

func (s *Session) TotalOutputTokens() int {
	return s.TotalTokens.OutputTokens
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

func SameSession(a, b *Session) bool {
	if a == nil || b == nil {
		return false
	}
	if a.ID != "" && b.ID == a.ID {
		return a.AgentType == "" || b.AgentType == a.AgentType
	}
	return a.LogPath != "" && b.LogPath == a.LogPath
}

func MergeSessionUpdates(current, incoming []*Session, removeMissing bool) []*Session {
	if len(current) == 0 {
		return append([]*Session(nil), incoming...)
	}

	out := append([]*Session(nil), current...)
	matched := make([]bool, len(out))
	for _, session := range incoming {
		if session == nil {
			continue
		}
		if idx := MatchingSessionIndex(out, session); idx >= 0 {
			out[idx] = session
			matched[idx] = true
			continue
		}
		out = append(out, session)
		matched = append(matched, true)
	}

	if !removeMissing {
		return out
	}

	reconciled := out[:0]
	for i, session := range out {
		if matched[i] || MatchingSessionIndex(incoming, session) >= 0 {
			reconciled = append(reconciled, session)
		}
	}
	return append([]*Session(nil), reconciled...)
}

func MatchingSessionIndex(sessions []*Session, target *Session) int {
	for i, session := range sessions {
		if SameSession(session, target) {
			return i
		}
	}
	return -1
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
