package watcher

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/codex/events"
	"github.com/SamMRoberts/vibe-watch/internal/codex/sessions"
)

type Options struct {
	Root       string
	Limit      int
	EventLimit int
}

type Snapshot struct {
	Root      string                   `json:"root"`
	CheckedAt time.Time                `json:"checked_at"`
	Sessions  []SessionSummary         `json:"sessions"`
	Active    *SessionDetail           `json:"active,omitempty"`
	Details   map[string]SessionDetail `json:"details,omitempty"`
}

type SessionSummary struct {
	ID            string    `json:"id"`
	Date          time.Time `json:"date"`
	Size          int64     `json:"size"`
	ModTime       time.Time `json:"mod_time"`
	Events        int       `json:"events"`
	Bad           int       `json:"bad"`
	Status        string    `json:"status"`
	Agent         string    `json:"agent"`
	RepoPath      string    `json:"repo_path,omitempty"`
	Model         string    `json:"model,omitempty"`
	LastEventType string    `json:"last_event_type,omitempty"`
}

type SessionDetail struct {
	SessionSummary
	Recent []EventSummary `json:"recent"`
}

type EventSummary struct {
	Line      int    `json:"line"`
	Type      string `json:"type"`
	Timestamp string `json:"timestamp,omitempty"`
	Tool      string `json:"tool,omitempty"`
	Model     string `json:"model,omitempty"`
	Repo      string `json:"repo,omitempty"`
	RepoPath  string `json:"repo_path,omitempty"`
}

func Capture(opts Options) (Snapshot, error) {
	if opts.EventLimit <= 0 {
		opts.EventLimit = 12
	}
	files, err := sessions.Discover(sessions.ScanOptions{
		Root:  opts.Root,
		Limit: opts.Limit,
	})
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{
		Root:      opts.Root,
		CheckedAt: time.Now(),
		Sessions:  make([]SessionSummary, 0, len(files)),
		Details:   map[string]SessionDetail{},
	}
	if len(files) == 0 {
		return snapshot, nil
	}

	activeIndex := newestIndex(files)
	for index, file := range files {
		parsed, err := parseFile(file.Path)
		if err != nil {
			summary := SessionSummary{
				ID:      file.ID,
				Date:    file.Date,
				Size:    file.Size,
				ModTime: file.ModTime,
				Status:  statusFor(index, activeIndex),
				Agent:   "Codex",
				Bad:     1,
			}
			snapshot.Sessions = append(snapshot.Sessions, summary)
			snapshot.Details[file.ID] = SessionDetail{SessionSummary: summary}
			continue
		}
		allEvents := summarizeRecent(parsed.Events, len(parsed.Events))
		recent := summarizeRecent(parsed.Events, opts.EventLimit)
		summary := SessionSummary{
			ID:            file.ID,
			Date:          file.Date,
			Size:          file.Size,
			ModTime:       file.ModTime,
			Events:        len(parsed.Events),
			Bad:           parsed.MalformedLines,
			Status:        statusFor(index, activeIndex),
			Agent:         "Codex",
			RepoPath:      latestField(allEvents, func(event EventSummary) string { return event.RepoPath }),
			Model:         latestField(allEvents, func(event EventSummary) string { return event.Model }),
			LastEventType: latestEventType(allEvents),
		}
		snapshot.Sessions = append(snapshot.Sessions, summary)
		detail := SessionDetail{
			SessionSummary: summary,
			Recent:         recent,
		}
		snapshot.Details[file.ID] = detail
		if index == activeIndex {
			snapshot.Active = &detail
		}
	}
	return snapshot, nil
}

func newestIndex(files []sessions.File) int {
	newest := 0
	for i := range files {
		if files[i].ModTime.After(files[newest].ModTime) {
			newest = i
			continue
		}
		if files[i].ModTime.Equal(files[newest].ModTime) && files[i].ID > files[newest].ID {
			newest = i
		}
	}
	return newest
}

func parseFile(path string) (events.FileResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return events.FileResult{}, err
	}
	defer file.Close()
	return events.ParseJSONL(file), nil
}

func summarizeRecent(source []events.Event, limit int) []EventSummary {
	start := len(source) - limit
	if start < 0 {
		start = 0
	}
	recent := make([]EventSummary, 0, len(source)-start)
	for _, event := range source[start:] {
		recent = append(recent, Summarize(event))
	}
	return recent
}

func Summarize(event events.Event) EventSummary {
	repoPath := firstNestedString(event.Raw, "cwd", "workdir", "repo", "repository")
	return EventSummary{
		Line:      event.Line,
		Type:      firstString(event.Raw, "type", "event", "kind", "unknown"),
		Timestamp: firstString(event.Raw, "timestamp", "time", "created_at"),
		Tool:      firstNestedString(event.Raw, "tool", "tool_name", "recipient_name"),
		Model:     firstNestedString(event.Raw, "model"),
		Repo:      cleanRepo(repoPath),
		RepoPath:  repoPath,
	}
}

func statusFor(index, activeIndex int) string {
	if index == activeIndex {
		return "active"
	}
	return "idle"
}

func latestField(events []EventSummary, value func(EventSummary) string) string {
	for i := len(events) - 1; i >= 0; i-- {
		if candidate := strings.TrimSpace(value(events[i])); candidate != "" {
			return candidate
		}
	}
	return ""
}

func latestEventType(events []EventSummary) string {
	if len(events) == 0 {
		return ""
	}
	return events[len(events)-1].Type
}

func firstString(raw map[string]any, keys ...string) string {
	fallback := ""
	if len(keys) > 0 && keys[len(keys)-1] == "unknown" {
		fallback = "unknown"
		keys = keys[:len(keys)-1]
	}
	for _, key := range keys {
		if value, ok := raw[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return fallback
}

func firstNestedString(value any, keys ...string) string {
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[strings.ToLower(key)] = true
	}
	var found []string
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, raw := range typed {
				if wanted[strings.ToLower(key)] {
					if value, ok := raw.(string); ok && strings.TrimSpace(value) != "" {
						found = append(found, strings.TrimSpace(value))
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
	if len(found) == 0 {
		return ""
	}
	return found[0]
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
