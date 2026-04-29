package tui

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/SamMRoberts/vibe-watch/internal/sessions"
	"github.com/SamMRoberts/vibe-watch/internal/theme"
	"github.com/SamMRoberts/vibe-watch/internal/watch"
)

const (
	tabDashboard = iota
	tabSessions
)

type Options struct {
	SessionsRoot string
	Days         int
	Limit        int
	Refresh      time.Duration
	Theme        string
	BatchSize    int
}

func Run(ctx context.Context, opts Options) error {
	if os.Getenv("DEBUG") != "" {
		logFile, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			return err
		}
		defer logFile.Close()
	}
	_, err := tea.NewProgram(NewModel(opts), tea.WithContext(ctx)).Run()
	return err
}

type Model struct {
	opts Options

	width  int
	height int

	activeTab      int
	detailMode     bool
	dashboardFocus int
	themeName      string
	theme          theme.Theme
	keys           keyMap
	help           help.Model

	sessions map[string]sessions.Summary
	order    []string
	offset   int
	total    int
	loading  bool
	lastLoad time.Time
	err      error

	dashboard []viewport.Model
	list      list.Model
	detail    viewport.Model
}

func NewModel(opts Options) Model {
	if opts.Refresh <= 0 {
		opts.Refresh = 2 * time.Second
	}
	if opts.Days == 0 {
		opts.Days = 7
	}
	if opts.Limit == 0 {
		opts.Limit = 200
	}
	if opts.BatchSize == 0 {
		opts.BatchSize = watch.DefaultBatchSize
	}
	if opts.Theme == "" {
		opts.Theme = "auto"
	}
	th := theme.Resolve(opts.Theme)
	delegate := list.NewDefaultDelegate()
	delegate.Styles = list.NewDefaultItemStyles(th.IsDark)
	delegate.SetSpacing(0)
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Sessions"
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	detail := viewport.New()
	detail.MouseWheelEnabled = true
	detail.MouseWheelDelta = 3
	dashboard := make([]viewport.Model, 5)
	for i := range dashboard {
		dashboard[i] = viewport.New()
		dashboard[i].MouseWheelEnabled = true
		dashboard[i].MouseWheelDelta = 3
	}
	return Model{
		opts:      opts,
		themeName: opts.Theme,
		theme:     th,
		keys:      newKeyMap(),
		help:      help.New(),
		sessions:  map[string]sessions.Summary{},
		loading:   true,
		dashboard: dashboard,
		list:      l,
		detail:    detail,
	}
}

