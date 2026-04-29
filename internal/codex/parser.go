package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/sessions"
	"github.com/SamMRoberts/vibe-watch/internal/tokens"
)

const activeWindow = 30 * time.Minute

func ParseFile(path string, now time.Time) (sessions.Summary, error) {
	file, err := os.Open(path)
	if err != nil {
		return sessions.Summary{}, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return sessions.Summary{}, err
	}
	summary, err := Parse(file, path, stat.ModTime(), now)
	if err != nil {
		return sessions.Summary{}, err
	}
	return summary, nil
}

func Parse(r io.Reader, path string, modTime time.Time, now time.Time) (sessions.Summary, error) {
	if now.IsZero() {
		now = time.Now()
	}
	summary := sessions.Summary{
		ID:        sessions.SessionID(path),
		Path:      path,
		Agent:     "codex",
		UpdatedAt: modTime,
		Status:    sessions.StatusUnknown,
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	lineNo := 0
	completed := false
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			addDiagnostic(&summary, lineNo, fmt.Sprintf("malformed JSONL: %v", err))
			continue
		}
		ts := parseTimestamp(record["timestamp"])
		if !ts.IsZero() && summary.StartedAt.IsZero() {
			summary.StartedAt = ts
		}
		payload, _ := record["payload"].(map[string]any)
		topType, _ := record["type"].(string)
		if topType == "session_meta" {
			handleSessionMeta(&summary, payload, ts)
			continue
		}
		if payload == nil {
			summary.Activities = append(summary.Activities, sessions.Activity{
				Kind:      sessions.ActivityOther,
				Timestamp: ts,
				Title:     topType,
				Body:      compactJSON(record),
			})
			continue
		}
		payloadType, _ := payload["type"].(string)
		switch payloadType {
		case "task_started":
			if started := parseTimestamp(payload["started_at"]); !started.IsZero() {
				summary.StartedAt = started
			}
			summary.Activities = append(summary.Activities, sessions.Activity{Kind: sessions.ActivityMetadata, Timestamp: ts, Title: "Task started", Body: stringValue(payload, "turn_id")})
		case "task_complete":
			completed = true
			summary.Activities = append(summary.Activities, sessions.Activity{Kind: sessions.ActivityMetadata, Timestamp: ts, Title: "Task complete", Body: stringValue(payload, "last_agent_message")})
		case "user_message":
			body := firstNonEmpty(stringValue(payload, "message"), textFromAny(payload["text_elements"]))
			addMessage(&summary, sessions.ActivityUserPrompt, ts, "user", "User prompt", body)
		case "agent_message":
			addMessage(&summary, sessions.ActivityAssistantMessage, ts, "assistant", "Assistant message", stringValue(payload, "message"))
		case "message":
			role := stringValue(payload, "role")
			body := textFromAny(payload["content"])
			kind := sessions.ActivityOther
			title := "Message"
			switch role {
			case "user":
				kind, title = sessions.ActivityUserPrompt, "User prompt"
			case "assistant":
				kind, title = sessions.ActivityAssistantMessage, "Assistant message"
			default:
				kind, title = sessions.ActivityMetadata, "Message: "+role
			}
			addMessage(&summary, kind, ts, role, title, body)
		case "function_call", "custom_tool_call":
			summary.ToolCallCount++
			name := stringValue(payload, "name")
			body := firstNonEmpty(textFromAny(payload["arguments"]), textFromAny(payload["input"]))
			summary.Activities = append(summary.Activities, sessions.Activity{Kind: sessions.ActivityToolCall, Timestamp: ts, Title: "Tool call: " + name, Body: body, ToolName: name, CallID: stringValue(payload, "call_id")})
		case "function_call_output", "custom_tool_call_output":
			summary.Activities = append(summary.Activities, sessions.Activity{Kind: sessions.ActivityToolResult, Timestamp: ts, Title: "Tool result", Body: textFromAny(payload["output"]), CallID: stringValue(payload, "call_id")})
		case "exec_command_end":
			body := firstNonEmpty(stringValue(payload, "formatted_output"), stringValue(payload, "aggregated_output"), stringValue(payload, "stdout"), stringValue(payload, "stderr"))
			title := "Command finished"
			if command := stringValue(payload, "command"); command != "" {
				title = "Command: " + command
			}
			summary.Activities = append(summary.Activities, sessions.Activity{Kind: sessions.ActivityToolResult, Timestamp: ts, Title: title, Body: body, CallID: stringValue(payload, "call_id")})
		case "token_count":
			summary.Tokens = tokens.FromInfo(payload["info"])
			summary.Activities = append(summary.Activities, sessions.Activity{Kind: sessions.ActivityTokenCount, Timestamp: ts, Title: "Token count", Body: tokenBody(summary.Tokens)})
		case "":
			summary.Activities = append(summary.Activities, sessions.Activity{Kind: sessions.ActivityMetadata, Timestamp: ts, Title: topType, Body: summarizeContextPayload(payload)})
		default:
			summary.Activities = append(summary.Activities, sessions.Activity{Kind: sessions.ActivityOther, Timestamp: ts, Title: payloadType, Body: compactJSON(payload)})
		}
	}
	if err := scanner.Err(); err != nil {
		addDiagnostic(&summary, lineNo, err.Error())
	}
	if summary.StartedAt.IsZero() {
		summary.StartedAt = timestampFromFilename(path)
	}
	if summary.UpdatedAt.IsZero() {
		summary.UpdatedAt = summary.StartedAt
	}
	switch {
	case len(summary.Diagnostics) > 0:
		summary.Status = sessions.StatusErrored
	case completed:
		summary.Status = sessions.StatusCompleted
	case !summary.UpdatedAt.IsZero() && now.Sub(summary.UpdatedAt) <= activeWindow:
		summary.Status = sessions.StatusActive
	default:
		summary.Status = sessions.StatusUnknown
	}
	return summary, nil
}

