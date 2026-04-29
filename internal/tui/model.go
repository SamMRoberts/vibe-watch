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
	loader       Loader
	interval     time.Duration
	keys         keyMap
	help         help.Model
	width        int
	height       int
	tab          int
	selected     int
	detailID     string
	detailScroll int
	loaded       bool
	err          error
	snapshot     watcher.Snapshot
}

type keyMap struct {
	Quit    key.Binding
	Refresh key.Binding
	NextTab key.Binding
	PrevTab key.Binding
	Up      key.Binding
	Down    key.Binding
	Select  key.Binding
	Back    key.Binding
	Dash    key.Binding
	List    key.Binding
	Detail  key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Select, k.Back, k.NextTab, k.Refresh, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Select, k.Back},
		{k.Dash, k.List, k.Detail},
		{k.NextTab, k.PrevTab, k.Refresh, k.Quit},
	}
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
			Back:    key.NewBinding(key.WithKeys("esc", "backspace", "b"), key.WithHelp("b/esc", "back")),
			Dash:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "dashboard")),
			List:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "sessions")),
			Detail:  key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "detail")),
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
			if m.tab == tabDetail {
				m.scrollDetail(-1)
			} else {
				m.moveSelection(-1)
			}
			return m, nil
		case key.Matches(msg, m.keys.Down):
			if m.tab == tabDetail {
				m.scrollDetail(1)
			} else {
				m.moveSelection(1)
			}
			return m, nil
		case key.Matches(msg, m.keys.Select):
			if m.tab != tabDetail && len(m.snapshot.Sessions) > 0 {
				m.detailID = m.selectedSessionID()
				m.tab = tabDetail
				m.detailScroll = 0
			}
			return m, nil
		case key.Matches(msg, m.keys.Back):
			if m.tab == tabDetail {
				m.tab = tabSessions
			} else {
				m.tab = tabDashboard
			}
			return m, nil
		case key.Matches(msg, m.keys.Dash):
			m.tab = tabDashboard
			return m, nil
		case key.Matches(msg, m.keys.List):
			m.tab = tabSessions
			return m, nil
		case key.Matches(msg, m.keys.Detail):
			if len(m.snapshot.Sessions) > 0 {
				m.detailID = m.selectedSessionID()
				m.tab = tabDetail
				m.detailScroll = 0
			}
			return m, nil
		}
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.tab == tabDetail {
				m.scrollDetail(-1)
			} else {
				m.moveSelection(-1)
			}
		case tea.MouseButtonWheelDown:
			if m.tab == tabDetail {
				m.scrollDetail(1)
			} else {
				m.moveSelection(1)
			}
		case tea.MouseButtonWheelLeft:
			m.tab = (m.tab + len(tabs) - 1) % len(tabs)
		case tea.MouseButtonWheelRight:
			m.tab = (m.tab + 1) % len(tabs)
		}
		return m, nil
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
	case tabDashboard:
		b.WriteString(renderDashboard(m.snapshot, m.selectedDetail(), width))
	case tabSessions:
		b.WriteString(renderSessionList(m.snapshot.Sessions, m.selected, width))
	case tabDetail:
		b.WriteString(renderDetail(m.selectedDetail(), width, m.detailScroll, m.detailHeight()))
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

func (m *Model) moveSelection(delta int) {
	if len(m.snapshot.Sessions) == 0 {
		return
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.snapshot.Sessions) {
		m.selected = len(m.snapshot.Sessions) - 1
	}
	m.detailID = m.selectedSessionID()
}

func (m *Model) scrollDetail(delta int) {
	m.detailScroll += delta
	if m.detailScroll < 0 {
		m.detailScroll = 0
	}
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

func (m Model) detailHeight() int {
	if m.height <= 0 {
		return 0
	}
	height := m.height - 9
	if height < 8 {
		return 8
	}
	return height
}

func renderHeader(width int) string {
	left := titleStyle.Render(" ◆ vibe-watch ")
	right := monitorStyle.Render(" local Codex session monitor ")
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return left + "\n" + right
	}
	line := left + strings.Repeat(" ", gap) + right
	return headerBarStyle.Width(width).Render(line)
}

