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

type ClaudeDetector struct{}

func NewClaudeDetector() *ClaudeDetector {
	return &ClaudeDetector{}
}

func (c *ClaudeDetector) Name() string { return "Claude Code" }

type claudeEntry struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Message   struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens         int `json:"input_tokens"`
			OutputTokens        int `json:"output_tokens"`
			CacheCreationTokens int `json:"cache_creation_input_tokens"`
			CacheReadTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

func (c *ClaudeDetector) Detect() ([]*models.Session, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	projectsDir := filepath.Join(home, ".claude", "projects")
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return nil, nil
	}

	var sessions []*models.Session

	projectEntries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}

	for _, projectEntry := range projectEntries {
		if !projectEntry.IsDir() {
			continue
		}
		projectDir := filepath.Join(projectsDir, projectEntry.Name())
		files, err := os.ReadDir(projectDir)
		if err != nil {
			continue
		}

		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}

			logPath := filepath.Join(projectDir, f.Name())
			session, err := c.parseSession(logPath, projectEntry.Name())
			if err != nil || session == nil {
				continue
			}
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

func (c *ClaudeDetector) parseSession(logPath, projectName string) (*models.Session, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	sessionID := fmt.Sprintf("%x", md5.Sum([]byte(logPath)))

	session := &models.Session{
		ID:          sessionID,
		AgentType:   models.AgentClaude,
		ProjectPath: projectName,
		LogPath:     logPath,
		LastUpdated: info.ModTime(),
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry claudeEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		var ts time.Time
		if entry.Timestamp != "" {
			ts, _ = time.Parse(time.RFC3339, entry.Timestamp)
			if ts.IsZero() {
				ts, _ = time.Parse(time.RFC3339Nano, entry.Timestamp)
			}
		}
		if ts.IsZero() {
			ts = info.ModTime()
		}

		if session.StartTime.IsZero() || ts.Before(session.StartTime) {
			session.StartTime = ts
		}

		if entry.Type == "assistant" || (entry.Message.Role == "assistant") {
			var content strings.Builder
			for _, c := range entry.Message.Content {
				if c.Type == "text" {
					content.WriteString(c.Text)
				}
			}

			msg := models.Message{
				Role:      "assistant",
				Content:   content.String(),
				Timestamp: ts,
				Tokens: models.TokenUsage{
					InputTokens:  entry.Message.Usage.InputTokens,
					OutputTokens: entry.Message.Usage.OutputTokens,
					CacheReads:   entry.Message.Usage.CacheReadTokens,
					CacheWrites:  entry.Message.Usage.CacheCreationTokens,
				},
			}
			session.Messages = append(session.Messages, msg)
			session.TotalTokens.InputTokens += entry.Message.Usage.InputTokens
			session.TotalTokens.OutputTokens += entry.Message.Usage.OutputTokens
			session.TotalTokens.CacheReads += entry.Message.Usage.CacheReadTokens
			session.TotalTokens.CacheWrites += entry.Message.Usage.CacheCreationTokens
		} else if entry.Type == "user" || entry.Message.Role == "user" {
			var content strings.Builder
			for _, c := range entry.Message.Content {
				if c.Type == "text" {
					content.WriteString(c.Text)
				}
			}
			msg := models.Message{
				Role:      "user",
				Content:   content.String(),
				Timestamp: ts,
			}
			session.Messages = append(session.Messages, msg)
		}
	}

	// Mark active if modified in last 5 minutes
	session.IsActive = time.Since(info.ModTime()) < 5*time.Minute
	session.CostUSD = session.EstimatedCost()

	return session, nil
}
