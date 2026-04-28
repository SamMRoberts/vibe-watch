package agents

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

type AmazonQDetector struct{}

func NewAmazonQDetector() *AmazonQDetector { return &AmazonQDetector{} }

func (c *AmazonQDetector) Name() string { return "Amazon Q" }

func (c *AmazonQDetector) Detect() ([]*models.Session, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	baseDir := filepath.Join(home, ".aws", "amazonq")
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return nil, nil
	}

	files, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	var sessions []*models.Session
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		logPath := filepath.Join(baseDir, f.Name())
		info, err := f.Info()
		if err != nil {
			continue
		}

		id := fmt.Sprintf("%x", md5.Sum([]byte(logPath)))
		session := &models.Session{
			ID:          id,
			AgentType:   models.AgentAmazonQ,
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
