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
	Root      string           `json:"root"`
	CheckedAt time.Time        `json:"checked_at"`
	Sessions  []SessionSummary `json:"sessions"`
	Active    *SessionDetail   `json:"active,omitempty"`
}

type SessionSummary struct {
	ID      string    `json:"id"`
	Date    time.Time `json:"date"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	Events  int       `json:"events"`
	Bad     int       `json:"bad"`
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
	}
	if len(files) == 0 {
		return snapshot, nil
	}

	activeIndex := newestIndex(files)
	for index, file := range files {
		parsed, err := parseFile(file.Path)
		if err != nil {
			snapshot.Sessions = append(snapshot.Sessions, SessionSummary{
				ID:      file.ID,
				Date:    file.Date,
				Size:    file.Size,
				ModTime: file.ModTime,
				Bad:     1,
			})
			continue
		}
		summary := SessionSummary{
			ID:      file.ID,
			Date:    file.Date,
			Size:    file.Size,
			ModTime: file.ModTime,
			Events:  len(parsed.Events),
			Bad:     parsed.MalformedLines,
		}
		snapshot.Sessions = append(snapshot.Sessions, summary)
		if index == activeIndex {
			snapshot.Active = &SessionDetail{
				SessionSummary: summary,
				Recent:         summarizeRecent(parsed.Events, opts.EventLimit),
			}
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
	return EventSummary{
		Line:      event.Line,
		Type:      firstString(event.Raw, "type", "event", "kind", "unknown"),
		Timestamp: firstString(event.Raw, "timestamp", "time", "created_at"),
		Tool:      firstNestedString(event.Raw, "tool", "tool_name", "recipient_name"),
		Model:     firstNestedString(event.Raw, "model"),
		Repo:      cleanRepo(firstNestedString(event.Raw, "cwd", "workdir", "repo", "repository")),
	}
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