func renderMeta(snapshot watcher.Snapshot, width int) string {
	root := trimMiddle(snapshot.Root, max(18, width-24))
	checked := "not checked"
	if !snapshot.CheckedAt.IsZero() {
		checked = snapshot.CheckedAt.Format("15:04:05")
	}
	cells := []string{
		metricBox("sessions", fmt.Sprintf("%d", len(snapshot.Sessions)), cyanTone),
		metricBox("checked", checked, greenTone),
		metricBox("root", root, violetTone),
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
		items = append(items, style.Render(fmt.Sprintf("%d %s", i+1, tab)))
	}
	return lipgloss.NewStyle().Width(width - 2).Render(strings.Join(items, "  "))
}

func renderDashboard(snapshot watcher.Snapshot, detail watcher.SessionDetail, width int) string {
	activeCount := 0
	badCount := 0
	totalEvents := 0
	for _, session := range snapshot.Sessions {
		if session.Status == "active" {
			activeCount++
		}
		if session.Bad > 0 {
			badCount++
		}
		totalEvents += session.Events
	}
	idleCount := len(snapshot.Sessions) - activeCount
	cells := []string{
		miniStat("active", fmt.Sprintf("%d", activeCount), activeStatusStyle),
		miniStat("idle", fmt.Sprintf("%d", idleCount), idleStatusStyle),
		miniStat("events", fmt.Sprintf("%d", totalEvents), accentStyle),
		miniStat("data", qualityLabel(badCount), qualityStyle(badCount)),
	}
	body := strings.Join([]string{
		lipgloss.JoinHorizontal(lipgloss.Top, cells...),
		"",
		barLine("active", activeCount, len(snapshot.Sessions), width-16, activeBarStyle),
		barLine("idle", idleCount, len(snapshot.Sessions), width-16, idleBarStyle),
		barLine("bad lines", badCount, len(snapshot.Sessions), width-16, warningBarStyle),
		"",
		selectedPreview(detail, width-4),
	}, "\n")
	return sectionPanel(width, "Dashboard", body)
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
		cursor = "▶"
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

func renderDetail(detail watcher.SessionDetail, width int, scroll int, height int) string {
	active := renderActive(detail, width)
	content := renderContent(detail.Content, width)
	events := renderEvents(detail.Recent, width)
	body := content + "\n" + active + "\n" + events
	if height <= 0 {
		return body
	}
	return clipLines(body, scroll, height, width)
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

func renderContent(events []watcher.EventSummary, width int) string {
	if len(events) == 0 {
		return sectionPanel(width, "Session content", "No prompt, assistant, reasoning, goal, description, or tool-call content detected.")
	}
	lines := make([]string, 0, len(events)*2)
	for _, event := range events {
		lines = append(lines, renderContentLine(event, width-4))
	}
	return sectionPanel(width, "Session content", strings.Join(lines, "\n"))
}

func renderContentLine(event watcher.EventSummary, width int) string {
	label := event.Kind
	if label == "" {
		label = event.Type
	}
	if event.Actor != "" {
		label = label + " · " + event.Actor
	}
	prefix := contentKindStyle(event.Kind).Render(trimRight(label, max(12, min(24, width/3))))
	meta := subtleStyle.Render(fmt.Sprintf("#%d", event.Line))
	if event.Timestamp != "" {
		meta = subtleStyle.Render(fmt.Sprintf("#%d  %s", event.Line, event.Timestamp))
	}
	textWidth := width - max(lipgloss.Width(prefix), 12) - 4
	if textWidth < 16 {
		textWidth = 16
	}
	text := wrapText(event.Text, textWidth)
	return prefix + "\n" + subtleStyle.Render("  "+meta) + "\n" + indentLines(valueStyle.Render(text), "  ")
}

func selectedPreview(detail watcher.SessionDetail, width int) string {
	if detail.ID == "" {
		return accentStyle.Render("Select a session to open the detail dashboard.")
	}
	state := statusStyle(detail.Status).Render(blankDefault(detail.Status, "unknown"))
	repo := trimMiddle(blankDefault(detail.RepoPath, "-"), max(16, width-22))
	return strings.Join([]string{
		accentStyle.Render("selected session"),
		"  " + valueStyle.Render(trimMiddle(detail.ID, max(16, width-4))),
		"  state " + state + "  events " + valueStyle.Render(fmt.Sprintf("%d", detail.Events)),
		"  repo  " + repo,
	}, "\n")
}

func miniStat(label, value string, style lipgloss.Style) string {
	return miniCardStyle.Render(labelStyle.Render(label) + "\n" + style.Render(value))
}

func barLine(label string, value, total, width int, style lipgloss.Style) string {
	if total <= 0 {
		total = 1
	}
	barWidth := max(8, min(28, width-lipgloss.Width(label)-10))
	filled := value * barWidth / total
	if value > 0 && filled == 0 {
		filled = 1
	}
	bar := style.Render(strings.Repeat("█", filled)) + trackStyle.Render(strings.Repeat("░", barWidth-filled))
	return fmt.Sprintf("%-9s %s %d/%d", label, bar, value, total)
}

func qualityLabel(badCount int) string {
	if badCount == 0 {
		return "clean"
	}
	return fmt.Sprintf("%d flagged", badCount)
}

func qualityStyle(badCount int) lipgloss.Style {
	if badCount == 0 {
		return activeStatusStyle
	}
	return warningStyle
}

func metricBox(label, value string, tone string) string {
	style := cardStyle
	switch tone {
	case cyanTone:
		style = style.BorderForeground(cyanColor)
	case greenTone:
		style = style.BorderForeground(greenColor)
	case violetTone:
		style = style.BorderForeground(violetColor)
	}
	return style.Render(kickerStyle.Render(strings.ToUpper(label)) + "\n" + valueStyle.Render(value))
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
	return panelStyle(width).Render(sectionStyle.Render(" "+title+" ") + "\n" + body)
}

func panelStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width-2).
		Foreground(textColor).
		Background(surfaceDeepColor).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(violetColor).
		Padding(0, 1).
		MarginTop(1)
}

