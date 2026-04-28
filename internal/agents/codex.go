package agents

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

type CodexDetector struct{}

func NewCodexDetector() *CodexDetector { return &CodexDetector{} }

func (c *CodexDetector) Name() string { return "Codex CLI" }

type codexSession struct {
	ID       string `json:"id"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
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

	files, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, err
	}

	var sessions []*models.Session
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		logPath := filepath.Join(sessionsDir, f.Name())
		session, err := c.parseSession(logPath)
		if err != nil || session == nil {
			continue
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (c *CodexDetector) parseSession(logPath string) (*models.Session, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(logPath)
	if err != nil {
		return nil, err
	}

	var cs codexSession
	if err := json.Unmarshal(data, &cs); err != nil {
		return nil, err
	}

	id := cs.ID
	if id == "" {
		id = fmt.Sprintf("%x", md5.Sum([]byte(logPath)))
	}

	session := &models.Session{
		ID:          id,
		AgentType:   models.AgentCodex,
		ProjectPath: strings.TrimSuffix(filepath.Base(logPath), filepath.Ext(logPath)),
		LogPath:     logPath,
		LastUpdated: info.ModTime(),
		IsActive:    time.Since(info.ModTime()) < 5*time.Minute,
	}

	if cs.CreatedAt != "" {
		t, _ := time.Parse(time.RFC3339, cs.CreatedAt)
		session.StartTime = t
	}
	if session.StartTime.IsZero() {
		session.StartTime = info.ModTime()
	}

	for _, m := range cs.Messages {
		session.Messages = append(session.Messages, models.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	return session, nil
}
