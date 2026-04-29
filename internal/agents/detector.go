package agents

import (
	"sort"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

// AgentDetector is the interface for session parsers
type AgentDetector interface {
	Name() string
	Detect() ([]*models.Session, error)
}

type sessionCandidate struct {
	UpdatedAt time.Time
	Parse     func() ([]*models.Session, error)
}

type incrementalDetector interface {
	sessionCandidates() ([]sessionCandidate, error)
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
		NewCopilotChatDetector(),
		NewAmazonQDetector(),
	}
	return r
}

func (r *Registry) DetectAll() ([]*models.Session, error) {
	return r.DetectAllIncremental(nil)
}

func (r *Registry) DetectAllIncremental(publish func([]*models.Session)) ([]*models.Session, error) {
	candidates := r.sessionCandidates()
	if len(candidates) == 0 {
		return nil, nil
	}

	var all []*models.Session
	published := false
	currentDay := candidateDay(candidates[0].UpdatedAt)
	for _, candidate := range candidates {
		day := candidateDay(candidate.UpdatedAt)
		if day != currentDay {
			if len(all) > 0 && publish != nil {
				publish(sortedSessionCopy(all))
				published = true
			}
			currentDay = day
		}

		sessions, err := candidate.Parse()
		if err != nil {
			continue
		}
		all = append(all, sessions...)
	}

	sorted := sortedSessionCopy(all)
	if publish != nil && len(sorted) > 0 && !published {
		publish(sorted)
	}
	return sorted, nil
}

func (r *Registry) sessionCandidates() []sessionCandidate {
	var candidates []sessionCandidate
	for _, detector := range r.detectors {
		if incremental, ok := detector.(incrementalDetector); ok {
			detectorCandidates, err := incremental.sessionCandidates()
			if err != nil {
				continue
			}
			candidates = append(candidates, detectorCandidates...)
			continue
		}

		detector := detector
		candidates = append(candidates, sessionCandidate{
			Parse: detector.Detect,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].UpdatedAt.After(candidates[j].UpdatedAt)
	})
	return candidates
}

func candidateDay(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func sortedSessionCopy(sessions []*models.Session) []*models.Session {
	out := append([]*models.Session(nil), sessions...)
	sort.SliceStable(out, func(i, j int) bool {
		return sessionSortTime(out[i]).After(sessionSortTime(out[j]))
	})
	return out
}

func sessionSortTime(session *models.Session) time.Time {
	if session == nil {
		return time.Time{}
	}
	if !session.StartTime.IsZero() {
		return session.StartTime
	}
	return session.LastUpdated
}

func (r *Registry) Detectors() []AgentDetector {
	return r.detectors
}
