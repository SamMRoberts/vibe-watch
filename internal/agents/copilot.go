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
	Role                  string                  `json:"role"`
	Content               string                  `json:"content"`
	TransformedContent    string                  `json:"transformedContent"`
	OutputTokens          int                     `json:"outputTokens"`
	ToolRequests          []copilotToolRequest    `json:"toolRequests"`
	ModelMetrics          map[string]modelMetric  `json:"modelMetrics"`
	CurrentModel          string                  `json:"currentModel"`
	CurrentTokens         int                     `json:"currentTokens"`
	SystemTokens          int                     `json:"systemTokens"`
	ConversationTokens    int                     `json:"conversationTokens"`
	ToolDefinitionsTokens int                     `json:"toolDefinitionsTokens"`
	TotalApiDurationMs    int                     `json:"totalApiDurationMs"`
	TotalPremiumRequests  int                     `json:"totalPremiumRequests"`
	ShutdownType          string                  `json:"shutdownType"`
	ToolCallID            string                  `json:"toolCallId"`
	ParentToolCallID      string                  `json:"parentToolCallId"`
	InteractionID         string                  `json:"interactionId"`
	ToolName              string                  `json:"toolName"`
	Arguments             json.RawMessage         `json:"arguments"`
	MCPServerName         string                  `json:"mcpServerName"`
	MCPToolName           string                  `json:"mcpToolName"`
	Success               *bool                   `json:"success"`
	Result                json.RawMessage         `json:"result"`
	Error                 json.RawMessage         `json:"error"`
	Model                 string                  `json:"model"`
	ToolTelemetry         map[string]any          `json:"toolTelemetry"`
	AgentID               string                  `json:"agentId"`
	SubagentID            string                  `json:"subagentId"`
	AgentName             string                  `json:"agentName"`
	AgentDisplayName      string                  `json:"agentDisplayName"`
	DurationMs            int                     `json:"durationMs"`
	TotalTokens           int                     `json:"totalTokens"`
	TotalToolCalls        int                     `json:"totalToolCalls"`
	NewModel              string                  `json:"newModel"`
	PreviousModel         string                  `json:"previousModel"`
	NewMode               string                  `json:"newMode"`
	PreviousMode          string                  `json:"previousMode"`
	Phase                 string                  `json:"phase"`
	ReasoningEffort       string                  `json:"reasoningEffort"`
	Operation             string                  `json:"operation"`
	InfoType              string                  `json:"infoType"`
	Message               string                  `json:"message"`
	Summary               string                  `json:"summary"`
	ErrorType             string                  `json:"errorType"`
	Reason                string                  `json:"reason"`
	Name                  string                  `json:"name"`
	CodeChanges           copilotCodeChanges      `json:"codeChanges"`
	CheckpointNumber      int                     `json:"checkpointNumber"`
	PreCompactionTokens   int                     `json:"preCompactionTokens"`
	CompactionTokensUsed  copilotCompactionTokens `json:"compactionTokensUsed"`
	Context               struct {
		CWD        string `json:"cwd"`
		GitRoot    string `json:"gitRoot"`
		Repository string `json:"repository"`
		Branch     string `json:"branch"`
	} `json:"context"`
	CWD        string `json:"cwd"`
	GitRoot    string `json:"gitRoot"`
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
}

type copilotToolRequest struct {
	ToolCallID       string `json:"toolCallId"`
	Name             string `json:"name"`
	IntentionSummary string `json:"intentionSummary"`
}

type copilotCodeChanges struct {
	LinesAdded    int      `json:"linesAdded"`
	LinesRemoved  int      `json:"linesRemoved"`
	FilesModified []string `json:"filesModified"`
}

type copilotCompactionTokens struct {
	InputTokens      int    `json:"inputTokens"`
	OutputTokens     int    `json:"outputTokens"`
	CacheReadTokens  int    `json:"cacheReadTokens"`
	CacheWriteTokens int    `json:"cacheWriteTokens"`
	Duration         int    `json:"duration"`
	Model            string `json:"model"`

	// Older inspected logs used shorter names; keep these as a fallback.
	Input       int `json:"input"`
	Output      int `json:"output"`
	CachedInput int `json:"cachedInput"`
}

