package agents

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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
	HasPendingEdits   bool   `json:"hasPendingEdits"`
	LastResponseState int    `json:"lastResponseState"`
	Timing            struct {
		Created            int64 `json:"created"`
		LastRequestStarted int64 `json:"lastRequestStarted"`
		LastRequestEnded   int64 `json:"lastRequestEnded"`
	} `json:"timing"`
	Stats struct {
		FileCount int `json:"fileCount"`
		Added     int `json:"added"`
		Removed   int `json:"removed"`
	} `json:"stats"`
}

type copilotChatMetadata struct {
	FirstUserMessage string `json:"firstUserMessage"`
	WrittenToDisc    bool   `json:"writtenToDisc"`
}

type copilotChatWorkspace struct {
	Root        string
	ProjectPath string
	StateDBPath string
}

type copilotChatWorkspaceFile struct {
	Folder    string `json:"folder"`
	Workspace string `json:"workspace"`
}

type copilotChatPatch struct {
	Kind  int             `json:"kind"`
	Path  []any           `json:"k"`
	At    *int            `json:"i"`
	Value json.RawMessage `json:"v"`
}

type copilotChatSessionState struct {
	Version           int                   `json:"version"`
	CreationDate      int64                 `json:"creationDate"`
	InitialLocation   string                `json:"initialLocation"`
	ResponderUsername string                `json:"responderUsername"`
	SessionID         string                `json:"sessionId"`
	HasPendingEdits   bool                  `json:"hasPendingEdits"`
	Requests          []copilotChatRequest  `json:"requests"`
	PendingRequests   []json.RawMessage     `json:"pendingRequests"`
	InputState        copilotChatInputState `json:"inputState"`
	CustomTitle       string                `json:"customTitle"`
}

type copilotChatInputState struct {
	InputText     string           `json:"inputText"`
	SelectedModel copilotChatModel `json:"selectedModel"`
}

type copilotChatModel struct {
	Identifier string `json:"identifier"`
}

type copilotChatRequest struct {
	RequestID        string                `json:"requestId"`
	ResponseID       string                `json:"responseId"`
	Timestamp        int64                 `json:"timestamp"`
	Message          any                   `json:"message"`
	ModelID          string                `json:"modelId"`
	ModelState       copilotChatModelState `json:"modelState"`
	CompletionTokens int                   `json:"completionTokens"`
	ElapsedMs        int                   `json:"elapsedMs"`
	Result           map[string]any        `json:"result"`
	Response         []copilotChatPart     `json:"response"`
}

type copilotChatModelState struct {
	ModelID string `json:"modelId"`
	Model   string `json:"model"`
}

type copilotChatPart struct {
	Kind              string          `json:"kind"`
	Value             any             `json:"value"`
	ToolCallID        string          `json:"toolCallId"`
	ToolID            string          `json:"toolId"`
	InvocationMessage any             `json:"invocationMessage"`
	PastTenseMessage  any             `json:"pastTenseMessage"`
	IsComplete        *bool           `json:"isComplete"`
	IsConfirmed       *bool           `json:"isConfirmed"`
	ResultDetails     json.RawMessage `json:"resultDetails"`
}

type copilotChatTranscriptEvent struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	ParentID  string          `json:"parentId"`
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

type copilotChatTranscriptData struct {
	SessionID      string               `json:"sessionId"`
	Producer       string               `json:"producer"`
	CopilotVersion string               `json:"copilotVersion"`
	VSCodeVersion  string               `json:"vscodeVersion"`
	StartTime      string               `json:"startTime"`
	Content        string               `json:"content"`
	Attachments    []json.RawMessage    `json:"attachments"`
	MessageID      string               `json:"messageId"`
	ReasoningText  string               `json:"reasoningText"`
	ToolRequests   []copilotToolRequest `json:"toolRequests"`
	TurnID         string               `json:"turnId"`
	ToolCallID     string               `json:"toolCallId"`
	ToolName       string               `json:"toolName"`
	Arguments      json.RawMessage      `json:"arguments"`
	Success        *bool                `json:"success"`
	Result         json.RawMessage      `json:"result"`
	ResultDetails  json.RawMessage      `json:"resultDetails"`
}

