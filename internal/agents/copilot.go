package agents

import (
	"bufio"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

type CopilotDetector struct{}

func NewCopilotDetector() *CopilotDetector { return &CopilotDetector{} }

func (c *CopilotDetector) Name() string { return "Copilot CLI" }

type copilotWorkspace struct {
	ID        string
	CWD       string
	GitRoot   string
	Summary   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type copilotEvent struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

type copilotMessageData struct {
	Content            string                 `json:"content"`
	TransformedContent string                 `json:"transformedContent"`
	OutputTokens       int                    `json:"outputTokens"`
	ToolRequests       []copilotToolRequest   `json:"toolRequests"`
	ModelMetrics       map[string]modelMetric `json:"modelMetrics"`
}

type copilotToolRequest struct {
	Name             string `json:"name"`
	IntentionSummary string `json:"intentionSummary"`
}

type modelMetric struct {
	Usage struct {
		InputTokens      int `json:"inputTokens"`
		OutputTokens     int `json:"outputTokens"`
		CacheReadTokens  int `json:"cacheReadTokens"`
		CacheWriteTokens int `json:"cacheWriteTokens"`
	} `json:"usage"`
}

func (c *CopilotDetector) Detect() ([]*models.Session, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	sessions, err := c.detectSessionState(filepath.Join(home, ".copilot", "session-state"))
	if err != nil {
		return nil, err
	}
	if len(sessions) > 0 {
		return sessions, nil
	}

	return c.detectLogFiles(filepath.Join(home, ".copilot", "logs"))
}

func (c *CopilotDetector) detectSessionState(sessionsDir string) ([]*models.Session, error) {
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, err
	}

	var sessions []*models.Session
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionDir := filepath.Join(sessionsDir, entry.Name())
		eventsPath := filepath.Join(sessionDir, "events.jsonl")
		if _, err := os.Stat(eventsPath); os.IsNotExist(err) {
			continue
		}

		workspace := parseCopilotWorkspace(filepath.Join(sessionDir, "workspace.yaml"))
		session, err := c.parseEventsSession(eventsPath, entry.Name(), workspace)
		if err != nil || session == nil {
			continue
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (c *CopilotDetector) parseEventsSession(eventsPath, sessionID string, workspace copilotWorkspace) (*models.Session, error) {
	file, err := os.Open(eventsPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if workspace.ID != "" {
		sessionID = workspace.ID
	}

	session := &models.Session{
		ID:          sessionID,
		AgentType:   models.AgentCopilot,
		ProjectPath: copilotProjectPath(workspace, sessionID),
		LogPath:     eventsPath,
		StartTime:   workspace.CreatedAt,
		LastUpdated: latestTime(workspace.UpdatedAt, info.ModTime()),
		IsActive:    time.Since(info.ModTime()) < 5*time.Minute,
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event copilotEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		timestamp := parseCopilotTime(event.Timestamp)
		if timestamp.IsZero() {
			timestamp = info.ModTime()
		}
		if session.StartTime.IsZero() || timestamp.Before(session.StartTime) {
			session.StartTime = timestamp
		}
		if timestamp.After(session.LastUpdated) {
			session.LastUpdated = timestamp
		}

		var data copilotMessageData
		if len(event.Data) > 0 {
			_ = json.Unmarshal(event.Data, &data)
		}

		switch event.Type {
		case "user.message":
			content := data.Content
			if content == "" {
				content = data.TransformedContent
			}
			session.Messages = append(session.Messages, models.Message{
				Role:      "user",
				Content:   content,
				Timestamp: timestamp,
			})
		case "assistant.message":
			content := data.Content
			if content == "" && len(data.ToolRequests) > 0 {
				content = summarizeToolRequests(data.ToolRequests)
			}
			session.Messages = append(session.Messages, models.Message{
				Role:      "assistant",
				Content:   content,
				Timestamp: timestamp,
				Tokens: models.TokenUsage{
					OutputTokens: data.OutputTokens,
				},
			})
			session.TotalTokens.OutputTokens += data.OutputTokens
		case "session.shutdown":
			applyCopilotMetrics(session, data.ModelMetrics)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if session.StartTime.IsZero() {
		session.StartTime = info.ModTime()
	}
	if session.LastUpdated.IsZero() {
		session.LastUpdated = info.ModTime()
	}

	return session, nil
}

func (c *CopilotDetector) detectLogFiles(logsDir string) ([]*models.Session, error) {
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		return nil, nil
	}

	files, err := os.ReadDir(logsDir)
	if err != nil {
		return nil, err
	}

	var sessions []*models.Session
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		logPath := filepath.Join(logsDir, f.Name())
		info, err := f.Info()
		if err != nil {
			continue
		}

		id := fmt.Sprintf("%x", md5.Sum([]byte(logPath)))
		session := &models.Session{
			ID:          id,
			AgentType:   models.AgentCopilot,
			ProjectPath: strings.TrimSuffix(f.Name(), filepath.Ext(f.Name())),
			LogPath:     logPath,
			StartTime:   info.ModTime(),
			LastUpdated: info.ModTime(),
			IsActive:    time.Since(info.ModTime()) < 5*time.Minute,
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func parseCopilotWorkspace(path string) copilotWorkspace {
	file, err := os.Open(path)
	if err != nil {
		return copilotWorkspace{}
	}
	defer file.Close()

	var workspace copilotWorkspace
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), ":")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), "'")
		value = strings.Trim(value, "\"")
		switch strings.TrimSpace(key) {
		case "id":
			workspace.ID = value
		case "cwd":
			workspace.CWD = value
		case "git_root":
			workspace.GitRoot = value
		case "summary":
			workspace.Summary = value
		case "created_at":
			workspace.CreatedAt = parseCopilotTime(value)
		case "updated_at":
			workspace.UpdatedAt = parseCopilotTime(value)
		}
	}
	return workspace
}

func parseCopilotTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func copilotProjectPath(workspace copilotWorkspace, fallback string) string {
	if workspace.GitRoot != "" {
		return workspace.GitRoot
	}
	if workspace.CWD != "" {
		return workspace.CWD
	}
	if workspace.Summary != "" {
		return workspace.Summary
	}
	return fallback
}

func summarizeToolRequests(requests []copilotToolRequest) string {
	parts := make([]string, 0, len(requests))
	for _, request := range requests {
		if request.IntentionSummary != "" {
			parts = append(parts, request.IntentionSummary)
			continue
		}
		if request.Name != "" {
			parts = append(parts, "Used tool: "+request.Name)
		}
	}
	return strings.Join(parts, "\n")
}

func applyCopilotMetrics(session *models.Session, metrics map[string]modelMetric) {
	if len(metrics) == 0 {
		return
	}

	session.TotalTokens = models.TokenUsage{}
	for _, metric := range metrics {
		session.TotalTokens.InputTokens += metric.Usage.InputTokens
		session.TotalTokens.OutputTokens += metric.Usage.OutputTokens
		session.TotalTokens.CacheReads += metric.Usage.CacheReadTokens
		session.TotalTokens.CacheWrites += metric.Usage.CacheWriteTokens
	}
}

func latestTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