type modelMetric struct {
	Usage struct {
		InputTokens      int `json:"inputTokens"`
		OutputTokens     int `json:"outputTokens"`
		CacheReadTokens  int `json:"cacheReadTokens"`
		CacheWriteTokens int `json:"cacheWriteTokens"`
		ReasoningTokens  int `json:"reasoningTokens"`
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
	toolNames := make(map[string]string)

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
			content := copilotUserContent(data)
			session.Messages = append(session.Messages, models.Message{
				Role:      "user",
				Content:   content,
				Timestamp: timestamp,
			})
		case "assistant.message":
			content := copilotAssistantContent(data)
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
			applyCopilotMetrics(session, data)
			if activity, ok := copilotActivityMessage(event.Type, data, toolNames); ok {
				activity.Timestamp = timestamp
				session.Messages = append(session.Messages, activity)
			}
		case "session.compaction_complete":
			usage := data.CompactionTokensUsed.usage()
			addTokenUsage(&session.TotalTokens, usage)
			if activity, ok := copilotActivityMessage(event.Type, data, toolNames); ok {
				activity.Timestamp = timestamp
				activity.Tokens = usage
				session.Messages = append(session.Messages, activity)
			}
		default:
			if activity, ok := copilotActivityMessage(event.Type, data, toolNames); ok {
				activity.Timestamp = timestamp
				session.Messages = append(session.Messages, activity)
			}
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

func copilotUserContent(data copilotMessageData) string {
	return firstNonEmpty(data.Content, data.TransformedContent, data.Message, data.Summary)
}

func copilotAssistantContent(data copilotMessageData) string {
	content := firstNonEmpty(data.Content, data.Message, data.Summary)
	if content == "" && len(data.ToolRequests) > 0 {
		content = summarizeToolRequests(data.ToolRequests)
	}
	if content == "" && data.Phase != "" {
		content = "phase: " + data.Phase
	}
	return content
}

func copilotActivityMessage(eventType string, data copilotMessageData, toolNames map[string]string) (models.Message, bool) {
	role, content := copilotActivity(eventType, data, toolNames)
	if content == "" {
		return models.Message{}, false
	}
	return models.Message{
		Role:    role,
		Content: content,
		Meta:    copilotActivityMeta(eventType, data, toolNames),
	}, true
}

func copilotActivityMeta(eventType string, data copilotMessageData, toolNames map[string]string) models.ActivityMeta {
	switch eventType {
	case "tool.execution_start":
		return models.ActivityMeta{
			Kind:          models.ActivityKindTool,
			Lifecycle:     models.ActivityLifecycleStarted,
			ID:            data.ToolCallID,
			ParentID:      data.ParentToolCallID,
			InteractionID: data.InteractionID,
			Label:         copilotToolLabel(data),
		}
	case "tool.user_requested":
		return models.ActivityMeta{
			Kind:          models.ActivityKindTool,
			Lifecycle:     models.ActivityLifecycleRequested,
			ID:            data.ToolCallID,
			ParentID:      data.ParentToolCallID,
			InteractionID: data.InteractionID,
			Label:         copilotToolLabel(data),
		}
	case "tool.execution_complete":
		lifecycle := models.ActivityLifecycleCompleted
		if data.Success != nil && !*data.Success {
			lifecycle = models.ActivityLifecycleFailed
		}
		return models.ActivityMeta{
			Kind:          models.ActivityKindTool,
			Lifecycle:     lifecycle,
			ID:            data.ToolCallID,
			ParentID:      data.ParentToolCallID,
			InteractionID: data.InteractionID,
			Label:         copilotToolLabelWithCache(data, toolNames),
		}
	case "subagent.started":
		return copilotSubagentMeta(data, models.ActivityLifecycleStarted)
	case "subagent.completed":
		return copilotSubagentMeta(data, models.ActivityLifecycleCompleted)
	case "subagent.failed":
		return copilotSubagentMeta(data, models.ActivityLifecycleFailed)
	case "session.task_complete":
		lifecycle := models.ActivityLifecycleCompleted
		if data.Success != nil && !*data.Success {
			lifecycle = models.ActivityLifecycleFailed
		}
		return models.ActivityMeta{
			Kind:      models.ActivityKindSession,
			Lifecycle: lifecycle,
			Label:     "task",
		}
	case "abort":
		return models.ActivityMeta{
			Kind:      models.ActivityKindSession,
			Lifecycle: models.ActivityLifecycleFailed,
			Label:     "abort",
		}
	}
	return models.ActivityMeta{}
}

func copilotActivity(eventType string, data copilotMessageData, toolNames map[string]string) (string, string) {
	switch eventType {
	case "tool.execution_start":
		label := copilotToolLabel(data)
		if data.ToolCallID != "" {
			toolNames[data.ToolCallID] = label
		}
		content := "Started tool: " + label
		if data.ParentToolCallID != "" {
			content += "\nparent: " + data.ParentToolCallID
		}
		if detail := semanticJSONSummary(data.Arguments, "input"); detail != "" {
			content += "\n" + detail
		}
		return "tool", content
	case "tool.user_requested":
		label := copilotToolLabel(data)
		if data.ToolCallID != "" {
			toolNames[data.ToolCallID] = label
		}
		return "tool", "User requested tool: " + label
	case "tool.execution_complete":
		return "tool", copilotToolCompletion(data, toolNames)
	case "subagent.started":
		return "subagent", fmt.Sprintf("Started subagent: %s", copilotAgentLabel(data))
	case "subagent.completed":
		return "subagent", fmt.Sprintf(
			"Completed subagent: %s\nmodel: %s\nduration: %s\ntokens: %d\ntool calls: %d",
			copilotAgentLabel(data),
			emptyDash(data.Model),
			models.FormatDuration(time.Duration(data.DurationMs)*time.Millisecond),
			data.TotalTokens,
			data.TotalToolCalls,
		)
	case "subagent.failed":
		content := fmt.Sprintf(
			"Failed subagent: %s\nmodel: %s\nduration: %s\ntokens: %d\ntool calls: %d",
			copilotAgentLabel(data),
			emptyDash(data.Model),
			models.FormatDuration(time.Duration(data.DurationMs)*time.Millisecond),
			data.TotalTokens,
			data.TotalToolCalls,
		)
		if errText := copilotErrorText(data.Error); errText != "" {
			content += "\nerror: " + errText
		}
		return "subagent", content
	case "subagent.selected":
		return "subagent", fmt.Sprintf("Selected subagent: %s", copilotAgentLabel(data))
	case "subagent.deselected":
		return "subagent", "Deselected subagent"
	case "session.model_change":
		return "session", fmt.Sprintf("Model changed: %s -> %s", emptyDash(data.PreviousModel), emptyDash(data.NewModel))
	case "session.mode_changed":
		return "session", fmt.Sprintf("Mode changed: %s -> %s", emptyDash(data.PreviousMode), emptyDash(data.NewMode))
	case "session.plan_changed":
		return "session", "Plan changed: " + emptyDash(data.Operation)
	case "session.task_complete":
		return "session", copilotTaskComplete(data)
	case "session.info":
		return "session", copilotInfo(data)
	case "session.error":
		content := "Session error: " + emptyDash(data.ErrorType)
		if data.Message != "" {
			content += "\n" + data.Message
		}
		return "error", content
	case "system.message":
		return "system", firstNonEmpty(data.Content, data.Message, "System message")
	case "session.context_changed":
		return "session", copilotContextChanged(data)
	case "session.shutdown":
		return "session", copilotShutdownSummary(data)
	case "session.compaction_start":
		return "session", fmt.Sprintf(
			"Started context compaction\nsystem tokens: %d\nconversation tokens: %d\ntool definition tokens: %d",
			data.SystemTokens,
			data.ConversationTokens,
			data.ToolDefinitionsTokens,
		)
	case "session.compaction_complete":
		usage := data.CompactionTokensUsed.usage()
		return "session", fmt.Sprintf(
			"Completed context compaction #%d\npre-compaction tokens: %d\ncompaction tokens input:%d output:%d cache read:%d cache write:%d",
			data.CheckpointNumber,
			data.PreCompactionTokens,
			usage.InputTokens,
			usage.OutputTokens,
			usage.CacheReads,
			usage.CacheWrites,
		)
	case "abort":
		return "error", "Aborted: " + emptyDash(data.Reason)
	case "system.notification":
		return "system", "System notification"
	case "skill.invoked":
		return "session", "Skill invoked: " + emptyDash(data.Name)
	}
	return "", ""
}

func copilotShutdownSummary(data copilotMessageData) string {
	parts := []string{"Session shutdown: " + emptyDash(data.ShutdownType)}
	if data.CurrentModel != "" {
		parts = append(parts, "model: "+data.CurrentModel)
	}
	if data.CurrentTokens > 0 {
		parts = append(parts, fmt.Sprintf(
			"context tokens: current:%d system:%d conversation:%d tools:%d",
			data.CurrentTokens,
			data.SystemTokens,
			data.ConversationTokens,
			data.ToolDefinitionsTokens,
		))
	}
	reasoning := copilotReasoningTokens(data)
	if reasoning > 0 {
		parts = append(parts, fmt.Sprintf("reasoning tokens: %d", reasoning))
	}
	if data.TotalPremiumRequests > 0 {
		parts = append(parts, fmt.Sprintf("premium requests: %d", data.TotalPremiumRequests))
	}
	if data.TotalApiDurationMs > 0 {
		parts = append(parts, "API duration: "+models.FormatDuration(time.Duration(data.TotalApiDurationMs)*time.Millisecond))
	}
	if data.CodeChanges.LinesAdded > 0 || data.CodeChanges.LinesRemoved > 0 || len(data.CodeChanges.FilesModified) > 0 {
		parts = append(parts, fmt.Sprintf(
			"code changes: +%d -%d files:%d",
			data.CodeChanges.LinesAdded,
			data.CodeChanges.LinesRemoved,
			len(data.CodeChanges.FilesModified),
		))
	}
	return strings.Join(parts, "\n")
}

func copilotReasoningTokens(data copilotMessageData) int {
	total := 0
	for _, metric := range data.ModelMetrics {
		total += metric.Usage.ReasoningTokens
	}
	return total
}

func copilotToolCompletion(data copilotMessageData, toolNames map[string]string) string {
	label := toolNames[data.ToolCallID]
	if label == "" {
		label = copilotToolLabel(data)
	}

	status := "completed"
	if data.Success != nil && !*data.Success {
		status = "failed"
	}

	parts := []string{fmt.Sprintf("Tool %s: %s", status, label)}
	if data.Model != "" {
		parts = append(parts, "model: "+data.Model)
	}
	if errText := copilotErrorText(data.Error); errText != "" {
		parts = append(parts, "error: "+errText)
	}
	if result := semanticJSONSummary(data.Result, "result"); result != "" {
		parts = append(parts, result)
	}
	if metrics := copilotTelemetrySummary(data.ToolTelemetry); metrics != "" {
		parts = append(parts, metrics)
	}
	return strings.Join(parts, "\n")
}

func copilotToolLabel(data copilotMessageData) string {
	if data.MCPServerName != "" && data.MCPToolName != "" {
		return data.MCPServerName + "." + data.MCPToolName
	}
	if data.ToolName != "" {
		return data.ToolName
	}
	if data.ToolCallID != "" {
		return data.ToolCallID
	}
	return "unknown"
}

func copilotToolLabelWithCache(data copilotMessageData, toolNames map[string]string) string {
	if label := toolNames[data.ToolCallID]; label != "" {
		return label
	}
	return copilotToolLabel(data)
}

func copilotAgentLabel(data copilotMessageData) string {
	if data.AgentDisplayName != "" {
		return data.AgentDisplayName
	}
	return emptyDash(data.AgentName)
}

func copilotSubagentMeta(data copilotMessageData, lifecycle string) models.ActivityMeta {
	return models.ActivityMeta{
		Kind:          models.ActivityKindSubagent,
		Lifecycle:     lifecycle,
		ID:            firstNonEmpty(data.SubagentID, data.AgentID, data.ToolCallID),
		ParentID:      data.ParentToolCallID,
		InteractionID: data.InteractionID,
		Label:         copilotAgentLabel(data),
	}
}

func copilotTaskComplete(data copilotMessageData) string {
	status := "Task complete"
	if data.Success != nil && !*data.Success {
		status = "Task incomplete"
	}
	if data.Message != "" {
		return status + "\n" + data.Message
	}
	if data.Summary != "" {
		return status + "\n" + data.Summary
	}
	if data.Content != "" {
		return status + "\n" + data.Content
	}
	return status
}

func copilotInfo(data copilotMessageData) string {
	if data.Message == "" {
		return "Session info: " + emptyDash(data.InfoType)
	}
	return fmt.Sprintf("Session info: %s\n%s", emptyDash(data.InfoType), data.Message)
}

func copilotContextChanged(data copilotMessageData) string {
	cwd := firstNonEmpty(data.GitRoot, data.Context.GitRoot, data.CWD, data.Context.CWD)
	repo := firstNonEmpty(data.Repository, data.Context.Repository)
	branch := firstNonEmpty(data.Branch, data.Context.Branch)
	parts := []string{"Context changed"}
	if repo != "" {
		parts = append(parts, "repository: "+repo)
	}
	if branch != "" {
		parts = append(parts, "branch: "+branch)
	}
	if cwd != "" {
		parts = append(parts, "path: "+cwd)
	}
	return strings.Join(parts, "\n")
}

func copilotErrorText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return string(raw)
	}
	for _, key := range []string{"message", "type", "error"} {
		if value, ok := obj[key].(string); ok && value != "" {
			return value
		}
	}
	return string(raw)
}

