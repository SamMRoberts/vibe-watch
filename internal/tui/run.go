package tui

import (
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/watcher"
	tea "github.com/charmbracelet/bubbletea"
)

type RunOptions struct {
	SessionRoot string
	Limit       int
	EventLimit  int
	Interval    time.Duration
}

func Run(opts RunOptions) error {
	model := NewModel(Options{
		Interval: opts.Interval,
		Loader: func() (watcher.Snapshot, error) {
			return watcher.Capture(watcher.Options{
				Root:       opts.SessionRoot,
				Limit:      opts.Limit,
				EventLimit: opts.EventLimit,
			})
		},
	})
	_, err := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}