func (c *CopilotChatDetector) Detect() ([]*models.Session, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	workspaceStorageDir := filepath.Join(home, "Library", "Application Support", "Code", "User", "workspaceStorage")
	workspaces, err := discoverCopilotChatWorkspaces(workspaceStorageDir)
	if err != nil {
		return nil, err
	}
	workspaces = filterCopilotChatWorkspacesForCWD(workspaces)

	var sessions []*models.Session
	for _, workspace := range workspaces {
		indexData, err := readVSCodeStateValue(workspace.StateDBPath, copilotChatStateKey)
		if err != nil || len(strings.TrimSpace(string(indexData))) == 0 {
			continue
		}
		workspaceSessions, err := parseCopilotChatWorkspace(indexData, nil, workspace.ProjectPath, workspace.StateDBPath, workspace.Root)
		if err != nil {
			continue
		}
		sessions = append(sessions, workspaceSessions...)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastUpdated.After(sessions[j].LastUpdated)
	})
	return sessions, nil
}

func discoverCopilotChatWorkspaces(workspaceStorageDir string) ([]copilotChatWorkspace, error) {
	entries, err := os.ReadDir(workspaceStorageDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var workspaces []copilotChatWorkspace
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		root := filepath.Join(workspaceStorageDir, entry.Name())
		stateDBPath := filepath.Join(root, "state.vscdb")
		info, err := os.Stat(stateDBPath)
		if err != nil || info.Size() == 0 {
			continue
		}
		projectPath := readVSCodeWorkspacePath(filepath.Join(root, "workspace.json"))
		if projectPath == "" {
			projectPath = entry.Name()
		}
		workspaces = append(workspaces, copilotChatWorkspace{
			Root:        root,
			ProjectPath: projectPath,
			StateDBPath: stateDBPath,
		})
	}
	return workspaces, nil
}

func filterCopilotChatWorkspacesForCWD(workspaces []copilotChatWorkspace) []copilotChatWorkspace {
	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		return workspaces
	}
	if abs, err := filepath.Abs(cwd); err == nil {
		cwd = abs
	}

	filtered := make([]copilotChatWorkspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		if workspaceMatchesPath(workspace.ProjectPath, cwd) {
			filtered = append(filtered, workspace)
		}
	}
	if len(filtered) == 0 {
		return workspaces
	}
	return filtered
}

func workspaceMatchesPath(workspacePath, path string) bool {
	if workspacePath == "" || path == "" {
		return false
	}
	workspacePath = filepath.Clean(workspacePath)
	path = filepath.Clean(path)
	if workspacePath == path {
		return true
	}
	if !filepath.IsAbs(workspacePath) || !filepath.IsAbs(path) {
		return false
	}
	if rel, err := filepath.Rel(workspacePath, path); err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return true
	}
	if rel, err := filepath.Rel(path, workspacePath); err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return true
	}
	return false
}

func readVSCodeWorkspacePath(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var workspace copilotChatWorkspaceFile
	if err := json.Unmarshal(data, &workspace); err != nil {
		return ""
	}
	return fileURIPath(firstNonEmpty(workspace.Folder, workspace.Workspace))
}

func fileURIPath(value string) string {
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "file" {
		return value
	}
	return parsed.Path
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
	return parseCopilotChatWorkspace(indexData, metadata, projectPath, logPath, "")
}

