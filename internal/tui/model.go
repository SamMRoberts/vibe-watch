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
	tab      int
	selected int
	detailID string
	loaded   bool
	err      error
	snapshot watcher.Snapshot
}

type keyMap struct {
	Quit    key.Binding
	Refresh key.Binding
	NextTab key.Binding
	PrevTab key.Binding
	Up      key.Binding
	Down    key.Binding
	Select  key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Select, k.NextTab, k.Refresh, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Select}, {k.NextTab, k.PrevTab, k.Refresh, k.Quit}}
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
			NextTab: key.NewBinding(key.WithKeys("tab", "right"), key.WithHelp("tab/right", "next tab")),
			PrevTab: key.NewBinding(key.WithKeys("shift+tab", "left"), key.WithHelp("left", "previous tab")),
			Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", "up")),
			Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", "down")),
			Select:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
			Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		},
		help: help.New(),
	}
}

func RenderSnapshot(snapshot watcher.Snapshot) string {
	model := NewModel(Options{})
	model.snapshot = snapshot
	model.loaded = true
	model.syncSelection()
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
		case key.Matches(msg, m.keys.NextTab):
			m.tab = (m.tab + 1) % len(tabs)
			return m, nil
		case key.Matches(msg, m.keys.PrevTab):
			m.tab = (m.tab + len(tabs) - 1) % len(tabs)
			return m, nil
		case key.Matches(msg, m.keys.Up):
			if m.tab == tabSessions && m.selected > 0 {
				m.selected--
				m.detailID = m.selectedSessionID()
			}
			return m, nil
		case key.Matches(msg, m.keys.Down):
			if m.tab == tabSessions && m.selected < len(m.snapshot.Sessions)-1 {
				m.selected++
				m.detailID = m.selectedSessionID()
			}
			return m, nil
		case key.Matches(msg, m.keys.Select):
			if m.tab == tabSessions && len(m.snapshot.Sessions) > 0 {
				m.detailID = m.selectedSessionID()
				m.tab = tabDetail
			}
			return m, nil
		}
	case snapshotMsg:
		m.snapshot = msg.snapshot
		m.loaded = true
		m.err = nil
		m.syncSelection()
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
	b.WriteString(renderTabs(m.tab, width))
	b.WriteString("\n")
	if len(m.snapshot.Sessions) == 0 {
		b.WriteString(panelStyle(width).Render("No Codex sessions found for this root."))
		b.WriteString("\n")
		b.WriteString(footerStyle.Render(m.help.View(m.keys)))
		return b.String()
	}

	switch m.tab {
	case tabSessions:
		b.WriteString(renderSessionList(m.snapshot.Sessions, m.selected, width))
	case tabDetail:
		b.WriteString(renderDetail(m.selectedDetail(), width))
	}
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

func (m *Model) syncSelection() {
	if len(m.snapshot.Sessions) == 0 {
		m.selected = 0
		m.detailID = ""
		return
	}
	if m.detailID != "" {
		for i, session := range m.snapshot.Sessions {
			if session.ID == m.detailID {
				m.selected = i
				return
			}
		}
	}
	if m.snapshot.Active != nil {
		for i, session := range m.snapshot.Sessions {
			if session.ID == m.snapshot.Active.ID {
				m.selected = i
				m.detailID = session.ID
				return
			}
		}
	}
	if m.selected >= len(m.snapshot.Sessions) {
		m.selected = len(m.snapshot.Sessions) - 1
	}
	m.detailID = m.selectedSessionID()
}

func (m Model) selectedSessionID() string {
	if m.selected < 0 || m.selected >= len(m.snapshot.Sessions) {
		return ""
	}
	return m.snapshot.Sessions[m.selected].ID
}