func (m Model) Init() tea.Cmd {
	m.loading = true
	return loadBatchCmd(m.opts, 0)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.nextTab):
			if m.detailMode {
				m.detailMode = false
			}
			m.activeTab = (m.activeTab + 1) % 2
		case key.Matches(msg, m.keys.prevTab):
			if m.detailMode {
				m.detailMode = false
			}
			m.activeTab = (m.activeTab + 1) % 2
		case key.Matches(msg, m.keys.refresh):
			m.offset = 0
			m.total = 0
			m.loading = true
			m.err = nil
			m.sessions = map[string]sessions.Summary{}
			m.order = nil
			m.updateListItems()
			m.refreshDashboardContent()
			return m, loadBatchCmd(m.opts, 0)
		case key.Matches(msg, m.keys.theme):
			m.themeName = theme.Next(m.themeName)
			m.theme = theme.Resolve(m.themeName)
			m.refreshDashboardContent()
			m.refreshDetailContent()
		case key.Matches(msg, m.keys.back):
			if m.detailMode {
				m.detailMode = false
				m.activeTab = tabSessions
			}
		case key.Matches(msg, m.keys.open):
			if m.activeTab == tabSessions && !m.detailMode {
				if item, ok := m.list.SelectedItem().(sessionItem); ok && !item.header {
					m.detailMode = true
					m.setDetail(item.id)
				}
			}
		case key.Matches(msg, m.keys.nextPanel):
			if m.activeTab == tabDashboard && !m.detailMode {
				m.dashboardFocus = (m.dashboardFocus + 1) % len(m.dashboard)
			}
		case key.Matches(msg, m.keys.prevPanel):
			if m.activeTab == tabDashboard && !m.detailMode {
				m.dashboardFocus = (m.dashboardFocus + len(m.dashboard) - 1) % len(m.dashboard)
			}
		}
	case tea.MouseWheelMsg:
		if m.detailMode {
			var cmd tea.Cmd
			m.detail, cmd = m.detail.Update(msg)
			return m, cmd
		}
		if m.activeTab == tabDashboard {
			var cmd tea.Cmd
			m.dashboard[m.dashboardFocus], cmd = m.dashboard[m.dashboardFocus].Update(msg)
			return m, cmd
		}
	case batchLoadedMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.total = msg.batch.Total
			m.lastLoad = time.Now()
			for _, summary := range msg.batch.Sessions {
				m.sessions[summary.ID] = summary
			}
			m.rebuildOrder()
			m.updateListItems()
			m.refreshDashboardContent()
			if m.detailMode {
				m.refreshDetailContent()
			}
			if !msg.batch.Done {
				m.loading = true
				m.offset = msg.batch.Offset + len(msg.batch.Sessions)
				cmds = append(cmds, loadBatchCmd(m.opts, m.offset))
			} else {
				m.offset = 0
				cmds = append(cmds, tickCmd(m.opts.Refresh))
			}
		} else {
			cmds = append(cmds, tickCmd(m.opts.Refresh))
		}
	case tickMsg:
		m.loading = true
		m.offset = 0
		cmds = append(cmds, loadBatchCmd(m.opts, 0))
	}

	if m.detailMode {
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.activeTab == tabSessions {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.activeTab == tabDashboard && len(m.dashboard) > 0 {
		var cmd tea.Cmd
		m.dashboard[m.dashboardFocus], cmd = m.dashboard[m.dashboardFocus].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) View() tea.View {
	body := m.render()
	v := tea.NewView(body)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "vibe-watch"
	return v
}

func (m *Model) resize() {
	bodyHeight := max(5, m.height-4)
	m.help.SetWidth(m.width)
	m.list.SetSize(max(20, m.width), bodyHeight)
	m.detail.SetWidth(max(20, m.width-2))
	m.detail.SetHeight(bodyHeight)
	for i := range m.dashboard {
		m.dashboard[i].SetWidth(max(16, (m.width/2)-4))
		m.dashboard[i].SetHeight(max(3, (bodyHeight/3)-2))
	}
	m.refreshDashboardContent()
	m.refreshDetailContent()
}

func (m *Model) rebuildOrder() {
	m.order = m.order[:0]
	for id := range m.sessions {
		m.order = append(m.order, id)
	}
	sort.SliceStable(m.order, func(i, j int) bool {
		a := m.sessions[m.order[i]]
		b := m.sessions[m.order[j]]
		if !a.StartedAt.Equal(b.StartedAt) {
			return a.StartedAt.After(b.StartedAt)
		}
		return a.ID > b.ID
	})
}

func (m *Model) updateListItems() {
	selectedID := ""
	if item, ok := m.list.SelectedItem().(sessionItem); ok {
		selectedID = item.id
	}
	items := make([]list.Item, 0, len(m.order)*2)
	seenGroup := map[string]bool{}
	selectedIndex := 0
	foundSelected := false
	for _, id := range m.order {
		summary := m.sessions[id]
		group := groupKey(summary)
		if !seenGroup[group] {
			seenGroup[group] = true
			items = append(items, sessionItem{header: true, title: group})
		}
		item := sessionItem{id: id, title: listTitle(summary), desc: listDescription(summary)}
		if id == selectedID || (!foundSelected && selectedID == "") {
			selectedIndex = len(items)
			foundSelected = true
		}
		items = append(items, item)
	}
	m.list.SetItems(items)
	if selectedIndex < len(items) {
		m.list.Select(selectedIndex)
	}
}

func (m *Model) refreshDashboardContent() {
	if len(m.dashboard) == 0 {
		return
	}
	m.dashboard[0].SetContent(m.renderCounts())
	m.dashboard[1].SetContent(m.renderStatuses())
	m.dashboard[2].SetContent(m.renderAgents())
	m.dashboard[3].SetContent(m.renderTokens())
	m.dashboard[4].SetContent(m.renderRecent())
}

func (m *Model) setDetail(id string) {
	summary, ok := m.sessions[id]
	if !ok {
		return
	}
	m.detail.SetContent(m.renderDetail(summary))
}

func (m *Model) refreshDetailContent() {
	if !m.detailMode {
		return
	}
	if item, ok := m.list.SelectedItem().(sessionItem); ok && !item.header {
		m.setDetail(item.id)
	}
}

func (m Model) render() string {
	header := m.renderHeader()
	var body string
	if m.detailMode {
		body = m.detail.View()
	} else if m.activeTab == tabSessions {
		body = m.list.View()
	} else {
		body = m.renderDashboard()
	}
	help := m.theme.Help.Render(m.help.ShortHelpView(m.keys.ShortHelp()))
	return lipgloss.JoinVertical(lipgloss.Left, header, body, help)
}

func (m Model) renderHeader() string {
	status := "idle"
	if m.loading {
		status = fmt.Sprintf("loading %d/%d", len(m.sessions), m.total)
	}
	if m.err != nil {
		status = "error: " + m.err.Error()
	}
	tabs := []string{
		m.renderTab("Dashboard", m.activeTab == tabDashboard && !m.detailMode),
		m.renderTab("Sessions", m.activeTab == tabSessions && !m.detailMode),
	}
	if m.detailMode {
		tabs = append(tabs, m.renderTab("Detail", true))
	}
	right := m.theme.Muted.Render(fmt.Sprintf("%s | theme=%s | %s", status, m.themeName, m.opts.SessionsRoot))
	return m.theme.Header.Width(max(20, m.width)).Render("vibe-watch  " + strings.Join(tabs, " ") + "  " + right)
}

func (m Model) renderTab(name string, active bool) string {
	if active {
		return m.theme.ActiveTab.Render(name)
	}
	return m.theme.Tab.Render(name)
}

func (m Model) renderDashboard() string {
	left := lipgloss.JoinVertical(lipgloss.Left, panel(m.theme, "Session counts", m.dashboard[0].View(), m.dashboardFocus == 0), panel(m.theme, "Agents", m.dashboard[2].View(), m.dashboardFocus == 2), panel(m.theme, "Recent", m.dashboard[4].View(), m.dashboardFocus == 4))
	right := lipgloss.JoinVertical(lipgloss.Left, panel(m.theme, "Status", m.dashboard[1].View(), m.dashboardFocus == 1), panel(m.theme, "Tokens", m.dashboard[3].View(), m.dashboardFocus == 3))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) renderCounts() string {
	status := m.statusCounts()
	return fmt.Sprintf("total: %d\nactive: %d\ncompleted: %d\nerrored: %d\nunknown: %d", len(m.sessions), status[sessions.StatusActive], status[sessions.StatusCompleted], status[sessions.StatusErrored], status[sessions.StatusUnknown])
}

func (m Model) renderStatuses() string {
	counts := m.statusCounts()
	lines := make([]string, 0, 4)
	for _, status := range []sessions.Status{sessions.StatusActive, sessions.StatusCompleted, sessions.StatusErrored, sessions.StatusUnknown} {
		lines = append(lines, fmt.Sprintf("%s: %d", status, counts[status]))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderAgents() string {
	counts := map[string]int{}
	for _, summary := range m.sessions {
		counts[summary.Agent]++
	}
	return sortedCounts(counts)
}

func (m Model) renderTokens() string {
	var known, missing, total int
	for _, summary := range m.sessions {
		if summary.Tokens.Known {
			known++
			total += summary.Tokens.Total.TotalTokens
		} else {
			missing++
		}
	}
	return fmt.Sprintf("known sessions: %d\nmissing sessions: %d\ntotal tokens: %d", known, missing, total)
}

func (m Model) renderRecent() string {
	lines := []string{}
	if !m.lastLoad.IsZero() {
		lines = append(lines, "last refresh: "+m.lastLoad.Format("15:04:05"))
	}
	for i, id := range m.order {
		if i >= 6 {
			break
		}
		summary := m.sessions[id]
		lines = append(lines, fmt.Sprintf("%s %s %s", summary.StartedAt.Format("Jan 02 15:04"), summary.Status, summary.PromptPreview))
	}
	if len(lines) == 0 {
		return "No sessions loaded yet."
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderDetail(summary sessions.Summary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", m.theme.Title.Render(summary.ID))
	fmt.Fprintf(&b, "%s | %s | %s | tools=%d | tokens known=%t\n\n", summary.Agent, summary.Status, summary.StartedAt.Format(time.RFC3339), summary.ToolCallCount, summary.Tokens.Known)
	for _, activity := range summary.Activities {
		style := m.theme.Muted
		switch activity.Kind {
		case sessions.ActivityUserPrompt:
			style = m.theme.User
		case sessions.ActivityAssistantMessage:
			style = m.theme.Assistant
		case sessions.ActivityToolCall, sessions.ActivityToolResult:
			style = m.theme.Tool
		case sessions.ActivityDiagnostic:
			style = m.theme.Diagnostic
		}
		fmt.Fprintf(&b, "%s", style.Render(string(activity.Kind)))
		if !activity.Timestamp.IsZero() {
			fmt.Fprintf(&b, " %s", activity.Timestamp.Format("15:04:05"))
		}
		if activity.Title != "" {
			fmt.Fprintf(&b, "  %s", activity.Title)
		}
		if activity.CallID != "" {
			fmt.Fprintf(&b, "  [%s]", activity.CallID)
		}
		fmt.Fprintln(&b)
		if strings.TrimSpace(activity.Body) != "" {
			fmt.Fprintf(&b, "%s\n", indent(activity.Body, "  "))
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}

func (m Model) statusCounts() map[sessions.Status]int {
	counts := map[sessions.Status]int{
		sessions.StatusActive:    0,
		sessions.StatusCompleted: 0,
		sessions.StatusErrored:   0,
		sessions.StatusUnknown:   0,
	}
	for _, summary := range m.sessions {
		counts[summary.Status]++
	}
	return counts
}

type sessionItem struct {
	id     string
	title  string
	desc   string
	header bool
}

func (i sessionItem) FilterValue() string {
	return i.title + " " + i.desc
}

func (i sessionItem) Title() string {
	if i.header {
		return i.title
	}
	return i.title
}

func (i sessionItem) Description() string {
	if i.header {
		return ""
	}
	return i.desc
}

type batchLoadedMsg struct {
	batch watch.Batch
	err   error
}

type tickMsg time.Time

func loadBatchCmd(opts Options, offset int) tea.Cmd {
	return func() tea.Msg {
		batch, err := watch.LoadBatch(context.Background(), watch.Options{
			Root:  opts.SessionsRoot,
			Days:  opts.Days,
			Limit: opts.Limit,
		}, offset, opts.BatchSize)
		return batchLoadedMsg{batch: batch, err: err}
	}
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type keyMap struct {
	quit      key.Binding
	nextTab   key.Binding
	prevTab   key.Binding
	refresh   key.Binding
	open      key.Binding
	back      key.Binding
	theme     key.Binding
	nextPanel key.Binding
	prevPanel key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		nextTab:   key.NewBinding(key.WithKeys("tab", "right"), key.WithHelp("tab", "next tab")),
		prevTab:   key.NewBinding(key.WithKeys("shift+tab", "left"), key.WithHelp("left", "prev tab")),
		refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		open:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		theme:     key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "theme")),
		nextPanel: key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next panel")),
		prevPanel: key.NewBinding(key.WithKeys("["), key.WithHelp("[", "prev panel")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.nextTab, k.open, k.back, k.refresh, k.theme, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.nextTab, k.prevTab, k.open, k.back}, {k.refresh, k.theme, k.nextPanel, k.prevPanel, k.quit}}
}

func groupKey(summary sessions.Summary) string {
	date := "unknown date"
	if !summary.StartedAt.IsZero() {
		date = summary.StartedAt.Format("2006-01-02")
	}
	agent := summary.Agent
	if agent == "" {
		agent = "codex"
	}
	return date + " / " + agent
}

func listTitle(summary sessions.Summary) string {
	start := "unknown"
	if !summary.StartedAt.IsZero() {
		start = summary.StartedAt.Format("15:04")
	}
	return fmt.Sprintf("%s  %s  %s", start, summary.Status, summary.ID)
}

func listDescription(summary sessions.Summary) string {
	tokenText := "tokens: missing"
	if summary.Tokens.Known {
		tokenText = fmt.Sprintf("tokens: %d", summary.Tokens.Total.TotalTokens)
	}
	diag := ""
	if len(summary.Diagnostics) > 0 {
		diag = fmt.Sprintf(" diagnostics=%d", len(summary.Diagnostics))
	}
	return fmt.Sprintf("%s | tools=%d | %s%s", summary.PromptPreview, summary.ToolCallCount, tokenText, diag)
}

func panel(th theme.Theme, title, body string, focused bool) string {
	label := title
	if focused {
		label = "> " + title
	}
	return th.Panel.Render(th.Title.Render(label) + "\n" + body)
}

func sortedCounts(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		if k == "" {
			k = "unknown"
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s: %d", k, counts[k]))
	}
	if len(lines) == 0 {
		return "none"
	}
	return strings.Join(lines, "\n")
}

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