func copilotTelemetrySummary(telemetry map[string]any) string {
	if len(telemetry) == 0 {
		return ""
	}
	metrics, _ := telemetry["metrics"].(map[string]any)
	if len(metrics) == 0 {
		return ""
	}
	keys := []string{
		"linesAdded",
		"linesRemoved",
		"rowsReturned",
		"rowsAffected",
		"file_count",
		"resultLength",
		"resultForLlmLength",
		"result_length",
		"elapsed_seconds",
		"total_turns",
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if value, ok := metrics[key]; ok {
			parts = append(parts, fmt.Sprintf("%s:%v", key, value))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "telemetry: " + strings.Join(parts, " ")
}

var semanticFieldOrder = []string{
	"intent",
	"goal",
	"description",
	"explanation",
	"summary",
	"query",
	"pattern",
	"prompt",
	"input",
	"command",
	"filePath",
	"path",
	"content",
	"text",
	"message",
	"value",
	"result",
	"output",
	"detailedContent",
}

func semanticJSONSummary(raw json.RawMessage, fallbackLabel string) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		text := strings.TrimSpace(string(raw))
		if text == "" {
			return ""
		}
		return formatSemanticLine(fallbackLabel, text)
	}
	label, text := semanticValueSummary(value, fallbackLabel)
	if text == "" {
		return ""
	}
	return formatSemanticLine(label, text)
}