func parseCopilotChatWorkspace(indexData []byte, metadata map[string]copilotChatMetadata, projectPath, logPath, workspaceRoot string) ([]*models.Session, error) {
	var store copilotChatStore
	if err := json.Unmarshal(indexData, &store); err != nil {
		return nil, err
	}

	sessions := make([]*models.Session, 0, len(store.Entries))
	for key, entry := range store.Entries {
		sessionID := firstNonEmpty(entry.SessionID, key)
		if sessionID == "" || entry.IsEmpty {
			continue
		}

		chatState := parseCopilotChatSessionFile(filepath.Join(workspaceRoot, "chatSessions", sessionID+".jsonl"))
		transcriptPath := filepath.Join(workspaceRoot, "GitHub.copilot-chat", "transcripts", sessionID+".jsonl")
		transcriptMessages, transcriptStart, transcriptUpdated := parseCopilotChatTranscript(transcriptPath)

		createdAt := latestTime(timeFromMillis(entry.Timing.Created), chatState.createdAt())
		if createdAt.IsZero() {
			createdAt = transcriptStart
		}
		lastUpdated := latestTime(timeFromMillis(entry.LastMessageDate), timeFromMillis(entry.Timing.LastRequestEnded))
		lastUpdated = latestTime(lastUpdated, timeFromMillis(entry.Timing.LastRequestStarted))
		lastUpdated = latestTime(lastUpdated, chatState.lastUpdated())
		lastUpdated = latestTime(lastUpdated, transcriptUpdated)
		if lastUpdated.IsZero() {
			lastUpdated = createdAt
		}

		sessionLogPath := logPath
		if workspaceRoot != "" {
			sessionLogPath = firstExistingPath(transcriptPath, filepath.Join(workspaceRoot, "chatSessions", sessionID+".jsonl"), logPath)
		}
		session := &models.Session{
			ID:          "vscode-copilot-chat-" + sessionID,
			AgentType:   models.AgentCopilotChat,
			ProjectPath: projectPath,
			LogPath:     sessionLogPath,
			StartTime:   createdAt,
			LastUpdated: lastUpdated,
			IsActive:    !lastUpdated.IsZero() && time.Since(lastUpdated) < 5*time.Minute,
		}

		session.TotalTokens.OutputTokens = chatState.totalCompletionTokens()
		if len(transcriptMessages) > 0 {
			session.Messages = transcriptMessages
		} else if stateMessages := copilotChatStateMessages(chatState); len(stateMessages) > 0 {
			session.Messages = stateMessages
		} else if message := copilotChatMessage(entry, metadata[sessionID], chatState); message != "" {
			session.Messages = append(session.Messages, models.Message{
				Role:      "user",
				Content:   message,
				Timestamp: createdAt,
			})
		}
		appendCopilotChatSummary(session, chatState, entry)

		sessions = append(sessions, session)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastUpdated.After(sessions[j].LastUpdated)
	})
	return sessions, nil
}

func firstExistingPath(paths ...string) string {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	for _, path := range paths {
		if path != "" {
			return path
		}
	}
	return ""
}

