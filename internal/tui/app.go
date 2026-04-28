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
	sessions []*models.Session
	err      error
}

type App struct {
	watcher     *watcher.Watcher
	sessions    []*models.Session
	agentFilter string
	view        viewState
	dashboard   *DashboardView
	detail      *DetailView
	analytics   *AnalyticsView
	width       int
	height      int
	lastRefresh time.Time
	loading     bool
	filterInput string
	filterMode  bool
	lastErr     error
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
		return sessionsUpdateMsg{sessions: update.Sessions, err: update.Err}
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
		cmds = append(cmds, tickCmd())

	case sessionsUpdateMsg:
		if msg.err == nil {
			a.sessions = msg.sessions
		}
		a.loading = false
		a.lastRefresh = time.Now()
		a.lastErr = msg.err
		a.updateViews()
		cmds = append(cmds, waitForUpdate(a.watcher))

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
				idx := a.dashboard.SelectedIndex()
				filtered := a.filteredSessions()
				if idx >= 0 && idx < len(filtered) {
					a.detail.SetSession(filtered[idx])
					if filtered[idx].IsActive {
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
			a.watcher.Refresh()

		case key.Matches(msg, keys.Filter):
			if a.view == viewDashboard {
				a.filterMode = true
				a.filterInput = ""
			}

		case key.Matches(msg, keys.Up):
			if a.view == viewDashboard && a.dashboard != nil {
				a.dashboard.table.MoveUp(1)
			} else if a.view == viewDetail && a.detail != nil {
				a.detail.SelectPreviousRow()
			} else if a.view == viewPromptDetail && a.detail != nil {
				a.detail.ScrollUp()
			}

		case key.Matches(msg, keys.Down):
			if a.view == viewDashboard && a.dashboard != nil {
				a.dashboard.table.MoveDown(1)
			} else if a.view == viewDetail && a.detail != nil {
				a.detail.SelectNextRow()
			} else if a.view == viewPromptDetail && a.detail != nil {
				a.detail.ScrollDown()
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
			}

		case key.Matches(msg, keys.End):
			if a.view == viewDetail && a.detail != nil {
				a.detail.FollowLatest()
			} else if a.view == viewPromptDetail && a.detail != nil {
				a.detail.viewport.GotoBottom()
			}

		case key.Matches(msg, keys.Collapse):
			if a.view == viewDetail && a.detail != nil {
				a.detail.ToggleSelectedThread()
			}

		case key.Matches(msg, keys.CollapseAll):
			if a.view == viewDetail && a.detail != nil {
				a.detail.ToggleAllThreadsCollapsed()
			}

		case key.Matches(msg, keys.Follow):
			if a.view == viewDetail && a.detail != nil {
				a.detail.ToggleFollow()
			}

		case key.Matches(msg, keys.PageUp):
			if (a.view == viewDetail || a.view == viewPromptDetail) && a.detail != nil {
				a.detail.PageUp()
			}

		case key.Matches(msg, keys.PageDown):
			if (a.view == viewDetail || a.view == viewPromptDetail) && a.detail != nil {
				a.detail.PageDown()
			}
		}
	}

	return a, tea.Batch(cmds...)
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
		a.analytics.SetSessions(a.sessions)
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
	title := styleTitle.Render("◈ vibe-watch")
	subtitleText := "agent session observatory"
	if a.width < 90 {
		subtitleText = "observatory"
	}
	subtitle := styleMuted.Render(subtitleText)
	headerLeft := lipgloss.JoinHorizontal(lipgloss.Bottom, title, "  ", subtitle)
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
	tabDash := styleTab.Render("☷ Dashboard")
	tabAnalytics := styleTab.Render("▣ Analytics")
	switch a.view {
	case viewDashboard:
		tabDash = styleActiveTab.Render("☷ Dashboard")
	case viewAnalytics:
		tabAnalytics = styleActiveTab.Render("▣ Analytics")
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
	if a.width < 90 {
		return ""
	}
	parts := []string{quietPill(fmt.Sprintf("%d sessions", len(a.sessions)))}
	if active > 0 {
		parts = append(parts, statusCountChip(statusActive, active))
	}
	if failed > 0 {
		parts = append(parts, statusCountChip(statusFailed, failed))
	}
	if !a.lastRefresh.IsZero() {
		parts = append(parts, styleMuted.Render(fmt.Sprintf("refreshed %ds", int(time.Since(a.lastRefresh).Seconds()))))
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, joinWithSpaces(parts)...)
}

func (a *App) renderShellFooter() string {
	helpText := a.renderHelpText()
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

func (a *App) renderHelpText() string {
	h := newHelpModel(a.width - 4)
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