func firstRawMessage(values ...json.RawMessage) json.RawMessage {
	for _, value := range values {
		if len(value) > 0 && string(value) != "null" {
			return value
		}
	}
	return nil
}

func semanticValueSummary(value any, fallbackLabel string) (string, string) {
	switch typed := value.(type) {
	case string:
		return fallbackLabel, typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			_, text := semanticValueSummary(item, fallbackLabel)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return fallbackLabel, strings.Join(parts, "\n")
	case map[string]any:
		for _, key := range semanticFieldOrder {
			if text := semanticMapText(typed, key, fallbackLabel); text != "" {
				return semanticDisplayLabel(key, fallbackLabel), text
			}
		}
		return "", ""
	case nil:
		return "", ""
	default:
		return fallbackLabel, fmt.Sprint(typed)
	}
}

func semanticMapText(values map[string]any, key, fallbackLabel string) string {
	value, ok := values[key]
	if !ok {
		return ""
	}
	_, text := semanticValueSummary(value, fallbackLabel)
	return text
}

func semanticDisplayLabel(key, fallbackLabel string) string {
	if fallbackLabel == "result" {
		switch key {
		case "content", "text", "message", "value", "result", "output", "detailedContent":
			return fallbackLabel
		}
	}
	return key
}

func formatSemanticLine(label, text string) string {
	text = summarizeSemanticText(text, 240)
	if text == "" {
		return ""
	}
	if label == "" {
		label = "value"
	}
	return label + ": " + text
}

