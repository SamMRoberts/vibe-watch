package agents

import (
	"testing"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

func TestParseCopilotChatSessions(t *testing.T) {
	index := []byte(`{
		"version": 1,
		"entries": {
			"session-a": {
				"sessionId": "session-a",
				"title": "Fallback title",
				"lastMessageDate": 1753283604903,
				"initialLocation": "panel",
				"isEmpty": false,
				"timing": {
					"created": 1753283603903,
					"lastRequestEnded": 1753283604903
				}
			},
			"empty-session": {
				"sessionId": "empty-session",
				"title": "Ignored",
				"isEmpty": true
			}
		}
	}`)
	metadata := map[string]copilotChatMetadata{
		"session-a": {FirstUserMessage: "first prompt"},
	}

	sessions, err := parseCopilotChatSessions(index, metadata, "/repo/vibe-watch", "/global/state.vscdb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	session := sessions[0]
	if session.ID != "vscode-copilot-chat-session-a" {
		t.Errorf("expected prefixed session ID, got %q", session.ID)
	}
	if session.AgentType != models.AgentCopilotChat {
		t.Errorf("expected AgentCopilotChat, got %q", session.AgentType)
	}
	if session.ProjectPath != "/repo/vibe-watch" {
		t.Errorf("expected project path, got %q", session.ProjectPath)
	}
	if session.LogPath != "/global/state.vscdb" {
		t.Errorf("expected log path, got %q", session.LogPath)
	}
	if len(session.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(session.Messages))
	}
	if session.Messages[0].Content != "first prompt" {
		t.Errorf("expected first prompt from metadata, got %q", session.Messages[0].Content)
	}
	if session.StartTime.IsZero() || session.LastUpdated.IsZero() {
		t.Fatalf("expected parsed timestamps, got start=%v updated=%v", session.StartTime, session.LastUpdated)
	}
	if !session.LastUpdated.After(session.StartTime) {
		t.Errorf("expected last updated after start, got start=%v updated=%v", session.StartTime, session.LastUpdated)
	}
}

func TestParseCopilotChatSessionsFallsBackToTitle(t *testing.T) {
	index := []byte(`{
		"entries": {
			"session-a": {
				"sessionId": "session-a",
				"title": "Title prompt",
				"lastMessageDate": 1753283603903,
				"timing": {"created": 1753283603903}
			}
		}
	}`)

	sessions, err := parseCopilotChatSessions(index, nil, "/repo/vibe-watch", "/global/state.vscdb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if len(sessions[0].Messages) != 1 || sessions[0].Messages[0].Content != "Title prompt" {
		t.Fatalf("expected title fallback message, got %#v", sessions[0].Messages)
	}
}

func TestTimeFromMillis(t *testing.T) {
	seconds := timeFromMillis(1753283603)
	millis := timeFromMillis(1753283603903)
	if !seconds.Equal(time.Unix(1753283603, 0)) {
		t.Errorf("expected seconds timestamp, got %v", seconds)
	}
	if !millis.Equal(time.UnixMilli(1753283603903)) {
		t.Errorf("expected millis timestamp, got %v", millis)
	}
}