func parseCopilotChatTranscript(path string) ([]models.Message, time.Time, time.Time) {
	file, err := os.Open(path)
	if err != nil {
		return nil, time.Time{}, time.Time{}
	}
	defer file.Close()

	info, _ := file.Stat()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 8*1024*1024)

	var messages []models.Message
	var startTime time.Time
	var lastUpdated time.Time
	toolNames := make(map[string]string)
	hiddenEventParents := make(map[string]string)
	toolRequestParents := make(map[string]string)
	endedTurns := make(map[string]bool)
	turnEndEvents := make(map[string]string)
	currentUserEventID := ""
	activeTurnID := ""
	activeTurnParentID := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event copilotChatTranscriptEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		timestamp := parseCopilotTime(event.Timestamp)
		if timestamp.IsZero() && info != nil {
			timestamp = info.ModTime()
		}
		if startTime.IsZero() || (!timestamp.IsZero() && timestamp.Before(startTime)) {
			startTime = timestamp
		}
		lastUpdated = latestTime(lastUpdated, timestamp)

		var data copilotChatTranscriptData
		if len(event.Data) > 0 {
			_ = json.Unmarshal(event.Data, &data)
		}

		switch event.Type {
		case "session.start":
			start := parseCopilotTime(data.StartTime)
			if !start.IsZero() {
				startTime = start
			}
			if event.ID != "" {
				hiddenEventParents[event.ID] = event.ParentID
			}
		case "user.message":
			currentUserEventID = event.ID
			activeTurnID = ""
			activeTurnParentID = ""
			messages = append(messages, models.Message{
				Role:      "user",
				Content:   data.Content,
				Timestamp: timestamp,
				Meta:      transcriptEventMeta(event, resolveHiddenEventParent(event.ParentID, hiddenEventParents)),
			})
		case "assistant.turn_start":
			if event.ID != "" {
				parentID := resolveHiddenEventParent(event.ParentID, hiddenEventParents)
				if parentTurnID := turnEndEvents[event.ParentID]; parentTurnID != "" && endedTurns[parentTurnID] {
					parentID = currentUserEventID
				}
				hiddenEventParents[event.ID] = parentID
				activeTurnID = data.TurnID
				activeTurnParentID = parentID
			}
		case "assistant.turn_end":
			if event.ID != "" {
				hiddenEventParents[event.ID] = resolveHiddenEventParent(event.ParentID, hiddenEventParents)
				turnEndEvents[event.ID] = data.TurnID
			}
			if data.TurnID != "" {
				endedTurns[data.TurnID] = true
			}
			if activeTurnID == data.TurnID {
				activeTurnID = ""
				activeTurnParentID = ""
			}
		case "assistant.message":
			content := data.Content
			if content == "" && len(data.ToolRequests) > 0 {
				content = summarizeToolRequests(data.ToolRequests)
			}
			if data.ReasoningText != "" {
				if content != "" {
					content += "\n"
				}
				content += "reasoning: " + data.ReasoningText
			}
			parentID := resolveHiddenEventParent(event.ParentID, hiddenEventParents)
			if activeTurnID != "" {
				parentID = activeTurnParentID
			}
			messages = append(messages, models.Message{
				Role:      "assistant",
				Content:   content,
				Timestamp: timestamp,
				Meta:      transcriptEventMeta(event, parentID),
			})
			if event.ID != "" {
				for _, request := range data.ToolRequests {
					if request.ToolCallID != "" {
						toolRequestParents[request.ToolCallID] = event.ID
					}
				}
			}
		case "tool.execution_start", "tool.execution_complete":
			activityData := copilotMessageData{
				ToolCallID: data.ToolCallID,
				ToolName:   data.ToolName,
				Arguments:  data.Arguments,
				Success:    data.Success,
				Result:     firstRawMessage(data.Result, data.ResultDetails),
			}
			if activity, ok := copilotActivityMessage(event.Type, activityData, toolNames); ok {
				activity.Timestamp = timestamp
				activity.Meta.EventID = event.ID
				activity.Meta.EventParentID = resolveHiddenEventParent(event.ParentID, hiddenEventParents)
				if event.Type == "tool.execution_start" && toolRequestParents[data.ToolCallID] != "" {
					activity.Meta.EventParentID = toolRequestParents[data.ToolCallID]
				}
				activity.Meta.RawParentID = event.ParentID
				messages = append(messages, activity)
			}
		}
	}

	if lastUpdated.IsZero() && info != nil {
		lastUpdated = info.ModTime()
	}
	return messages, startTime, lastUpdated
}

func transcriptEventMeta(event copilotChatTranscriptEvent, parentID string) models.ActivityMeta {
	return models.ActivityMeta{
		EventID:       event.ID,
		EventParentID: parentID,
		RawParentID:   event.ParentID,
	}
}

func resolveHiddenEventParent(parentID string, hiddenParents map[string]string) string {
	seen := make(map[string]bool)
	for parentID != "" && !seen[parentID] {
		next, ok := hiddenParents[parentID]
		if !ok {
			break
		}
		seen[parentID] = true
		parentID = next
	}
	return parentID
}

func parseCopilotChatSessionFile(path string) copilotChatSessionState {
	file, err := os.Open(path)
	if err != nil {
		return copilotChatSessionState{}
	}
	defer file.Close()

	var state copilotChatSessionState
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var patch copilotChatPatch
		if err := json.Unmarshal([]byte(line), &patch); err != nil {
			continue
		}
		applyCopilotChatPatch(&state, patch)
	}
	return state
}

func applyCopilotChatPatch(state *copilotChatSessionState, patch copilotChatPatch) {
	if patch.Kind == 0 {
		_ = json.Unmarshal(patch.Value, state)
		return
	}

	path := chatPatchPath(patch.Path)
	switch {
	case patch.Kind == 1 && len(path) == 1 && path[0] == "customTitle":
		_ = json.Unmarshal(patch.Value, &state.CustomTitle)
	case patch.Kind == 1 && len(path) == 1 && path[0] == "hasPendingEdits":
		_ = json.Unmarshal(patch.Value, &state.HasPendingEdits)
	case patch.Kind == 1 && len(path) == 2 && path[0] == "inputState" && path[1] == "inputText":
		_ = json.Unmarshal(patch.Value, &state.InputState.InputText)
	case patch.Kind == 1 && len(path) == 2 && path[0] == "inputState" && path[1] == "selectedModel":
		_ = json.Unmarshal(patch.Value, &state.InputState.SelectedModel)
	case patch.Kind == 2 && len(path) == 1 && path[0] == "requests":
		var requests []copilotChatRequest
		if err := json.Unmarshal(patch.Value, &requests); err == nil {
			if patch.At != nil {
				insertCopilotChatRequests(state, *patch.At, requests)
			} else {
				state.Requests = append(state.Requests, requests...)
			}
		}
	case len(path) >= 3 && path[0] == "requests":
		idx, err := strconv.Atoi(path[1])
		if err != nil {
			return
		}
		ensureCopilotChatRequest(state, idx)
		applyCopilotChatRequestPatch(&state.Requests[idx], path[2:], patch)
	}
}

