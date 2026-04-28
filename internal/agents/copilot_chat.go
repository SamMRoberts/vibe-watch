package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

const copilotChatStateKey = "chat.ChatSessionStore.index"

type CopilotChatDetector struct{}

func NewCopilotChatDetector() *CopilotChatDetector { return &CopilotChatDetector{} }

func (c *CopilotChatDetector) Name() string { return string(models.AgentCopilotChat) }

type copilotChatStore struct {
	Entries map[string]copilotChatEntry `json:"entries"`
}

type copilotChatEntry struct {
	SessionID         string `json:"sessionId"`
	Title             string `json:"title"`
	LastMessageDate   int64  `json:"lastMessageDate"`
	InitialLocation   string `json:"initialLocation"`
	IsEmpty           bool   `json:"isEmpty"`
	LastResponseState int    `json:"lastResponseState"`
	Timing            struct {
		Created          int64 `json:"created"`
		LastRequestEnded int64 `json:"lastRequestEnded"`
	} `json:"timing"`
}

type copilotChatMetadata struct {
	FirstUserMessage string `json:"firstUserMessage"`
	WrittenToDisc    bool   `json:"writtenToDisc"`
}

func (c *CopilotChatDetector) Detect() ([]*models.Session, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	globalStorageDir := filepath.Join(home, "Library", "Application Support", "Code", "User", "globalStorage")
	stateDBPath := filepath.Join(globalStorageDir, "state.vscdb")
	if _, err := os.Stat(stateDBPath); os.IsNotExist(err) {
		return nil, nil
	}

	indexData, err := readVSCodeStateValue(stateDBPath, copilotChatStateKey)
	if err != nil || len(strings.TrimSpace(string(indexData))) == 0 {
		return nil, err
	}

	metadataPath := filepath.Join(globalStorageDir, "github.copilot-chat", "copilotCli", "copilotcli.session.metadata.json")
	metadata := readCopilotChatMetadata(metadataPath)

	projectPath, err := os.Getwd()
	if err != nil {
		projectPath = "VS Code globalStorage"
	}

	return parseCopilotChatSessions(indexData, metadata, projectPath, stateDBPath)
}

func readVSCodeStateValue(dbPath, key string) ([]byte, error) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		return nil, nil
	}

	query := fmt.Sprintf("SELECT value FROM ItemTable WHERE key = '%s';", strings.ReplaceAll(key, "'", "''"))
	out, err := exec.Command("sqlite3", dbPath, query).Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func readCopilotChatMetadata(path string) map[string]copilotChatMetadata {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var metadata map[string]copilotChatMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil
	}
	return metadata
}

func parseCopilotChatSessions(indexData []byte, metadata map[string]copilotChatMetadata, projectPath, logPath string) ([]*models.Session, error) {
	var store copilotChatStore
	if err := json.Unmarshal(indexData, &store); err != nil {
		return nil, err
	}

	sessions := make([]*models.Session, 0, len(store.Entries))
	for key, entry := range store.Entries {
		if entry.IsEmpty {
			continue
		}

		sessionID := entry.SessionID
		if sessionID == "" {
			sessionID = key
		}

		createdAt := timeFromMillis(entry.Timing.Created)
		lastUpdated := latestTime(timeFromMillis(entry.LastMessageDate), timeFromMillis(entry.Timing.LastRequestEnded))
		if lastUpdated.IsZero() {
			lastUpdated = createdAt
		}

		session := &models.Session{
			ID:          "vscode-copilot-chat-" + sessionID,
			AgentType:   models.AgentCopilotChat,
			ProjectPath: projectPath,
			LogPath:     logPath,
			StartTime:   createdAt,
			LastUpdated: lastUpdated,
			IsActive:    !lastUpdated.IsZero() && time.Since(lastUpdated) < 5*time.Minute,
		}

		if message := copilotChatMessage(entry, metadata[sessionID]); message != "" {
			session.Messages = append(session.Messages, models.Message{
				Role:      "user",
				Content:   message,
				Timestamp: createdAt,
			})
		}

		sessions = append(sessions, session)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastUpdated.After(sessions[j].LastUpdated)
	})
	return sessions, nil
}

func copilotChatMessage(entry copilotChatEntry, metadata copilotChatMetadata) string {
	if metadata.FirstUserMessage != "" {
		return metadata.FirstUserMessage
	}
	return entry.Title
}

func timeFromMillis(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	if value < 10_000_000_000 {
		return time.Unix(value, 0)
	}
	return time.UnixMilli(value)
}