func errorPanel(width int) lipgloss.Style {
	return panelStyle(width).BorderForeground(amberColor)
}

func labelValue(label, value string) string {
	return labelStyle.Render(label+":") + " " + valueStyle.Render(value)
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

func contentKindStyle(kind string) lipgloss.Style {
	switch kind {
	case "user prompt":
		return userContentStyle
	case "assistant":
		return assistantContentStyle
	case "reasoning":
		return reasoningContentStyle
	case "tool call":
		return toolContentStyle
	case "goal", "description":
		return accentStyle
	default:
		return eventTypeStyle
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

func wrapText(value string, width int) string {
	words := strings.Fields(value)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		if lipgloss.Width(current)+1+lipgloss.Width(word) > width {
			lines = append(lines, current)
			current = word
			continue
		}
		current += " " + word
	}
	lines = append(lines, current)
	return strings.Join(lines, "\n")
}

func indentLines(value string, prefix string) string {
	lines := strings.Split(value, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func clipLines(value string, scroll int, height int, width int) string {
	lines := strings.Split(value, "\n")
	if len(lines) <= height {
		return value
	}
	maxScroll := len(lines) - height
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	visible := append([]string{}, lines[scroll:scroll+height]...)
	hint := fmt.Sprintf("showing %d-%d of %d lines • up/down scroll", scroll+1, scroll+height, len(lines))
	visible = append(visible, scrollHintStyle.Width(width-2).Render(hint))
	return strings.Join(visible, "\n")
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
	headerBarStyle        = lipgloss.NewStyle().Background(surfaceDeepColor).Foreground(textColor)
	titleStyle            = lipgloss.NewStyle().Bold(true).Foreground(textColor).Background(cyanDarkColor)
	monitorStyle          = lipgloss.NewStyle().Foreground(warmTextColor).Background(violetDarkColor)
	sectionStyle          = lipgloss.NewStyle().Bold(true).Foreground(textColor).Background(violetDarkColor)
	subtleStyle           = lipgloss.NewStyle().Foreground(mutTextColor)
	kickerStyle           = lipgloss.NewStyle().Foreground(cyanColor).Bold(true)
	labelStyle            = lipgloss.NewStyle().Foreground(cyanSoftColor).Bold(true)
	valueStyle            = lipgloss.NewStyle().Foreground(warmTextColor).Bold(true)
	accentStyle           = lipgloss.NewStyle().Foreground(cyanColor).Bold(true)
	eventTypeStyle        = lipgloss.NewStyle().Foreground(greenColor).Bold(true)
	userContentStyle      = lipgloss.NewStyle().Foreground(textColor).Background(cyanDarkColor).Bold(true).Padding(0, 1)
	assistantContentStyle = lipgloss.NewStyle().Foreground(inkColor).Background(greenColor).Bold(true).Padding(0, 1)
	reasoningContentStyle = lipgloss.NewStyle().Foreground(textColor).Background(violetColor).Bold(true).Padding(0, 1)
	toolContentStyle      = lipgloss.NewStyle().Foreground(inkColor).Background(amberColor).Bold(true).Padding(0, 1)
	cardStyle             = lipgloss.NewStyle().Foreground(textColor).Background(surfaceColor).Border(lipgloss.NormalBorder()).BorderForeground(cyanColor).Padding(0, 1).MarginRight(1)
	miniCardStyle         = lipgloss.NewStyle().Foreground(textColor).Background(surfaceColor).Border(lipgloss.RoundedBorder()).BorderForeground(violetColor).Padding(0, 1).MarginRight(1).Width(14)
	footerStyle           = lipgloss.NewStyle().Foreground(mutTextColor).MarginTop(1)
	tabStyle              = lipgloss.NewStyle().Padding(0, 2).Foreground(mutTextColor).Background(surfaceDeepColor)
	activeTabStyle        = lipgloss.NewStyle().Padding(0, 2).Bold(true).Foreground(textColor).Background(cyanDarkColor)
	sessionRowStyle       = lipgloss.NewStyle().Foreground(textColor)
	selectedRowStyle      = lipgloss.NewStyle().Foreground(textColor).Background(violetDarkColor).Bold(true)
	activeStatusStyle     = lipgloss.NewStyle().Foreground(inkColor).Background(greenColor).Bold(true).Padding(0, 1)
	idleStatusStyle       = lipgloss.NewStyle().Foreground(warmTextColor).Background(surfaceLiftColor).Bold(true).Padding(0, 1)
	warningStyle          = lipgloss.NewStyle().Foreground(inkColor).Background(amberColor).Bold(true).Padding(0, 1)
	activeBarStyle        = lipgloss.NewStyle().Foreground(greenColor)
	idleBarStyle          = lipgloss.NewStyle().Foreground(violetColor)
	warningBarStyle       = lipgloss.NewStyle().Foreground(amberColor)
	trackStyle            = lipgloss.NewStyle().Foreground(trackColor)
	scrollHintStyle       = lipgloss.NewStyle().Foreground(warmTextColor).Background(surfaceLiftColor).Padding(0, 1)
)

const (
	cyanTone   = "cyan"
	greenTone  = "green"
	violetTone = "violet"
)

var (
	inkColor         = lipgloss.Color("16")
	textColor        = lipgloss.Color("255")
	warmTextColor    = lipgloss.Color("230")
	mutTextColor     = lipgloss.Color("250")
	surfaceDeepColor = lipgloss.Color("17")
	surfaceColor     = lipgloss.Color("236")
	surfaceLiftColor = lipgloss.Color("238")
	cyanColor        = lipgloss.Color("51")
	cyanSoftColor    = lipgloss.Color("159")
	cyanDarkColor    = lipgloss.Color("24")
	greenColor       = lipgloss.Color("120")
	amberColor       = lipgloss.Color("222")
	violetColor      = lipgloss.Color("141")
	violetDarkColor  = lipgloss.Color("60")
	trackColor       = lipgloss.Color("242")
)

const (
	tabDashboard = iota
	tabSessions
	tabDetail
)

var tabs = []string{"Dashboard", "Sessions", "Detail"}