func handleSessionMeta(summary *sessions.Summary, payload map[string]any, ts time.Time) {
	if originator := stringValue(payload, "originator"); originator != "" {
		summary.Agent = originator
	}
	if summary.Agent == "" {
		summary.Agent = "codex"
	}
	if metaTS := parseTimestamp(payload["timestamp"]); !metaTS.IsZero() {
		summary.StartedAt = metaTS
	} else if !ts.IsZero() && summary.StartedAt.IsZero() {
		summary.StartedAt = ts
	}
	summary.Activities = append(summary.Activities, sessions.Activity{
		Kind:      sessions.ActivityMetadata,
		Timestamp: firstTime(summary.StartedAt, ts),
		Title:     "Session metadata",
		Body:      summarizeContextPayload(payload),
	})
}

func addMessage(summary *sessions.Summary, kind sessions.ActivityKind, ts time.Time, role, title, body string) {
	if body == "" {
		body = "(empty)"
	}
	if kind == sessions.ActivityUserPrompt && summary.PromptPreview == "" {
		summary.PromptPreview = truncate(oneLine(body), 120)
	}
	summary.Activities = append(summary.Activities, sessions.Activity{Kind: kind, Timestamp: ts, Role: role, Title: title, Body: body})
}

func addDiagnostic(summary *sessions.Summary, line int, message string) {
	diag := sessions.Diagnostic{Line: line, Message: message}
	summary.Diagnostics = append(summary.Diagnostics, diag)
	summary.Activities = append(summary.Activities, sessions.Activity{Kind: sessions.ActivityDiagnostic, Title: "Parser diagnostic", Body: message})
}

func parseTimestamp(v any) time.Time {
	s, ok := v.(string)
	if !ok || s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15-04-05"} {
		if ts, err := time.Parse(layout, s); err == nil {
			return ts
		}
	}
	return time.Time{}
}

func timestampFromFilename(path string) time.Time {
	base := filepath.Base(path)
	if len(base) < len("rollout-2006-01-02T15-04-05") {
		return time.Time{}
	}
	part := strings.TrimPrefix(base, "rollout-")
	if len(part) >= len("2006-01-02T15-04-05") {
		part = part[:len("2006-01-02T15-04-05")]
	}
	return parseTimestamp(part)
}

func stringValue(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return textFromAny(v)
}

func textFromAny(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if text := textFromAny(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		for _, key := range []string{"text", "content", "message", "summary", "output"} {
			if text := textFromAny(value[key]); text != "" {
				return text
			}
		}
		return compactJSON(value)
	default:
		return compactJSON(value)
	}
}

func compactJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func tokenBody(summary sessions.TokenSummary) string {
	if !summary.Known {
		return "tokens unavailable"
	}
	return fmt.Sprintf("total=%d input=%d output=%d reasoning=%d context=%d", summary.Total.TotalTokens, summary.Total.InputTokens, summary.Total.OutputTokens, summary.Total.ReasoningOutputTokens, summary.ModelContextWindow)
}

func summarizeContextPayload(payload map[string]any) string {
	parts := make([]string, 0, 4)
	for _, key := range []string{"cwd", "model", "model_provider", "originator", "source", "turn_id", "collaboration_mode"} {
		if value := stringValue(payload, key); value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	if len(parts) == 0 {
		return compactJSON(payload)
	}
	return strings.Join(parts, " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