func summarizeSemanticText(text string, width int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " · "))
	text = strings.Join(strings.Fields(text), " ")
	if text == "" || len([]rune(text)) <= width {
		return text
	}
	runes := []rune(text)
	if width <= 1 {
		return "."
	}
	if width <= 3 {
		return strings.Repeat(".", width)
	}
	return string(runes[:width-3]) + "..."
}

func emptyDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func applyCopilotMetrics(session *models.Session, data copilotMessageData) {
	if len(data.ModelMetrics) > 0 {
		session.TotalTokens = models.TokenUsage{}
	}
	for _, metric := range data.ModelMetrics {
		session.TotalTokens.InputTokens += metric.Usage.InputTokens
		session.TotalTokens.OutputTokens += metric.Usage.OutputTokens
		session.TotalTokens.CacheReads += metric.Usage.CacheReadTokens
		session.TotalTokens.CacheWrites += metric.Usage.CacheWriteTokens
	}
	if session.TotalTokens.InputTokens == 0 {
		session.TotalTokens.InputTokens = copilotInputTokens(data)
	}
}

func (t copilotCompactionTokens) usage() models.TokenUsage {
	input := t.InputTokens
	if input == 0 {
		input = t.Input
	}
	output := t.OutputTokens
	if output == 0 {
		output = t.Output
	}
	cacheReads := t.CacheReadTokens
	if cacheReads == 0 {
		cacheReads = t.CachedInput
	}
	return models.TokenUsage{
		InputTokens:  input,
		OutputTokens: output,
		CacheReads:   cacheReads,
		CacheWrites:  t.CacheWriteTokens,
	}
}

func addTokenUsage(total *models.TokenUsage, usage models.TokenUsage) {
	total.InputTokens += usage.InputTokens
	total.OutputTokens += usage.OutputTokens
	total.CacheReads += usage.CacheReads
	total.CacheWrites += usage.CacheWrites
}

func copilotInputTokens(data copilotMessageData) int {
	if data.CurrentTokens > 0 {
		return data.CurrentTokens
	}
	return data.SystemTokens + data.ConversationTokens + data.ToolDefinitionsTokens
}

func latestTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
