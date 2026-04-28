package agents

import (
	"os"
	"path/filepath"
	"strings"
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

func TestParseCopilotChatWorkspaceMergesTranscriptAndChatSessionMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "GitHub.copilot-chat", "transcripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "chatSessions"), 0o755); err != nil {
		t.Fatal(err)
	}

	index := []byte(`{
		"entries": {
			"session-a": {
				"sessionId": "session-a",
				"title": "VS Code session",
				"lastMessageDate": 1777401600000,
				"isEmpty": false,
				"timing": {
					"created": 1777401500000,
					"lastRequestStarted": 1777401510000,
					"lastRequestEnded": 1777401600000
				}
			}
		}
	}`)

	transcript := strings.Join([]string{
		`{"type":"session.start","id":"s","parentId":"","timestamp":"2026-04-28T20:00:00.000Z","data":{"sessionId":"session-a","producer":"copilot-agent","startTime":"2026-04-28T20:00:00.000Z"}}`,
		`{"type":"user.message","id":"u","parentId":"s","timestamp":"2026-04-28T20:00:01.000Z","data":{"content":"hello vscode","attachments":[]}}`,
		`{"type":"assistant.message","id":"a","parentId":"u","timestamp":"2026-04-28T20:00:02.000Z","data":{"messageId":"m1","content":"I will inspect files","toolRequests":[{"toolCallId":"tool-1","name":"read_file","type":"function"}]}}`,
		`{"type":"tool.execution_start","id":"t1","parentId":"a","timestamp":"2026-04-28T20:00:03.000Z","data":{"toolCallId":"tool-1","toolName":"read_file","arguments":{"filePath":"internal/tui/detail.go"}}}`,
		`{"type":"tool.execution_complete","id":"t2","parentId":"t1","timestamp":"2026-04-28T20:00:04.000Z","data":{"toolCallId":"tool-1","success":true}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(root, "GitHub.copilot-chat", "transcripts", "session-a.jsonl"), []byte(transcript), 0o644); err != nil {
		t.Fatal(err)
	}

	chatSession := strings.Join([]string{
		`{"kind":0,"v":{"version":3,"creationDate":1777401500000,"sessionId":"session-a","requests":[],"pendingRequests":[],"inputState":{"selectedModel":{"identifier":"copilot/gpt-5.5"}}}}`,
		`{"kind":2,"k":["requests"],"v":[{"requestId":"req-1","responseId":"resp-1","timestamp":1777401510000,"message":{"text":"hello vscode"},"modelId":"copilot/gpt-5.5","response":[]}]} `,
		`{"kind":1,"k":["requests",0,"completionTokens"],"v":1234}`,
		`{"kind":1,"k":["requests",0,"elapsedMs"],"v":5678}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(root, "chatSessions", "session-a.jsonl"), []byte(chatSession), 0o644); err != nil {
		t.Fatal(err)
	}

	sessions, err := parseCopilotChatWorkspace(index, nil, "/repo/vibe-watch", filepath.Join(root, "state.vscdb"), root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	session := sessions[0]
	if session.LogPath != filepath.Join(root, "GitHub.copilot-chat", "transcripts", "session-a.jsonl") {
		t.Fatalf("expected transcript log path, got %q", session.LogPath)
	}
	if session.TotalTokens.OutputTokens != 1234 {
		t.Fatalf("expected completion tokens from chatSessions, got %d", session.TotalTokens.OutputTokens)
	}
	if len(session.Messages) != 5 {
		t.Fatalf("expected transcript messages plus summary, got %d: %#v", len(session.Messages), session.Messages)
	}
	if session.Messages[0].Role != "user" || session.Messages[0].Content != "hello vscode" {
		t.Fatalf("unexpected user message: %#v", session.Messages[0])
	}
	start := session.Messages[2]
	if start.Meta.Kind != models.ActivityKindTool ||
		start.Meta.Lifecycle != models.ActivityLifecycleStarted ||
		start.Meta.ID != "tool-1" ||
		start.Meta.Label != "read_file" {
		t.Fatalf("unexpected start metadata: %#v", start.Meta)
	}
	complete := session.Messages[3]
	if complete.Meta.Kind != models.ActivityKindTool ||
		complete.Meta.Lifecycle != models.ActivityLifecycleCompleted ||
		complete.Meta.ID != "tool-1" ||
		complete.Meta.Label != "read_file" {
		t.Fatalf("unexpected completion metadata: %#v", complete.Meta)
	}
	summary := session.Messages[len(session.Messages)-1]
	if summary.Role != "session" ||
		!strings.Contains(summary.Content, "model: copilot/gpt-5.5") ||
		!strings.Contains(summary.Content, "completion tokens: 1234") ||
		!strings.Contains(summary.Content, "elapsed: 5s") {
		t.Fatalf("expected useful summary, got %#v", summary)
	}
}

func TestReadVSCodeWorkspacePathDecodesFileURI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.json")
	if err := os.WriteFile(path, []byte(`{"folder":"file:///Users/samroberts/Repo/SamMRoberts/vibe-watch"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readVSCodeWorkspacePath(path); got != "/Users/samroberts/Repo/SamMRoberts/vibe-watch" {
		t.Fatalf("expected decoded file URI path, got %q", got)
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
