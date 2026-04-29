package watcher

import (
	"sync"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/agents"
	"github.com/SamMRoberts/vibe-watch/internal/models"
)

type UpdateMsg struct {
	Sessions    []*models.Session
	Err         error
	HasSessions bool
	Refreshing  bool
	Done        bool
}

type Watcher struct {
	registry       *agents.Registry
	staticSessions []*models.Session // non-nil means demo/static mode — no polling
	interval       time.Duration
	mu             sync.RWMutex
	pollMu         sync.Mutex
	sessions       []*models.Session
	updates        chan UpdateMsg
	quit           chan struct{}
	stopOnce       sync.Once
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

// NewStatic returns a Watcher that immediately publishes a fixed set of
// sessions and never re-polls from disk. Intended for demo and screenshot use.
func NewStatic(sessions []*models.Session) *Watcher {
	return &Watcher{
		staticSessions: sessions,
		interval:       365 * 24 * time.Hour, // never fires
		updates:        make(chan UpdateMsg, 10),
		quit:           make(chan struct{}),
	}
}

func (w *Watcher) Start() {
	go func() {
		if w.staticSessions != nil {
			w.publish(w.staticSessions, nil, false, true)
			<-w.quit
			return
		}

		// Normal polling
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
	if w.registry == nil {
		return
	}
	w.pollMu.Lock()
	defer w.pollMu.Unlock()

	w.publishStatus(true)
	sessions, err := w.registry.DetectAllIncremental(func(partial []*models.Session) {
		w.publish(partial, nil, true, false)
	})
	w.publish(sessions, err, false, true)
}

func (w *Watcher) publish(sessions []*models.Session, err error, refreshing, done bool) {
	copied := append([]*models.Session(nil), sessions...)
	w.mu.Lock()
	if err == nil {
		copied = models.MergeSessionUpdates(w.sessions, copied, done)
		w.sessions = copied
	}
	w.mu.Unlock()
	select {
	case w.updates <- UpdateMsg{Sessions: copied, Err: err, HasSessions: true, Refreshing: refreshing, Done: done}:
	default:
	}
}

func (w *Watcher) publishStatus(refreshing bool) {
	select {
	case w.updates <- UpdateMsg{Refreshing: refreshing}:
	default:
	}
}

func (w *Watcher) Refresh() {
	go w.poll()
}
