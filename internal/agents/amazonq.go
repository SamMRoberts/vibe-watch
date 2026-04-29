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
	candidates, err := c.sessionCandidates()
	if err != nil {
		return nil, err
	}

	var sessions []*models.Session
	for _, candidate := range candidates {
		candidateSessions, err := candidate.Parse()
		if err != nil {
			continue
		}
		sessions = append(sessions, candidateSessions...)
	}
	return sessions, nil
}

func (c *AmazonQDetector) sessionCandidates() ([]sessionCandidate, error) {
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

	var candidates []sessionCandidate
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		logPath := filepath.Join(baseDir, f.Name())
		info, err := f.Info()
		if err != nil {
			continue
		}

		fileName := f.Name()
		modTime := info.ModTime()
		candidates = append(candidates, sessionCandidate{
			UpdatedAt: modTime,
			Parse: func() ([]*models.Session, error) {
				id := fmt.Sprintf("%x", md5.Sum([]byte(logPath)))
				return []*models.Session{{
					ID:          id,
					AgentType:   models.AgentAmazonQ,
					ProjectPath: fileName,
					LogPath:     logPath,
					StartTime:   modTime,
					LastUpdated: modTime,
					IsActive:    time.Since(modTime) < 5*time.Minute,
				}}, nil
			},
		})
	}
	return candidates, nil
}
