package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SamMRoberts/vibe-watch/internal/models"
	"github.com/SamMRoberts/vibe-watch/internal/watcher"
)

type viewState int

const (
	viewDashboard viewState = iota
	viewDetail
	viewPromptDetail
	viewAnalytics
)

type tickMsg time.Time

type sessionsUpdateMsg struct {
	sessions    []*models.Session
	err         error
	hasSessions bool
	refreshing  bool
	done        bool
}

type App struct {
	watcher                 *watcher.Watcher
	sessions                []*models.Session
	analyticsSessions       []*models.Session
	analyticsRefreshPending bool
	agentFilter             string
	view                    viewState
	dashboard               *DashboardView
	detail                  *DetailView
	analytics               *AnalyticsView
	width                   int
	height                  int
	lastRefresh             time.Time
	loading                 bool
	refreshing              bool
	filterInput             string
	filterMode              bool
	lastErr                 error
}

func NewApp(w *watcher.Watcher, agentFilter string) *App {
	return &App{
		watcher:     w,
		agentFilter: agentFilter,
		view:        viewDashboard,
		loading:     true,
	}
}

func (a *App) Init() tea.Cmd {
	a.watcher.Start()
	return tea.Batch(
		waitForUpdate(a.watcher),
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func waitForUpdate(w *watcher.Watcher) tea.Cmd {
	return func() tea.Msg {
		update := <-w.Updates()
		return sessionsUpdateMsg{
			sessions:    update.Sessions,
			err:         update.Err,
			hasSessions: update.HasSessions,
			refreshing:  update.Refreshing,
			done:        update.Done,
		}
	}
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.initViews()

	case tickMsg:
		if a.view == viewDetail && a.detail != nil {
			a.detail.AdvanceAnimation()
		}
		cmds = append(cmds, tickCmd())

	case sessionsUpdateMsg:
		a.refreshing = msg.refreshing
		if msg.hasSessions && msg.err == nil {
			a.sessions = msg.sessions
		}
		if msg.hasSessions {
			a.loading = false
			a.lastErr = msg.err
			a.updateAnalyticsSnapshot(msg)
			a.updateViews()
		}
		if msg.done {
			a.lastRefresh = time.Now()
			a.analyticsRefreshPending = false
		}
		cmds = append(cmds, waitForUpdate(a.watcher))

	case tea.MouseMsg:
		a.handleMouse(msg)

	case tea.KeyMsg:
		if a.filterMode {
			return a.handleFilterInput(msg)
		}

		switch {
		case key.Matches(msg, keys.Quit) && a.view != viewDetail && a.view != viewPromptDetail:
			a.watcher.Stop()
			return a, tea.Quit

		case key.Matches(msg, keys.Tab):
			if a.view == viewDashboard {
				a.view = viewAnalytics
			} else {
				a.view = viewDashboard
			}

		case key.Matches(msg, keys.ShiftTab):
			if a.view == viewDashboard {
				a.view = viewAnalytics
			} else {
				a.view = viewDashboard
			}

		case key.Matches(msg, keys.Esc):
			if a.view == viewPromptDetail {
				a.view = viewDetail
				a.detail.ShowSessionDetail()
			} else if a.view == viewDetail {
				a.view = viewDashboard
			}

		case key.Matches(msg, keys.Enter):
			if a.view == viewDashboard && a.dashboard != nil {
				selected := a.dashboard.SelectedSession()
				if selected != nil {
					a.detail.SetSession(selected)
					if selected.IsActive {
						a.detail.FollowLatest()
					}
					a.view = viewDetail
				}
			} else if a.view == viewDetail && a.detail != nil {
				if a.detail.OpenSelectedDetail() {
					a.view = viewPromptDetail
				}
			}

		case key.Matches(msg, keys.Refresh):
			if a.view == viewAnalytics {
				a.analyticsRefreshPending = true
			}
			if a.watcher != nil {
				a.watcher.Refresh()
			}

		case key.Matches(msg, keys.Filter):
			if a.view == viewDashboard {
				a.filterMode = true
				a.filterInput = ""
			}

		case key.Matches(msg, keys.Up):
			if a.view == viewDashboard && a.dashboard != nil {
				a.dashboard.MoveUp()
			} else if a.view == viewDetail && a.detail != nil {
				a.detail.SelectPreviousRow()
			} else if a.view == viewPromptDetail && a.detail != nil {
				a.detail.ScrollUp()
			} else if a.view == viewAnalytics && a.analytics != nil {
				a.analytics.ScrollUp()
			}

		case key.Matches(msg, keys.Down):
			if a.view == viewDashboard && a.dashboard != nil {
				a.dashboard.MoveDown()
			} else if a.view == viewDetail && a.detail != nil {
				a.detail.SelectNextRow()
			} else if a.view == viewPromptDetail && a.detail != nil {
				a.detail.ScrollDown()
			} else if a.view == viewAnalytics && a.analytics != nil {
				a.analytics.ScrollDown()
			}

		case key.Matches(msg, keys.PreviousPrompt):
			if a.view == viewDetail && a.detail != nil {
				a.detail.SelectPreviousUser()
			}

		case key.Matches(msg, keys.NextPrompt):
			if a.view == viewDetail && a.detail != nil {
				a.detail.SelectNextUser()
			}

		case key.Matches(msg, keys.Home):
			if a.view == viewDetail && a.detail != nil {
				a.detail.SelectFirstRow()
			} else if a.view == viewPromptDetail && a.detail != nil {
				a.detail.viewport.GotoTop()
			} else if a.view == viewAnalytics && a.analytics != nil {
				a.analytics.GotoTop()
			}

		case key.Matches(msg, keys.End):
			if a.view == viewDetail && a.detail != nil {
				a.detail.FollowLatest()
			} else if a.view == viewPromptDetail && a.detail != nil {
				a.detail.viewport.GotoBottom()
			} else if a.view == viewAnalytics && a.analytics != nil {
				a.analytics.GotoBottom()
			}

		case key.Matches(msg, keys.Collapse):
			if a.view == viewDetail && a.detail != nil {
				a.detail.ToggleSelectedThread()
			}

		case key.Matches(msg, keys.CollapseAll):
			if a.view == viewDetail && a.detail != nil {
				a.detail.ToggleAllThreadsCollapsed()
			}

		case key.Matches(msg, keys.DetailLevel):
			if a.view == viewDetail && a.detail != nil {
				a.detail.ToggleTimelineDetailLevel()
			}

		case key.Matches(msg, keys.Timestamps):
			if a.view == viewDetail && a.detail != nil {
				a.detail.ToggleTimestamps()
			}

		case key.Matches(msg, keys.Follow):
			if a.view == viewDetail && a.detail != nil {
				a.detail.ToggleFollow()
			}

		case key.Matches(msg, keys.PageUp):
			if (a.view == viewDetail || a.view == viewPromptDetail) && a.detail != nil {
				a.detail.PageUp()
			} else if a.view == viewAnalytics && a.analytics != nil {
				a.analytics.PageUp()
			}

		case key.Matches(msg, keys.PageDown):
			if (a.view == viewDetail || a.view == viewPromptDetail) && a.detail != nil {
				a.detail.PageDown()
			} else if a.view == viewAnalytics && a.analytics != nil {
				a.analytics.PageDown()
			}
		}
	}

	return a, tea.Batch(cmds...)
}

func (a *App) handleMouse(msg tea.MouseMsg) {
	switch {
	case isMouseWheelUp(msg):
		a.scrollActiveViewport(-1)
	case isMouseWheelDown(msg):
		a.scrollActiveViewport(1)
	}
}

func (a *App) scrollActiveViewport(direction int) {
	switch {
	case a.view == viewDetail && a.detail != nil:
		if direction < 0 {
			a.detail.ScrollUp()
		} else {
			a.detail.ScrollDown()
		}
	case a.view == viewPromptDetail && a.detail != nil:
		if direction < 0 {
			a.detail.ScrollUp()
		} else {
			a.detail.ScrollDown()
		}
	case a.view == viewAnalytics && a.analytics != nil:
		if direction < 0 {
			a.analytics.ScrollUp()
		} else {
			a.analytics.ScrollDown()
		}
	}
}

func isMouseWheelUp(msg tea.MouseMsg) bool {
	return msg.Button == tea.MouseButtonWheelUp || msg.Type == tea.MouseWheelUp
}

func isMouseWheelDown(msg tea.MouseMsg) bool {
	return msg.Button == tea.MouseButtonWheelDown || msg.Type == tea.MouseWheelDown
}

func (a *App) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc":
		a.filterMode = false
		if a.dashboard != nil {
			a.dashboard.filter = a.filterInput
			a.dashboard.SetSessions(a.sessions, a.agentFilter)
		}
	case "backspace":
		if len(a.filterInput) > 0 {
			a.filterInput = a.filterInput[:len(a.filterInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			a.filterInput += msg.String()
		}
	}
	return a, nil
}

func (a *App) filteredSessions() []*models.Session {
	if a.dashboard == nil {
		return a.sessions
	}
	if a.agentFilter == "" && a.dashboard.filter == "" {
		return a.sessions
	}
	var out []*models.Session
	filterLower := strings.ToLower(a.dashboard.filter)
	for _, s := range a.sessions {
		if a.agentFilter != "" {
			agentLower := strings.ToLower(string(s.AgentType))
			if !strings.Contains(agentLower, strings.ToLower(a.agentFilter)) {
				continue
			}
		}
		if filterLower != "" {
			proj := strings.ToLower(s.ProjectPath)
			agent := strings.ToLower(string(s.AgentType))
			if !strings.Contains(proj, filterLower) && !strings.Contains(agent, filterLower) {
				continue
			}
		}
		out = append(out, s)
	}
	return out
}

func (a *App) initViews() {
	a.dashboard = NewDashboardView(a.width, a.height)
	a.detail = NewDetailView(a.width, a.height)
	a.analytics = NewAnalyticsView(a.width, a.height)
	a.updateViews()
}

func (a *App) updateViews() {
	if a.dashboard != nil {
		a.dashboard.SetSessions(a.sessions, a.agentFilter)
	}
	if a.detail != nil {
		a.refreshDetailSession()
	}
	if a.analytics != nil {
		a.analytics.SetSessions(a.analyticsSessions)
	}
}

func (a *App) updateAnalyticsSnapshot(msg sessionsUpdateMsg) {
	if !msg.done || msg.err != nil {
		return
	}
	if a.analyticsSessions != nil && !a.analyticsRefreshPending {
		return
	}
	a.analyticsSessions = append([]*models.Session{}, msg.sessions...)
	if a.analytics != nil {
		a.analytics.SetSessions(a.analyticsSessions)
	}
}

func (a *App) refreshDetailSession() {
	if a.detail.session == nil {
		return
	}
	if updated := findMatchingSession(a.detail.session, a.sessions); updated != nil {
		shouldFollow := a.detail.Following()
		userPausedFollow := a.detail.UserPausedFollow()
		wasAtBottom := a.detail.AtBottom()
		a.detail.SetSession(updated)
		if a.view == viewPromptDetail {
			a.detail.RefreshFocusedDetail()
		}
		// Only auto-follow if user didn't explicitly pause follow. wasAtBottom alone isn't enough
		// because user may have just paused but viewport still appears at bottom.
		if a.view == viewDetail && updated.IsActive && (shouldFollow || (wasAtBottom && !userPausedFollow)) {
			a.detail.FollowLatest()
		} else if a.view == viewPromptDetail && updated.IsActive && (shouldFollow || (wasAtBottom && !userPausedFollow)) {
			a.detail.ScrollToBottom()
		}
	}
}

func findMatchingSession(target *models.Session, sessions []*models.Session) *models.Session {
	for _, session := range sessions {
		if sameSession(target, session) {
			return session
		}
	}
	return nil
}

func sameSession(a, b *models.Session) bool {
	if a == nil || b == nil {
		return false
	}
	if a.ID != "" && b.ID == a.ID {
		return a.AgentType == "" || b.AgentType == a.AgentType
	}
	return a.LogPath != "" && b.LogPath == a.LogPath
}

func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	header := a.renderShellHeader()
	footer := a.renderShellFooter()
	contentHeight := a.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if contentHeight < 1 {
		contentHeight = 1
	}
	content := fitRenderedHeight(a.renderContent(), contentHeight)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (a *App) renderContent() string {
	var sb strings.Builder
	switch a.view {
	case viewDashboard:
		if a.dashboard == nil {
			sb.WriteString(styleMuted.Render("Initializing..."))
		} else {
			if a.loading {
				sb.WriteString(observatoryPanel(a.width-6, styleAccent.Render("✦ scanning session telemetry...")))
			} else {
				a.dashboard.filterMode = a.filterMode
				a.dashboard.filterInput = a.filterInput
				sb.WriteString(a.dashboard.View(a.agentFilter))
			}
		}
	case viewDetail:
		if a.detail != nil {
			sb.WriteString(a.detail.View())
		}
	case viewPromptDetail:
		if a.detail != nil {
			sb.WriteString(a.detail.ThreadView())
		}
	case viewAnalytics:
		if a.analytics != nil {
			sb.WriteString(a.analytics.View())
		}
	}
	return sb.String()
}

func fitRenderedHeight(rendered string, height int) string {
	if height <= 0 {
		return ""
	}
	rendered = strings.TrimRight(rendered, "\n")
	lines := strings.Split(rendered, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (a *App) renderShellHeader() string {
	contentWidth := maxInt(1, a.width-2)
	titleText := "◈ vibe-watch"
	if a.width < 54 {
		titleText = "◈ vw"
	}
	title := styleTitle.Render(titleText)
	subtitleText := ""
	switch {
	case a.width >= 112:
		subtitleText = "agent session observatory"
	case a.width >= 82:
		subtitleText = "observatory"
	}
	headerLeft := title
	if subtitleText != "" {
		headerLeft = lipgloss.JoinHorizontal(lipgloss.Bottom, title, "  ", styleMuted.Render(subtitleText))
	}
	headerTabs := a.renderViewTabs()
	headerRight := a.renderHeaderStatus()

	headerWidth := contentWidth - lipgloss.Width(headerLeft) - lipgloss.Width(headerRight)
	if headerWidth < lipgloss.Width(headerTabs) {
		headerTabs = a.renderCompactViewTab()
		headerWidth = contentWidth - lipgloss.Width(headerLeft) - lipgloss.Width(headerRight)
	}
	if headerWidth < lipgloss.Width(headerTabs) {
		headerRight = ""
		headerWidth = contentWidth - lipgloss.Width(headerLeft)
	}
	if headerWidth < lipgloss.Width(headerTabs) {
		headerLeft = styleTitle.Render("◈ vw")
		headerWidth = contentWidth - lipgloss.Width(headerLeft) - lipgloss.Width(headerRight)
	}
	if headerWidth < lipgloss.Width(headerTabs) {
		headerTabs = ""
		headerWidth = contentWidth - lipgloss.Width(headerLeft)
	}
	if headerWidth < 1 {
		headerWidth = 1
	}
	headerMiddle := lipgloss.NewStyle().Width(headerWidth).Align(lipgloss.Center).Render(headerTabs)

	return lipgloss.NewStyle().
		Background(colorSurface).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(colorPrimary).
		Padding(0, 1).
		Width(contentWidth).
		Render(lipgloss.JoinHorizontal(lipgloss.Bottom, headerLeft, headerMiddle, headerRight))
}

func (a *App) renderViewTabs() string {
	dashboardLabel := "☷ Dashboard"
	analyticsLabel := "▣ Analytics"
	if a.width < 92 {
		dashboardLabel = "☷ Dash"
	}
	if a.width < 86 {
		analyticsLabel = "▣ Stats"
	}
	tabDash := styleTab.Render(dashboardLabel)
	tabAnalytics := styleTab.Render(analyticsLabel)
	switch a.view {
	case viewDashboard:
		tabDash = styleActiveTab.Render(dashboardLabel)
	case viewAnalytics:
		tabAnalytics = styleActiveTab.Render(analyticsLabel)
	case viewDetail:
		tabDash = styleActiveTab.Render("◈ Detail")
	case viewPromptDetail:
		tabDash = styleActiveTab.Render("◉ Focus")
	}
	return lipgloss.JoinHorizontal(lipgloss.Bottom, tabDash, tabAnalytics)
}

func (a *App) renderCompactViewTab() string {
	switch a.view {
	case viewAnalytics:
		return styleActiveTab.Render("▣ Analytics")
	case viewDetail:
		return styleActiveTab.Render("◈ Detail")
	case viewPromptDetail:
		return styleActiveTab.Render("◉ Focus")
	default:
		return styleActiveTab.Render("☷ Dashboard")
	}
}

func (a *App) renderHeaderStatus() string {
	active := 0
	failed := 0
	for _, s := range a.sessions {
		if s.IsActive {
			active++
		}
		if sessionTerminalFailed(s) {
			failed++
		}
	}
	if len(a.sessions) == 0 && a.lastRefresh.IsZero() && !a.refreshing {
		return ""
	}
	if a.width < 112 {
		return ""
	}
	var parts []string
	if a.refreshing {
		parts = append(parts, statusChip(statusRefreshing))
	}
	parts = append(parts, quietPill(fmt.Sprintf("%d sessions", len(a.sessions))))
	if active > 0 {
		parts = append(parts, statusCountChip(statusActive, active))
	}
	if failed > 0 {
		parts = append(parts, statusCountChip(statusFailed, failed))
	}
	if !a.lastRefresh.IsZero() && a.width >= 122 {
		parts = append(parts, styleMuted.Render(fmt.Sprintf("%ds", int(time.Since(a.lastRefresh).Seconds()))))
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, joinWithSpaces(parts)...)
}

func (a *App) renderShellFooter() string {
	innerWidth := maxInt(1, a.width-4)
	helpText := a.renderFooterLine(innerWidth)
	if a.lastErr != nil {
		helpText = styleError.Render(fmt.Sprintf("⚠ detection error: %v", a.lastErr)) +
			styleMuted.Render("  Check detector paths and logs, then press r to retry.")
	}
	return lipgloss.NewStyle().
		Background(colorSurface).
		Foreground(colorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(colorPrimary).
		Padding(0, 1).
		Width(maxInt(1, a.width-2)).
		Render(helpText)
}

func (a *App) renderFooterLine(width int) string {
	status := a.renderFooterStatusText()
	if status == "" {
		return a.renderHelpTextForWidth(width)
	}
	statusMax := width * 3 / 5
	if statusMax < 28 {
		statusMax = minInt(width, 28)
	}
	status = truncateEnd(status, statusMax)
	statusText := styleMuted.Render(status)
	remaining := width - lipgloss.Width(status) - 3
	if remaining < 16 {
		return statusText
	}
	helpText := a.renderHelpTextForWidth(remaining)
	return lipgloss.JoinHorizontal(lipgloss.Center, statusText, styleDivider.Render(" │ "), helpText)
}

func (a *App) renderFooterStatusText() string {
	refreshStatus := ""
	if a.refreshing {
		refreshStatus = "↻ refreshing"
	}
	viewStatus := ""
	switch {
	case a.view == viewDetail && a.detail != nil:
		viewStatus = a.detail.FooterStatus()
	case a.view == viewPromptDetail && a.detail != nil:
		viewStatus = a.detail.FocusedFooterStatus()
	case a.view == viewAnalytics && a.analytics != nil:
		viewStatus = a.analytics.FooterStatus()
	}
	switch {
	case refreshStatus != "" && viewStatus != "":
		return refreshStatus + " · " + viewStatus
	case refreshStatus != "":
		return refreshStatus
	default:
		return viewStatus
	}
}

func (a *App) renderHelpText() string {
	return a.renderHelpTextForWidth(a.width - 4)
}

func (a *App) renderHelpTextForWidth(width int) string {
	h := newHelpModel(width)
	switch {
	case a.filterMode:
		return h.View(filterHelp())
	case a.view == viewDetail:
		return h.View(detailHelp())
	case a.view == viewPromptDetail:
		return h.View(focusedHelp())
	case a.view == viewAnalytics:
		return h.View(analyticsHelp())
	default:
		return h.View(dashboardHelp())
	}
}
