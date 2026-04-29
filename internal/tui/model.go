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
	width := m.contentWidth()
	var b strings.Builder
	b.WriteString(renderHeader(width))
	b.WriteString("\n")
	if m.err != nil {
		b.WriteString(errorPanel(width).Render("watch error: " + m.err.Error()))
		b.WriteString("\n")
	}
	if !m.loaded {
		b.WriteString(panelStyle(width).Render("Loading session data..."))
		b.WriteString("\n")
		b.WriteString(footerStyle.Render(m.help.View(m.keys)))
		return b.String()
	}

	b.WriteString(renderMeta(m.snapshot, width))
	b.WriteString("\n")
	if m.snapshot.Active == nil {
		b.WriteString(panelStyle(width).Render("No Codex sessions found for this root."))
		b.WriteString("\n")
		b.WriteString(footerStyle.Render(m.help.View(m.keys)))
		return b.String()
	}

	active := m.snapshot.Active
	b.WriteString(renderActive(*active, width))
	b.WriteString("\n")
	b.WriteString(renderEvents(active.Recent, width))
	b.WriteString("\n")
	b.WriteString(footerStyle.Render(m.help.View(m.keys)))
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

func (m Model) contentWidth() int {
	if m.width <= 0 {
		return 96
	}
	if m.width < 48 {
		return 48
	}
	return m.width
}

func renderHeader(width int) string {
	left := titleStyle.Render("vibe-watch")
	right := subtleStyle.Render("real-time Codex JSONL monitor")
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return left + "\n" + right
	}
	return left + strings.Repeat(" ", gap) + right
}

func renderMeta(snapshot watcher.Snapshot, width int) string {
	root := trimMiddle(snapshot.Root, max(18, width-24))
	checked := "not checked"
	if !snapshot.CheckedAt.IsZero() {
		checked = snapshot.CheckedAt.Format("15:04:05")
	}
	cells := []string{
		metricBox("sessions", fmt.Sprintf("%d", len(snapshot.Sessions))),
		metricBox("checked", checked),
		metricBox("root", root),
	}
	return joinCards(cells, width)
}

func renderActive(active watcher.SessionDetail, width int) string {
	status := "clean"
	if active.Bad > 0 {
		status = fmt.Sprintf("%d bad lines", active.Bad)
	}
	updated := "unknown"
	if !active.ModTime.IsZero() {
		updated = active.ModTime.Format("15:04:05")
	}
	body := strings.Join([]string{
		labelValue("session", trimMiddle(active.ID, width-18)),
		labelValue("events", fmt.Sprintf("%d", active.Events)),
		labelValue("quality", status),
		labelValue("size", byteCount(active.Size)),
		labelValue("updated", updated),
	}, "\n")
	return sectionPanel(width, "Active session", body)
}

func renderEvents(events []watcher.EventSummary, width int) string {
	if len(events) == 0 {
		return sectionPanel(width, "Recent events", "No parsed events yet.")
	}
	lines := make([]string, 0, len(events))
	for _, event := range events {
		lines = append(lines, renderEventLine(event, width-4))
	}
	return sectionPanel(width, "Recent events", strings.Join(lines, "\n"))
}

func renderEventLine(event watcher.EventSummary, width int) string {
	headRaw := trimRight(event.Type, 18)
	meta := []string{fmt.Sprintf("#%d", event.Line)}
	if event.Timestamp != "" {
		meta = append(meta, event.Timestamp)
	}
	if event.Tool != "" {
		meta = append(meta, "tool="+event.Tool)
	}
	if event.Model != "" {
		meta = append(meta, "model="+event.Model)
	}
	if event.Repo != "" {
		meta = append(meta, "repo="+event.Repo)
	}
	metaRaw := strings.Join(meta, "  ")
	metaWidth := width - lipgloss.Width(headRaw) - 2
	if metaWidth < 8 {
		metaWidth = 8
	}
	return eventTypeStyle.Render(headRaw) + "  " + subtleStyle.Render(trimRight(metaRaw, metaWidth))
}

func metricBox(label, value string) string {
	return cardStyle.Render(labelStyle.Render(label) + "\n" + valueStyle.Render(value))
}

func joinCards(cards []string, width int) string {
	if len(cards) == 0 {
		return ""
	}
	cardWidth := 24
	if width >= 90 {
		cardWidth = (width - 4) / 3
	}
	styled := make([]string, 0, len(cards))
	for _, card := range cards {
		styled = append(styled, lipgloss.NewStyle().Width(cardWidth).Render(card))
	}
	if width < 90 {
		return strings.Join(styled, "\n")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, styled...)
}

func sectionPanel(width int, title, body string) string {
	return panelStyle(width).Render(sectionStyle.Render(title) + "\n" + body)
}

func panelStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width-2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1).
		MarginTop(1)
}

func errorPanel(width int) lipgloss.Style {
	return panelStyle(width).BorderForeground(lipgloss.Color("9"))
}

func labelValue(label, value string) string {
	return labelStyle.Render(label+":") + " " + value
}

func byteCount(size int64) string {
	switch {
	case size > 1024*1024:
		return fmt.Sprintf("%.1f MiB", float64(size)/(1024*1024))
	case size > 1024:
		return fmt.Sprintf("%.1f KiB", float64(size)/1024)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func trimMiddle(value string, width int) string {
	if width <= 3 || lipgloss.Width(value) <= width {
		return value
	}
	left := (width - 3) / 2
	right := width - left - 3
	return value[:left] + "..." + value[len(value)-right:]
}

func trimRight(value string, width int) string {
	if width <= 1 || lipgloss.Width(value) <= width {
		return value
	}
	if width == 2 {
		return value[:1] + "."
	}
	return value[:width-3] + "..."
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var (
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	sectionStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	subtleStyle    = lipgloss.NewStyle().Faint(true)
	labelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	valueStyle     = lipgloss.NewStyle().Bold(true)
	eventTypeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	cardStyle      = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("8")).Padding(0, 1).MarginRight(1)
	footerStyle    = lipgloss.NewStyle().Faint(true).MarginTop(1)
)
