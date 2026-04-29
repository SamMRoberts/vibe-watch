package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/watcher"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Loader func() (watcher.Snapshot, error)

type Options struct {
	Loader   Loader
	Interval time.Duration
}

type Model struct {
	loader   Loader
	interval time.Duration
	keys     keyMap
	help     help.Model
	width    int
	height   int
	loaded   bool
	err      error
	snapshot watcher.Snapshot
}

type keyMap struct {
	Quit    key.Binding
	Refresh key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Refresh, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Refresh, k.Quit}}
}

type snapshotMsg struct {
	snapshot watcher.Snapshot
}

type errMsg struct {
	err error
}

type tickMsg time.Time

func NewModel(opts Options) Model {
	if opts.Interval <= 0 {
		opts.Interval = 2 * time.Second
	}
	return Model{
		loader:   opts.Loader,
		interval: opts.Interval,
		keys: keyMap{
			Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
			Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		},
		help: help.New(),
	}
}

func RenderSnapshot(snapshot watcher.Snapshot) string {
	model := NewModel(Options{})
	model.snapshot = snapshot
	model.loaded = true
	return model.View()
}

func (m Model) Init() tea.Cmd {
	return m.load()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Refresh):
			return m, m.load()
		}
	case snapshotMsg:
		m.snapshot = msg.snapshot
		m.loaded = true
		m.err = nil
		return m, m.tick()
	case errMsg:
		m.err = msg.err
		m.loaded = true
		return m, m.tick()
	case tickMsg:
		return m, m.load()
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("vibe-watch"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render("real-time Codex JSONL monitor"))
	b.WriteString("\n\n")
	if m.err != nil {
		b.WriteString(errorStyle.Render("watch error: " + m.err.Error()))
		b.WriteString("\n\n")
	}
	if !m.loaded {
		b.WriteString("Loading session data...\n\n")
		b.WriteString(m.help.View(m.keys))
		return b.String()
	}

	b.WriteString(fmt.Sprintf("Root: %s\n", m.snapshot.Root))
	b.WriteString(fmt.Sprintf("Checked: %s\n", m.snapshot.CheckedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("Sessions: %d\n", len(m.snapshot.Sessions)))
	if m.snapshot.Active == nil {
		b.WriteString("\nNo Codex sessions found for this root.\n\n")
		b.WriteString(m.help.View(m.keys))
		return b.String()
	}

	active := m.snapshot.Active
	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Active session"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%s  events=%d bad=%d size=%d bytes\n", active.ID, active.Events, active.Bad, active.Size))
	b.WriteString(fmt.Sprintf("updated %s\n", active.ModTime.Format(time.RFC3339)))
	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Recent events"))
	b.WriteString("\n")
	for _, event := range active.Recent {
		b.WriteString(formatEvent(event))
		b.WriteString("\n")
	}
	if len(active.Recent) == 0 {
		b.WriteString("No parsed events yet.\n")
	}
	b.WriteString("\n")
	b.WriteString(m.help.View(m.keys))
	return b.String()
}

func (m Model) load() tea.Cmd {
	return func() tea.Msg {
		if m.loader == nil {
			return errMsg{err: fmt.Errorf("watcher loader is not configured")}
		}
		snapshot, err := m.loader()
		if err != nil {
			return errMsg{err: err}
		}
		return snapshotMsg{snapshot: snapshot}
	}
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(m.interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func formatEvent(event watcher.EventSummary) string {
	parts := []string{fmt.Sprintf("#%d", event.Line), event.Type}
	if event.Timestamp != "" {
		parts = append(parts, event.Timestamp)
	}
	if event.Tool != "" {
		parts = append(parts, "tool="+event.Tool)
	}
	if event.Model != "" {
		parts = append(parts, "model="+event.Model)
	}
	if event.Repo != "" {
		parts = append(parts, "repo="+event.Repo)
	}
	return "  " + strings.Join(parts, "  ")
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	sectionStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	subtleStyle  = lipgloss.NewStyle().Faint(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)