func chatPatchPath(path []any) []string {
	parts := make([]string, 0, len(path))
	for _, part := range path {
		switch value := part.(type) {
		case string:
			parts = append(parts, value)
		case float64:
			parts = append(parts, strconv.Itoa(int(value)))
		}
	}
	return parts
}

func insertCopilotChatRequests(state *copilotChatSessionState, idx int, requests []copilotChatRequest) {
	if idx < 0 || idx > len(state.Requests) {
		state.Requests = append(state.Requests, requests...)
		return
	}
	state.Requests = append(state.Requests[:idx], append(requests, state.Requests[idx:]...)...)
}

func ensureCopilotChatRequest(state *copilotChatSessionState, idx int) {
	for len(state.Requests) <= idx {
		state.Requests = append(state.Requests, copilotChatRequest{})
	}
}

func applyCopilotChatRequestPatch(request *copilotChatRequest, path []string, patch copilotChatPatch) {
	if len(path) == 0 || patch.Kind != 1 && patch.Kind != 2 {
		return
	}
	switch path[0] {
	case "completionTokens":
		_ = json.Unmarshal(patch.Value, &request.CompletionTokens)
	case "elapsedMs":
		_ = json.Unmarshal(patch.Value, &request.ElapsedMs)
	case "modelState":
		_ = json.Unmarshal(patch.Value, &request.ModelState)
	case "result":
		_ = json.Unmarshal(patch.Value, &request.Result)
	case "response":
		if patch.Kind == 2 {
			var parts []copilotChatPart
			if err := json.Unmarshal(patch.Value, &parts); err == nil {
				if patch.At != nil && *patch.At >= 0 && *patch.At <= len(request.Response) {
					request.Response = append(request.Response[:*patch.At], append(parts, request.Response[*patch.At:]...)...)
				} else {
					request.Response = append(request.Response, parts...)
				}
			}
		}
	}
}

func (s copilotChatSessionState) createdAt() time.Time {
	return timeFromMillis(s.CreationDate)
}

func (s copilotChatSessionState) lastUpdated() time.Time {
	var last time.Time
	for _, request := range s.Requests {
		last = latestTime(last, timeFromMillis(request.Timestamp))
	}
	return last
}

func (s copilotChatSessionState) totalCompletionTokens() int {
	total := 0
	for _, request := range s.Requests {
		total += request.CompletionTokens
	}
	return total
}

func copilotChatMessage(entry copilotChatEntry, metadata copilotChatMetadata, state copilotChatSessionState) string {
	if metadata.FirstUserMessage != "" {
		return metadata.FirstUserMessage
	}
	for _, request := range state.Requests {
		if message := copilotChatRequestText(request.Message); message != "" {
			return message
		}
	}
	if state.InputState.InputText != "" {
		return state.InputState.InputText
	}
	if state.CustomTitle != "" {
		return state.CustomTitle
	}
	return entry.Title
}

func copilotChatRequestText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]any:
		for _, key := range []string{"text", "content", "value", "message", "prompt", "query", "input", "description", "goal"} {
			if text, ok := typed[key].(string); ok && text != "" {
				return text
			}
		}
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := copilotChatRequestText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func copilotChatStateMessages(state copilotChatSessionState) []models.Message {
	messages := make([]models.Message, 0, len(state.Requests)*2)
	for _, request := range state.Requests {
		timestamp := timeFromMillis(request.Timestamp)
		if content := copilotChatRequestText(request.Message); content != "" {
			messages = append(messages, models.Message{Role: "user", Content: content, Timestamp: timestamp})
		}
		for _, part := range request.Response {
			if msg, ok := copilotChatPartMessage(part, timestamp); ok {
				messages = append(messages, msg)
			}
		}
	}
	return messages
}

