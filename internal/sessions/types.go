package sessions

import "time"

type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusErrored   Status = "errored"
	StatusUnknown   Status = "unknown"
)

type TokenUsage struct {
	Known                 bool `json:"known"`
	InputTokens           int  `json:"input_tokens,omitempty"`
	CachedInputTokens     int  `json:"cached_input_tokens,omitempty"`
	OutputTokens          int  `json:"output_tokens,omitempty"`
	ReasoningOutputTokens int  `json:"reasoning_output_tokens,omitempty"`
	TotalTokens           int  `json:"total_tokens,omitempty"`
}

type TokenSummary struct {
	Known              bool       `json:"known"`
	Latest             TokenUsage `json:"latest,omitempty"`
	Total              TokenUsage `json:"total,omitempty"`
	ModelContextWindow int        `json:"model_context_window,omitempty"`
}

type ActivityKind string

const (
	ActivityUserPrompt       ActivityKind = "user_prompt"
	ActivityAssistantMessage ActivityKind = "assistant_message"
	ActivityToolCall         ActivityKind = "tool_call"
	ActivityToolResult       ActivityKind = "tool_result"
	ActivityDiagnostic       ActivityKind = "diagnostic"
	ActivityMetadata         ActivityKind = "metadata"
	ActivityTokenCount       ActivityKind = "token_count"
	ActivityOther            ActivityKind = "other"
)

type Activity struct {
	Kind      ActivityKind `json:"kind"`
	Timestamp time.Time    `json:"timestamp,omitempty"`
	Role      string       `json:"role,omitempty"`
	Title     string       `json:"title,omitempty"`
	Body      string       `json:"body,omitempty"`
	ToolName  string       `json:"tool_name,omitempty"`
	CallID    string       `json:"call_id,omitempty"`
}

type Diagnostic struct {
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}

type Summary struct {
	ID            string       `json:"id"`
	Path          string       `json:"path"`
	Agent         string       `json:"agent"`
	StartedAt     time.Time    `json:"started_at,omitempty"`
	UpdatedAt     time.Time    `json:"updated_at,omitempty"`
	Status        Status       `json:"status"`
	PromptPreview string       `json:"prompt_preview,omitempty"`
	ToolCallCount int          `json:"tool_call_count"`
	Tokens        TokenSummary `json:"tokens"`
	Diagnostics   []Diagnostic `json:"diagnostics,omitempty"`
	Activities    []Activity   `json:"activities,omitempty"`
}

type FileInfo struct {
	ID      string
	Path    string
	Date    time.Time
	ModTime time.Time
}

func SessionID(path string) string {
	base := filepathBase(path)
	if len(base) > len(".jsonl") && base[len(base)-len(".jsonl"):] == ".jsonl" {
		return base[:len(base)-len(".jsonl")]
	}
	return base
}

func filepathBase(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
