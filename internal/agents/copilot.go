package agents

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

type CopilotDetector struct{}

func NewCopilotDetector() *CopilotDetector { return &CopilotDetector{} }

func (c *CopilotDetector) Name() string { return "Copilot CLI" }

func (c *CopilotDetector) Detect() ([]*models.Session, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	logsDir := filepath.Join(home, ".config", "github-copilot", "logs")
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
			ProjectPath: f.Name(),
			LogPath:     logPath,
			StartTime:   info.ModTime(),
			LastUpdated: info.ModTime(),
			IsActive:    time.Since(info.ModTime()) < 5*time.Minute,
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}
