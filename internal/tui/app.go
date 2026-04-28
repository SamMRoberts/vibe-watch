package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SamMRoberts/vibe-watch/internal/models"
	"github.com/SamMRoberts/vibe-watch/internal/watcher"
)

type viewState int

const (
	viewDashboard viewState = iota
	viewDetail
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
		case msg.String() == "ctrl+c" || (msg.String() == "q" && a.view != viewDetail):
			a.watcher.Stop()
			return a, tea.Quit

		case msg.String() == "tab":
			a.view = (a.view + 1) % 3
			if a.view == viewDetail {
				a.view = viewAnalytics
			}

		case msg.String() == "shift+tab":
			if a.view == 0 {
				a.view = viewAnalytics
			} else {
				a.view--
				if a.view == viewDetail {
					a.view = viewDashboard
				}
			}

		case msg.String() == "esc":
			if a.view == viewDetail {
				a.view = viewDashboard
			}

		case msg.String() == "enter":
			if a.view == viewDashboard && a.dashboard != nil {
				idx := a.dashboard.SelectedIndex()
				filtered := a.filteredSessions()
				if idx >= 0 && idx < len(filtered) {
					a.detail.SetSession(filtered[idx])
					a.view = viewDetail
				}
			}

		case msg.String() == "r":
			a.watcher.Refresh()

		case msg.String() == "/":
			if a.view == viewDashboard {
				a.filterMode = true
				a.filterInput = ""
			}

		case msg.String() == "up" || msg.String() == "k":
			if a.view == viewDashboard && a.dashboard != nil {
				a.dashboard.table.MoveUp(1)
			} else if a.view == viewDetail && a.detail != nil {
				a.detail.ScrollUp()
			}

		case msg.String() == "down" || msg.String() == "j":
			if a.view == viewDashboard && a.dashboard != nil {
				a.dashboard.table.MoveDown(1)
			} else if a.view == viewDetail && a.detail != nil {
				a.detail.ScrollDown()
			}

		case msg.String() == "pgup" || msg.String() == "b":
			if a.view == viewDetail && a.detail != nil {
				a.detail.PageUp()
			}

		case msg.String() == "pgdown" || msg.String() == "f":
			if a.view == viewDetail && a.detail != nil {
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
		a.detail.SetSession(updated)
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

	var sb strings.Builder

	// Header bar
	title := styleTitle.Render("⚡ vibe-watch")
	subtitle := styleMuted.Render("◆ Agentic Session Monitor")

	tabDash := styleTab.Render("☷ Dashboard")
	tabAnalytics := styleTab.Render("▣ Analytics")

	switch a.view {
	case viewDashboard:
		tabDash = styleActiveTab.Render("☷ Dashboard")
	case viewAnalytics:
		tabAnalytics = styleActiveTab.Render("▣ Analytics")
	case viewDetail:
		tabDash = styleActiveTab.Render("◈ Detail")
	}

	refreshStr := ""
	if !a.lastRefresh.IsZero() {
		refreshStr = styleMuted.Render(fmt.Sprintf("refreshed %ds ago", int(time.Since(a.lastRefresh).Seconds())))
	}

	headerLeft := lipgloss.JoinHorizontal(lipgloss.Bottom, title, "  ", subtitle)
	headerTabs := lipgloss.JoinHorizontal(lipgloss.Bottom, tabDash, tabAnalytics)
	headerRight := refreshStr

	headerWidth := a.width - lipgloss.Width(headerLeft) - lipgloss.Width(headerRight)
	if headerWidth < 0 {
		headerWidth = 0
	}
	headerMiddle := lipgloss.NewStyle().Width(headerWidth).Align(lipgloss.Center).Render(headerTabs)

	header := lipgloss.NewStyle().
		Background(colorSurface).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(colorPrimary).
		Padding(0, 1).
		Width(a.width).
		Render(lipgloss.JoinHorizontal(lipgloss.Bottom, headerLeft, headerMiddle, headerRight))

	sb.WriteString(header + "\n\n")

	// Content
	switch a.view {
	case viewDashboard:
		if a.dashboard == nil {
			sb.WriteString(styleMuted.Render("Initializing..."))
		} else {
			if a.loading {
				sb.WriteString(stylePanel.Width(a.width - 6).Render(styleAccent.Render("✦ Loading sessions...")))
			} else {
				sb.WriteString(a.dashboard.View(a.agentFilter))
			}
		}
	case viewDetail:
		if a.detail != nil {
			sb.WriteString(a.detail.View())
		}
	case viewAnalytics:
		if a.analytics != nil {
			sb.WriteString(a.analytics.View())
		}
	}

	// Footer help
	helpText := styleMuted.Render("  q quit  │  tab/shift+tab views  │  ↑↓ navigate  │  enter select  │  r refresh  │  / filter")
	if a.view == viewDetail {
		helpText = styleMuted.Render("  q quit  │  esc back  │  ↑↓ scroll  │  pgup/pgdn page")
	}
	if a.lastErr != nil {
		helpText = styleError.Render(fmt.Sprintf("  ⚠ detection error: %v", a.lastErr))
	}

	// Pad to bottom
	contentHeight := a.height - 2 // header + footer
	currentLines := strings.Count(sb.String(), "\n")
	paddingLines := contentHeight - currentLines - 1
	for i := 0; i < paddingLines; i++ {
		sb.WriteString("\n")
	}

	footer := lipgloss.NewStyle().
		Background(colorSurface).
		Foreground(colorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(colorPrimary).
		Width(a.width).
		Render(helpText)

	sb.WriteString(footer)

	return sb.String()
}
