package agents

import (
	"bufio"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

type CodexDetector struct{}

func NewCodexDetector() *CodexDetector { return &CodexDetector{} }

func (c *CodexDetector) Name() string { return "Codex CLI" }

type codexEntry struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type codexPayload struct {
	Type               string                 `json:"type"`
	ID                 string                 `json:"id"`
	CWD                string                 `json:"cwd"`
	Model              string                 `json:"model"`
	Effort             string                 `json:"effort"`
	TurnID             string                 `json:"turn_id"`
	TurnIDCamel        string                 `json:"turnId"`
	Role               string                 `json:"role"`
	Phase              string                 `json:"phase"`
	Message            string                 `json:"message"`
	Content            []codexContent         `json:"content"`
	Name               string                 `json:"name"`
	Namespace          string                 `json:"namespace"`
	Tool               string                 `json:"tool"`
	Arguments          json.RawMessage        `json:"arguments"`
	Input              string                 `json:"input"`
	Output             string                 `json:"output"`
	CallID             string                 `json:"call_id"`
	CallIDCamel        string                 `json:"callId"`
	Status             string                 `json:"status"`
	Success            *bool                  `json:"success"`
	ExitCode           *int                   `json:"exit_code"`
	Command            []string               `json:"command"`
	Stdout             string                 `json:"stdout"`
	Stderr             string                 `json:"stderr"`
	FormattedOutput    string                 `json:"formatted_output"`
	AggregatedOutput   string                 `json:"aggregated_output"`
	Changes            map[string]codexChange `json:"changes"`
	StartedAt          float64                `json:"started_at"`
	CompletedAt        float64                `json:"completed_at"`
	DurationMS         int                    `json:"duration_ms"`
	TimeToFirstTokenMS int                    `json:"time_to_first_token_ms"`
	LastAgentMessage   string                 `json:"last_agent_message"`
	Info               *codexTokenInfo        `json:"info"`
	Git                codexGit               `json:"git"`
}

type codexContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type codexChange struct {
	Type string `json:"type"`
}

type codexGit struct {
	Branch        string `json:"branch"`
	CommitHash    string `json:"commit_hash"`
	RepositoryURL string `json:"repository_url"`
}

type codexTokenInfo struct {
	TotalTokenUsage codexTokenUsage `json:"total_token_usage"`
	LastTokenUsage  codexTokenUsage `json:"last_token_usage"`
}

type codexTokenUsage struct {
	InputTokens           int `json:"input_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens"`
	TotalTokens           int `json:"total_tokens"`
}

func (c *CodexDetector) Detect() ([]*models.Session, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	sessionsDir := filepath.Join(home, ".codex", "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return nil, nil
	}

	var sessions []*models.Session
	err = filepath.WalkDir(sessionsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		session, err := c.parseSession(path)
		if err != nil || session == nil {
			return nil
		}
		sessions = append(sessions, session)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastUpdated.After(sessions[j].LastUpdated)
	})
	return sessions, nil
}

func (c *CodexDetector) parseSession(logPath string) (*models.Session, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	sessionID := fmt.Sprintf("%x", md5.Sum([]byte(logPath)))
	session := &models.Session{
		ID:          sessionID,
		AgentType:   models.AgentCodex,
		ProjectPath: strings.TrimSuffix(filepath.Base(logPath), filepath.Ext(logPath)),
		LogPath:     logPath,
		LastUpdated: info.ModTime(),
		IsActive:    time.Since(info.ModTime()) < 5*time.Minute,
	}

	toolNames := make(map[string]string)
	completedTools := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry codexEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		timestamp := parseCodexTime(entry.Timestamp)
		if timestamp.IsZero() {
			timestamp = info.ModTime()
		}
		if session.StartTime.IsZero() || timestamp.Before(session.StartTime) {
			session.StartTime = timestamp
		}
		session.LastUpdated = latestTime(session.LastUpdated, timestamp)

		var payload codexPayload
		if len(entry.Payload) > 0 {
			_ = json.Unmarshal(entry.Payload, &payload)
		}
		c.applyEntry(session, entry.Type, payload, timestamp, toolNames, completedTools)
	}
	if session.StartTime.IsZero() {
		session.StartTime = info.ModTime()
	}
	return session, nil
}