func copilotChatPartMessage(part copilotChatPart, timestamp time.Time) (models.Message, bool) {
	switch part.Kind {
	case "thinking":
		if content := copilotChatRequestText(part.Value); content != "" {
			return models.Message{Role: "assistant", Content: "reasoning: " + content, Timestamp: timestamp}, true
		}
	case "toolInvocationSerialized":
		label := firstNonEmpty(part.ToolID, part.ToolCallID, "tool")
		content := copilotChatRequestText(part.InvocationMessage)
		lifecycle := models.ActivityLifecycleStarted
		if complete := copilotChatRequestText(part.PastTenseMessage); complete != "" {
			content = complete
			lifecycle = models.ActivityLifecycleCompleted
		} else if part.IsComplete != nil && *part.IsComplete {
			lifecycle = models.ActivityLifecycleCompleted
		}
		if content == "" {
			content = "Tool " + lowerChatLifecycle(lifecycle) + ": " + label
		}
		if result := semanticJSONSummary(part.ResultDetails, "result"); result != "" {
			content += "\n" + result
		}
		return models.Message{
			Role:      "tool",
			Content:   content,
			Timestamp: timestamp,
			Meta: models.ActivityMeta{
				Kind:      models.ActivityKindTool,
				Lifecycle: lifecycle,
				ID:        part.ToolCallID,
				Label:     label,
			},
		}, true
	default:
		if content := copilotChatRequestText(part.Value); content != "" {
			return models.Message{Role: "assistant", Content: content, Timestamp: timestamp}, true
		}
	}
	return models.Message{}, false
}

func lowerChatLifecycle(lifecycle string) string {
	switch lifecycle {
	case models.ActivityLifecycleCompleted:
		return "completed"
	case models.ActivityLifecycleFailed:
		return "failed"
	case models.ActivityLifecycleRequested:
		return "requested"
	default:
		return "started"
	}
}

func appendCopilotChatSummary(session *models.Session, state copilotChatSessionState, entry copilotChatEntry) {
	parts := make([]string, 0, 6)
	if model := copilotChatModelID(state); model != "" {
		parts = append(parts, "model: "+model)
	}
	if len(state.Requests) > 0 {
		parts = append(parts, fmt.Sprintf("requests: %d", len(state.Requests)))
	}
	if session.TotalTokens.OutputTokens > 0 {
		parts = append(parts, fmt.Sprintf("completion tokens: %d", session.TotalTokens.OutputTokens))
	}
	if elapsed := copilotChatElapsed(state); elapsed > 0 {
		parts = append(parts, "elapsed: "+models.FormatDuration(time.Duration(elapsed)*time.Millisecond))
	}
	if entry.HasPendingEdits || state.HasPendingEdits {
		parts = append(parts, "pending edits")
	}
	if entry.Stats.FileCount > 0 || entry.Stats.Added > 0 || entry.Stats.Removed > 0 {
		parts = append(parts, fmt.Sprintf("edits: files:%d +%d -%d", entry.Stats.FileCount, entry.Stats.Added, entry.Stats.Removed))
	}
	if len(parts) == 0 {
		return
	}
	session.Messages = append(session.Messages, models.Message{
		Role:      "session",
		Content:   "VS Code Copilot Chat\n" + strings.Join(parts, "\n"),
		Timestamp: session.LastUpdated,
	})
}

func copilotChatModelID(state copilotChatSessionState) string {
	for i := len(state.Requests) - 1; i >= 0; i-- {
		request := state.Requests[i]
		if request.ModelID != "" {
			return request.ModelID
		}
		if request.ModelState.ModelID != "" {
			return request.ModelState.ModelID
		}
		if request.ModelState.Model != "" {
			return request.ModelState.Model
		}
	}
	return state.InputState.SelectedModel.Identifier
}

func copilotChatElapsed(state copilotChatSessionState) int {
	total := 0
	for _, request := range state.Requests {
		total += request.ElapsedMs
	}
	return total
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