func (m Model) selectedDetail() watcher.SessionDetail {
	if m.detailID != "" {
		if detail, ok := m.snapshot.Details[m.detailID]; ok {
			return detail
		}
		if m.snapshot.Active != nil && m.snapshot.Active.ID == m.detailID {
			return *m.snapshot.Active
		}
		for _, session := range m.snapshot.Sessions {
			if session.ID == m.detailID {
				return watcher.SessionDetail{SessionSummary: session}
			}
		}
	}
	if m.snapshot.Active != nil {
		return *m.snapshot.Active
	}
	if len(m.snapshot.Sessions) == 0 {
		return watcher.SessionDetail{}
	}
	return watcher.SessionDetail{SessionSummary: m.snapshot.Sessions[m.selected]}
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

func renderTabs(active int, width int) string {
	items := make([]string, 0, len(tabs))
	for i, tab := range tabs {
		style := tabStyle
		if i == active {
			style = activeTabStyle
		}
		items = append(items, style.Render(tab))
	}
	return lipgloss.NewStyle().Width(width - 2).Render(strings.Join(items, " "))
}

func renderSessionList(sessions []watcher.SessionSummary, selected int, width int) string {
	if len(sessions) == 0 {
		return sectionPanel(width, "Sessions", "No sessions detected.")
	}
	lines := make([]string, 0, len(sessions)+1)
	lines = append(lines, sessionHeader(width-4))
	for i, session := range sessions {
		lines = append(lines, renderSessionRow(session, i == selected, width-4))
	}
	return sectionPanel(width, "Sessions", strings.Join(lines, "\n"))
}

func renderSessionRow(session watcher.SessionSummary, selected bool, width int) string {
	cursor := " "
	style := sessionRowStyle
	if selected {
		cursor = ">"
		style = selectedRowStyle
	}
	status := statusStyle(session.Status).Render(padRight(session.Status, 6))
	agent := trimRight(blankDefault(session.Agent, "unknown"), 8)
	repo := trimMiddle(blankDefault(session.RepoPath, "-"), max(12, width-58))
	model := trimRight(blankDefault(session.Model, "-"), 12)
	updated := "unknown"
	if !session.ModTime.IsZero() {
		updated = session.ModTime.Format("15:04:05")
	}
	idWidth := max(12, min(22, width-72))
	id := trimMiddle(blankDefault(session.ID, "-"), idWidth)
	row := fmt.Sprintf("%s %-*s  %s  %-8s  %-12s  %-6d  %-8s  %s", cursor, idWidth, id, status, agent, model, session.Events, updated, repo)
	return style.Render(trimRight(row, width))
}

func sessionHeader(width int) string {
	idWidth := max(12, min(22, width-72))
	header := fmt.Sprintf("  %-*s  state   agent     model         events  updated   repo path", idWidth, "session")
	return subtleStyle.Render(trimRight(header, width))
}

func renderDetail(detail watcher.SessionDetail, width int) string {
	active := renderActive(detail, width)
	events := renderEvents(detail.Recent, width)
	return active + "\n" + events
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
		labelValue("state", blankDefault(active.Status, "unknown")),
		labelValue("agent", blankDefault(active.Agent, "unknown")),
		labelValue("repo path", trimMiddle(blankDefault(active.RepoPath, "-"), width-18)),
		labelValue("model", blankDefault(active.Model, "-")),
		labelValue("last event", blankDefault(active.LastEventType, "-")),
		labelValue("events", fmt.Sprintf("%d", active.Events)),
		labelValue("quality", status),
		labelValue("size", byteCount(active.Size)),
		labelValue("updated", updated),
	}, "\n")
	return sectionPanel(width, "Session detail", body)
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

func statusStyle(status string) lipgloss.Style {
	switch status {
	case "active":
		return activeStatusStyle
	case "idle":
		return idleStatusStyle
	default:
		return labelStyle
	}
}

func blankDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func padRight(value string, width int) string {
	if lipgloss.Width(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-lipgloss.Width(value))
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var (
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	sectionStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	subtleStyle       = lipgloss.NewStyle().Faint(true)
	labelStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	valueStyle        = lipgloss.NewStyle().Bold(true)
	eventTypeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	cardStyle         = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("8")).Padding(0, 1).MarginRight(1)
	footerStyle       = lipgloss.NewStyle().Faint(true).MarginTop(1)
	tabStyle          = lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("8"))
	activeTabStyle    = lipgloss.NewStyle().Padding(0, 2).Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("12"))
	sessionRowStyle   = lipgloss.NewStyle()
	selectedRowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("10"))
	activeStatusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	idleStatusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

const (
	tabSessions = iota
	tabDetail
)

var tabs = []string{"Sessions", "Detail"}