func (c *CodexDetector) applyEntry(session *models.Session, entryType string, payload codexPayload, timestamp time.Time, toolNames map[string]string, completedTools map[string]bool) {
	switch entryType {
	case "session_meta":
		if payload.ID != "" {
			session.ID = payload.ID
		}
		if payload.CWD != "" {
			session.ProjectPath = payload.CWD
		}
	case "turn_context":
		if payload.CWD != "" {
			session.ProjectPath = payload.CWD
		}
	case "event_msg":
		c.applyEventMessage(session, payload, timestamp, completedTools)
	case "response_item":
		c.applyResponseItem(session, payload, timestamp, toolNames, completedTools)
	}
}

func (c *CodexDetector) applyEventMessage(session *models.Session, payload codexPayload, timestamp time.Time, completedTools map[string]bool) {
	switch payload.Type {
	case "user_message":
		if strings.TrimSpace(payload.Message) != "" {
			session.Messages = append(session.Messages, models.Message{
				Role:      "user",
				Content:   payload.Message,
				Timestamp: timestamp,
			})
		}
	case "task_started":
		session.Messages = append(session.Messages, codexSessionActivity(payload, timestamp, models.ActivityLifecycleStarted))
	case "task_complete":
		session.Messages = append(session.Messages, codexSessionActivity(payload, timestamp, models.ActivityLifecycleCompleted))
	case "token_count":
		if payload.Info != nil {
			session.TotalTokens = codexTokenUsageToModel(payload.Info.TotalTokenUsage)
		}
	case "exec_command_end", "patch_apply_end", "mcp_tool_call_end", "web_search_end", "dynamic_tool_call_response":
		if msg, ok := codexToolCompletion(payload, timestamp); ok {
			session.Messages = append(session.Messages, msg)
			completedTools[payload.callID()] = true
		}
	}
}

func (c *CodexDetector) applyResponseItem(session *models.Session, payload codexPayload, timestamp time.Time, toolNames map[string]string, completedTools map[string]bool) {
	switch payload.Type {
	case "message":
		if payload.Role == "assistant" {
			if content := codexContentText(payload.Content); content != "" {
				session.Messages = append(session.Messages, models.Message{
					Role:      "assistant",
					Content:   content,
					Timestamp: timestamp,
				})
			}
		}
	case "function_call", "custom_tool_call", "tool_search_call", "web_search_call":
		if msg, ok := codexToolStart(payload, timestamp); ok {
			session.Messages = append(session.Messages, msg)
			toolNames[payload.callID()] = msg.Meta.Label
		}
	case "function_call_output", "custom_tool_call_output", "tool_search_output":
		callID := payload.callID()
		if completedTools[callID] {
			return
		}
		payload.Name = toolNames[callID]
		if msg, ok := codexToolOutput(payload, timestamp); ok {
			session.Messages = append(session.Messages, msg)
			completedTools[callID] = true
		}
	}
}

func codexSessionActivity(payload codexPayload, timestamp time.Time, lifecycle string) models.Message {
	label := "turn"
	content := "Task " + lifecycle
	if payload.Model != "" {
		content += "\nmodel: " + payload.Model
	}
	if payload.DurationMS > 0 {
		content += fmt.Sprintf("\nduration: %s", codexFormatDuration(time.Duration(payload.DurationMS)*time.Millisecond))
	}
	if payload.TimeToFirstTokenMS > 0 {
		content += fmt.Sprintf("\ntime to first token: %s", codexFormatDuration(time.Duration(payload.TimeToFirstTokenMS)*time.Millisecond))
	}
	if payload.LastAgentMessage != "" {
		content += "\nlast message: " + payload.LastAgentMessage
	}
	return models.Message{
		Role:      "session",
		Content:   content,
		Timestamp: timestamp,
		Meta: models.ActivityMeta{
			Kind:      models.ActivityKindSession,
			Lifecycle: lifecycle,
			ID:        payload.turnID(),
			Label:     label,
		},
	}
}

func codexToolStart(payload codexPayload, timestamp time.Time) (models.Message, bool) {
	callID := payload.callID()
	if callID == "" {
		return models.Message{}, false
	}
	label := codexToolName(payload)
	content := "Started tool: " + label
	if detail := codexToolStartDetail(payload); detail != "" {
		content += "\n" + detail
	}
	return models.Message{
		Role:      "tool",
		Content:   content,
		Timestamp: timestamp,
		Meta: models.ActivityMeta{
			Kind:      models.ActivityKindTool,
			Lifecycle: models.ActivityLifecycleStarted,
			ID:        callID,
			Label:     label,
		},
	}, true
}

