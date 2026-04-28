package watcher

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/agents"
	"github.com/SamMRoberts/vibe-watch/internal/models"
)

type UpdateMsg struct {
	Sessions []*models.Session
	Err      error
}

type Watcher struct {
	registry    *agents.Registry
	interval    time.Duration
	projectRoot string
	mu          sync.RWMutex
	sessions    []*models.Session
	updates     chan UpdateMsg
	quit        chan struct{}
	stopOnce    sync.Once
}

type Option func(*Watcher)

func WithProjectRoot(projectRoot string) Option {
	return func(w *Watcher) {
		if projectRoot == "" {
			return
		}
		if abs, err := filepath.Abs(projectRoot); err == nil {
			projectRoot = abs
		}
		w.projectRoot = filepath.Clean(projectRoot)
	}
}

func New(registry *agents.Registry, interval time.Duration, opts ...Option) *Watcher {
	if interval < time.Second {
		interval = 2 * time.Second
	}
	w := &Watcher{
		registry: registry,
		interval: interval,
		updates:  make(chan UpdateMsg, 10),
		quit:     make(chan struct{}),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func (w *Watcher) Start() {
	go func() {
		// Initial load
		w.poll()

		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.poll()
			case <-w.quit:
				return
			}
		}
	}()
}

func (w *Watcher) Stop() {
	w.stopOnce.Do(func() { close(w.quit) })
}

func (w *Watcher) Updates() <-chan UpdateMsg {
	return w.updates
}

func (w *Watcher) Sessions() []*models.Session {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.sessions
}

func (w *Watcher) poll() {
	sessions, err := w.registry.DetectAll()
	if err == nil {
		sessions = filterSessionsByProject(sessions, w.projectRoot)
	}

	w.mu.Lock()
	if err == nil {
		w.sessions = sessions
	}
	w.mu.Unlock()

	select {
	case w.updates <- UpdateMsg{Sessions: sessions, Err: err}:
	default:
	}
}

func (w *Watcher) Refresh() {
	w.poll()
}

func filterSessionsByProject(sessions []*models.Session, projectRoot string) []*models.Session {
	if projectRoot == "" {
		return sessions
	}

	filtered := make([]*models.Session, 0, len(sessions))
	for _, session := range sessions {
		if sessionMatchesProject(session, projectRoot) {
			filtered = append(filtered, session)
		}
	}
	return filtered
}

func sessionMatchesProject(session *models.Session, projectRoot string) bool {
	if session == nil || session.ProjectPath == "" {
		return false
	}

	projectRoot = filepath.Clean(projectRoot)
	projectPath := filepath.Clean(session.ProjectPath)
	if filepath.IsAbs(projectPath) {
		if rel, err := filepath.Rel(projectRoot, projectPath); err == nil {
			return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
		}
		return projectPath == projectRoot
	}

	rootBase := filepath.Base(projectRoot)
	if strings.EqualFold(projectPath, rootBase) {
		return true
	}

	normalizedProject := normalizeProjectName(projectPath)
	return normalizedProject == normalizeProjectName(rootBase) ||
		strings.Contains(normalizedProject, normalizeProjectName(projectRoot))
}

func normalizeProjectName(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, value)
}
