package agents

import "github.com/SamMRoberts/vibe-watch/internal/models"

// AgentDetector is the interface for session parsers
type AgentDetector interface {
	Name() string
	Detect() ([]*models.Session, error)
}

// Registry holds all registered detectors
type Registry struct {
	detectors []AgentDetector
}

func NewRegistry() *Registry {
	r := &Registry{}
	r.detectors = []AgentDetector{
		NewClaudeDetector(),
		NewCodexDetector(),
		NewCopilotDetector(),
		NewAmazonQDetector(),
	}
	return r
}

func (r *Registry) DetectAll() ([]*models.Session, error) {
	var all []*models.Session
	for _, d := range r.detectors {
		sessions, err := d.Detect()
		if err != nil {
			continue
		}
		all = append(all, sessions...)
	}
	return all, nil
}

func (r *Registry) Detectors() []AgentDetector {
	return r.detectors
}