func codexToolCompletion(payload codexPayload, timestamp time.Time) (models.Message, bool) {
	callID := payload.callID()
	if callID == "" {
		return models.Message{}, false
	}
	lifecycle := models.ActivityLifecycleCompleted
	if payload.Success != nil && !*payload.Success || payload.Status == "failed" || payload.ExitCode != nil && *payload.ExitCode != 0 {
		lifecycle = models.ActivityLifecycleFailed
	}
	label := codexToolName(payload)
	content := "Tool completed: " + label
	if lifecycle == models.ActivityLifecycleFailed {
		content = "Tool failed: " + label
	}
	if detail := codexToolCompletionDetail(payload); detail != "" {
		content += "\n" + detail
	}
	return models.Message{
		Role:      "tool",
		Content:   content,
		Timestamp: timestamp,
		Meta: models.ActivityMeta{
			Kind:      models.ActivityKindTool,
			Lifecycle: lifecycle,
			ID:        callID,
			Label:     label,
		},
	}, true
}

func codexToolOutput(payload codexPayload, timestamp time.Time) (models.Message, bool) {
	callID := payload.callID()
	if callID == "" {
		return models.Message{}, false
	}
	label := codexToolName(payload)
	content := "Tool completed: " + label
	if payload.Output != "" {
		content += "\noutput: " + payload.Output
	}
	return models.Message{
		Role:      "tool",
		Content:   content,
		Timestamp: timestamp,
		Meta: models.ActivityMeta{
			Kind:      models.ActivityKindTool,
			Lifecycle: models.ActivityLifecycleCompleted,
			ID:        callID,
			Label:     label,
		},
	}, true
}

func codexToolName(payload codexPayload) string {
	switch {
	case payload.Name != "":
		return payload.Name
	case payload.Tool != "" && payload.Namespace != "":
		return payload.Namespace + "." + payload.Tool
	case payload.Tool != "":
		return payload.Tool
	case payload.Type == "exec_command_end":
		return "exec_command"
	case payload.Type == "patch_apply_end":
		return "apply_patch"
	case payload.Type == "mcp_tool_call_end":
		return "mcp_tool"
	case payload.Type == "web_search_end":
		return "web_search"
	case payload.Type == "dynamic_tool_call_response":
		return "dynamic_tool"
	default:
		return "tool"
	}
}

func codexToolStartDetail(payload codexPayload) string {
	switch {
	case len(payload.Arguments) > 0:
		return "arguments: " + codexSummarize(string(payload.Arguments), 240)
	case payload.Input != "":
		return "input: " + codexSummarize(payload.Input, 240)
	default:
		return ""
	}
}

func codexToolCompletionDetail(payload codexPayload) string {
	parts := make([]string, 0, 5)
	if payload.Status != "" {
		parts = append(parts, "status: "+payload.Status)
	}
	if payload.ExitCode != nil {
		parts = append(parts, fmt.Sprintf("exit: %d", *payload.ExitCode))
	}
	if len(payload.Command) > 0 {
		parts = append(parts, "command: "+strings.Join(payload.Command, " "))
	}
	if len(payload.Changes) > 0 {
		parts = append(parts, fmt.Sprintf("changes: %d", len(payload.Changes)))
	}
	output := firstNonEmpty(payload.FormattedOutput, payload.AggregatedOutput, payload.Stdout, payload.Stderr, payload.Output)
	if output != "" {
		parts = append(parts, "output: "+codexSummarize(output, 240))
	}
	return strings.Join(parts, "\n")
}

func codexContentText(content []codexContent) string {
	var b strings.Builder
	for _, item := range content {
		if item.Text == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(item.Text)
	}
	return b.String()
}

func codexTokenUsageToModel(usage codexTokenUsage) models.TokenUsage {
	return models.TokenUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		CacheReads:   usage.CachedInputTokens,
	}
}

func (p codexPayload) callID() string {
	return firstNonEmpty(p.CallID, p.CallIDCamel)
}

func (p codexPayload) turnID() string {
	return firstNonEmpty(p.TurnID, p.TurnIDCamel)
}

func parseCodexTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t
	}
	return time.Time{}
}

func codexSummarize(value string, width int) string {
	value = strings.Join(strings.Fields(value), " ")
	if width <= 0 || len(value) <= width {
		return value
	}
	if width <= 3 {
		return "..."
	}
	return value[:width-3] + "..."
}

func codexFormatDuration(duration time.Duration) string {
	if duration <= 0 {
		return "0s"
	}
	if duration < time.Second {
		return duration.Round(time.Millisecond).String()
	}
	return duration.Round(time.Second).String()
}
