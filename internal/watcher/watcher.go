package watcher

import (
	"sync"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/agents"
	"github.com/SamMRoberts/vibe-watch/internal/models"
)

type UpdateMsg struct {
	Sessions []*models.Session
}

type Watcher struct {
	registry *agents.Registry
	interval time.Duration
	mu       sync.RWMutex
	sessions []*models.Session
	updates  chan UpdateMsg
	quit     chan struct{}
	stopOnce sync.Once
}

func New(registry *agents.Registry, interval time.Duration) *Watcher {
	if interval < time.Second {
		interval = 2 * time.Second
	}
	return &Watcher{
		registry: registry,
		interval: interval,
		updates:  make(chan UpdateMsg, 10),
		quit:     make(chan struct{}),
	}
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
	if err != nil {
		return
	}

	w.mu.Lock()
	w.sessions = sessions
	w.mu.Unlock()

	select {
	case w.updates <- UpdateMsg{Sessions: sessions}:
	default:
	}
}

func (w *Watcher) Refresh() {
	w.poll()
}
