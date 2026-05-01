package analysis

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/codex/events"
	"github.com/SamMRoberts/vibe-watch/internal/codex/sessions"
)

type builder struct {
	root         string
	summary      ScanSummary
	metrics      Metrics
	skippedFiles int
}

func newBuilder(root string) *builder {
	return &builder{
		root: root,
		metrics: Metrics{
			SessionsByDay: map[string]int{},
			EventTypes:    map[string]int{},
			Repositories:  map[string]int{},
			Models:        map[string]int{},
			Tools:         map[string]int{},
		},
	}
}

func (b *builder) AddFile(file sessions.File, parsed events.FileResult) {
	b.summary.FilesScanned++
	b.summary.SessionsScanned++
	b.summary.LinesScanned += parsed.Lines
	b.summary.EventsScanned += len(parsed.Events)
	b.summary.MalformedLines += parsed.MalformedLines

	dateKey := file.Date.Format(time.DateOnly)
	b.metrics.SessionsByDay[dateKey]++
	if b.summary.FirstDate.IsZero() || file.Date.Before(b.summary.FirstDate) {
		b.summary.FirstDate = file.Date
	}
	if b.summary.LastDate.IsZero() || file.Date.After(b.summary.LastDate) {
		b.summary.LastDate = file.Date
	}

	for _, event := range parsed.Events {
		b.addEvent(file, event)
	}
}

func (b *builder) Result() Result {
	b.summary.SessionRoot = b.root
	b.summary.SkippedFiles = b.skippedFiles
	b.metrics.DataQuality = DataQuality{
		FilesScanned:      b.summary.FilesScanned,
		LinesScanned:      b.summary.LinesScanned,
		MalformedLines:    b.summary.MalformedLines,
		UnknownEventTypes: b.metrics.UnknownEventTypes,
		MissingTimestamps: b.metrics.DataQuality.MissingTimestamps,
	}
	return Result{
		Summary:     b.summary,
		Metrics:     b.metrics,
		Suggestions: Suggest(b.summary, b.metrics),
	}
}

func (b *builder) addEvent(file sessions.File, event events.Event) {
	eventType := firstString(event.Raw, "type", "event", "kind")
	if eventType == "" {
		eventType = "unknown"
		b.metrics.UnknownEventTypes++
	}
	b.metrics.EventTypes[eventType]++
	if !hasAnyKey(event.Raw, "timestamp", "time", "created_at") {
		b.metrics.DataQuality.MissingTimestamps++
	}

	for _, repo := range findStringsByKeys(event.Raw, "cwd", "workdir", "repo", "repository") {
		if cleaned := cleanRepo(repo); cleaned != "" {
			b.metrics.Repositories[cleaned]++
		}
	}
	for _, model := range findStringsByKeys(event.Raw, "model") {
		if model != "" {
			b.metrics.Models[model]++
		}
	}
	for _, tool := range findStringsByKeys(event.Raw, "tool", "tool_name", "recipient_name") {
		if tool != "" {
			b.metrics.Tools[tool]++
		}
		if strings.Contains(strings.ToLower(tool), "exec_command") {
			if hasAnyKey(event.Raw, "cmd", "command") {
				b.metrics.ShellCommands++
			}
			if hasFailedExit(event.Raw) {
				b.metrics.FailedShellCommands++
			}
		}
	}

	lowerType := strings.ToLower(eventType)
	if strings.Contains(lowerType, "approval") || containsText(event.Raw, "require_escalated") {
		b.metrics.ApprovalEvents++
	}
	if strings.Contains(lowerType, "error") || containsText(event.Raw, "error") || containsText(event.Raw, "failed") {
		b.metrics.ErrorEvents++
	}
	if containsText(event.Raw, "go test") || containsText(event.Raw, "verification") || containsText(event.Raw, "passed") {
		b.metrics.VerificationSignals++
	}
	if strings.Contains(lowerType, "final") || containsText(event.Raw, "final answer") || containsText(event.Raw, "final response") {
		b.metrics.FinalResponses++
	}
	_ = file
}

func cleanRepo(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "/") {
		return filepath.Base(value)
	}
	return value
}

func firstString(value any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := asMap(value)[key]; ok {
			if s, ok := raw.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func findStringsByKeys(value any, keys ...string) []string {
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[strings.ToLower(key)] = true
	}
	seen := map[string]bool{}
	var found []string
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, raw := range typed {
				if wanted[strings.ToLower(key)] {
					if s, ok := raw.(string); ok {
						s = strings.TrimSpace(s)
						if s != "" && !seen[s] {
							seen[s] = true
							found = append(found, s)
						}
					}
				}
				walk(raw)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	sort.Strings(found)
	return found
}

func hasAnyKey(value any, keys ...string) bool {
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[strings.ToLower(key)] = true
	}
	var found bool
	var walk func(any)
	walk = func(current any) {
		if found {
			return
		}
		switch typed := current.(type) {
		case map[string]any:
			for key, raw := range typed {
				if wanted[strings.ToLower(key)] {
					found = true
					return
				}
				walk(raw)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	return found
}

func hasFailedExit(value any) bool {
	var failed bool
	var walk func(any)
	walk = func(current any) {
		if failed {
			return
		}
		switch typed := current.(type) {
		case map[string]any:
			for key, raw := range typed {
				if strings.EqualFold(key, "exit_code") || strings.EqualFold(key, "exitCode") {
					switch code := raw.(type) {
					case float64:
						failed = code != 0
					case int:
						failed = code != 0
					case string:
						failed = code != "" && code != "0"
					}
				}
				walk(raw)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	return failed
}

func containsText(value any, needle string) bool {
	needle = strings.ToLower(needle)
	var found bool
	var walk func(any)
	walk = func(current any) {
		if found {
			return
		}
		switch typed := current.(type) {
		case string:
			found = strings.Contains(strings.ToLower(typed), needle)
		case map[string]any:
			for _, raw := range typed {
				walk(raw)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case fmt.Stringer:
			found = strings.Contains(strings.ToLower(typed.String()), needle)
		}
	}
	walk(value)
	return found
}

func asMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return nil
}
