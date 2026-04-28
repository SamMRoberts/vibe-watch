package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

type DashboardView struct {
	table       table.Model
	sessions    []*models.Session
	rowSessions []*models.Session
	filter      string
	width       int
	height      int
	filterMode  bool
	filterInput string
}

func NewDashboardView(width, height int) *DashboardView {
	t := table.New(
		table.WithColumns(dashboardColumns(width)),
		table.WithFocused(true),
		table.WithHeight(height-8),
		table.WithWidth(tableWidth(width)),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorPrimary).
		BorderBottom(true).
		Bold(true).
		Foreground(colorGlow)
	s.Selected = s.Selected.
		Foreground(colorText).
		Background(colorSurfaceGlow).
		Bold(true)
	t.SetStyles(s)

	return &DashboardView{
		table:  t,
		width:  width,
		height: height,
	}
}

func (d *DashboardView) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.table.SetColumns(dashboardColumns(width))
	d.table.SetWidth(tableWidth(width))
	d.table.SetHeight(height - 8)
}

func (d *DashboardView) SetSessions(sessions []*models.Session, agentFilter string) {
	d.sessions = sessions
	d.updateTable(agentFilter)
}

func (d *DashboardView) updateTable(agentFilter string) {
	var rows []table.Row
	var rowSessions []*models.Session
	filtered := d.filteredSessions(agentFilter)
	sort.SliceStable(filtered, func(i, j int) bool {
		return dashboardSessionLess(filtered[i], filtered[j])
	})

	groupCount := dashboardGroupCount(filtered)
	lastGroup := ""
	for i, s := range filtered {
		group := dashboardGroupKey(s)
		if groupCount > 1 && group != lastGroup {
			runCount := 1
			for j := i + 1; j < len(filtered) && dashboardGroupKey(filtered[j]) == group; j++ {
				runCount++
			}
			rows = append(rows, d.dashboardGroupRow(s, runCount))
			rowSessions = append(rowSessions, nil)
			lastGroup = group
		}

		rows = append(rows, d.dashboardSessionRow(s))
		rowSessions = append(rowSessions, s)
	}

	d.table.SetRows(rows)
	d.rowSessions = rowSessions
	d.ensureCursorOnSession(1)
}

func (d *DashboardView) filteredSessions(agentFilter string) []*models.Session {
	out := make([]*models.Session, 0, len(d.sessions))
	filterLower := strings.ToLower(d.filter)
	agentFilterLower := strings.ToLower(agentFilter)
	for _, s := range d.sessions {
		if s == nil {
			continue
		}
		if agentFilterLower != "" {
			agentLower := strings.ToLower(string(s.AgentType))
			if !strings.Contains(agentLower, agentFilterLower) {
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

func (d *DashboardView) dashboardSessionRow(s *models.Session) table.Row {
	projectWidth := dashboardColumnWidth(d.table.Columns(), "Project")
	agentWidth := dashboardColumnWidth(d.table.Columns(), "Agent")
	stateWidth := dashboardColumnWidth(d.table.Columns(), "State")
	updatedWidth := dashboardColumnWidth(d.table.Columns(), "Updated")
	return table.Row{
		agentLabel(string(s.AgentType), agentWidth),
		truncateStart(s.ProjectPath, projectWidth),
		compactInt(s.MessageCount()),
		sessionInputTokens(s),
		compactInt(s.TotalOutputTokens()),
		formatTableDuration(s.Duration()),
		statusText(s, stateWidth),
		formatLastUpdated(s.LastUpdated, updatedWidth),
	}
}

func (d *DashboardView) dashboardGroupRow(s *models.Session, count int) table.Row {
	agentWidth := dashboardColumnWidth(d.table.Columns(), "Agent")
	projectWidth := dashboardColumnWidth(d.table.Columns(), "Project")
	return table.Row{
		truncateEnd(dashboardGroupDate(s), agentWidth),
		truncateEnd(fmt.Sprintf("%s · %d sessions", string(s.AgentType), count), projectWidth),
		"",
		"",
		"",
		"",
		"",
		"",
	}
}

func dashboardGroupCount(sessions []*models.Session) int {
	seen := make(map[string]bool)
	for _, session := range sessions {
		seen[dashboardGroupKey(session)] = true
	}
	return len(seen)
}

func dashboardSessionLess(a, b *models.Session) bool {
	aTime := dashboardGroupTime(a)
	bTime := dashboardGroupTime(b)
	if !sameDay(aTime, bTime) {
		return aTime.After(bTime)
	}
	if a.AgentType != b.AgentType {
		return string(a.AgentType) < string(b.AgentType)
	}
	return aTime.After(bTime)
}

func dashboardGroupKey(session *models.Session) string {
	return dashboardGroupDate(session) + "\x00" + string(session.AgentType)
}

func dashboardGroupDate(session *models.Session) string {
	t := dashboardGroupTime(session)
	if t.IsZero() {
		return "Unknown"
	}
	return t.Format("Jan 02, 2006")
}

func dashboardGroupTime(session *models.Session) time.Time {
	if session == nil {
		return time.Time{}
	}
	if !session.LastUpdated.IsZero() {
		return session.LastUpdated
	}
	return session.StartTime
}

func sameDay(a, b time.Time) bool {
	if a.IsZero() || b.IsZero() {
		return a.IsZero() && b.IsZero()
	}
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func dashboardColumns(width int) []table.Column {
	const columnCount = 8

	available := tableWidth(width)
	contentBudget := available - columnCount*2
	if contentBudget < 0 {
		contentBudget = 0
	}

	agentWidth, msgWidth, inputWidth, outputWidth, durationWidth, stateWidth, updatedWidth := 12, 4, 6, 6, 7, 8, 8
	fixedWidth := agentWidth + msgWidth + inputWidth + outputWidth + durationWidth + stateWidth + updatedWidth
	if contentBudget < fixedWidth+8 {
		agentWidth, msgWidth, inputWidth, outputWidth, durationWidth, stateWidth, updatedWidth = 8, 3, 4, 4, 5, 4, 5
		fixedWidth = agentWidth + msgWidth + inputWidth + outputWidth + durationWidth + stateWidth + updatedWidth
	}

	projectWidth := contentBudget - fixedWidth
	if projectWidth < 4 {
		projectWidth = 4
	}

	return []table.Column{
		{Title: "Agent", Width: agentWidth},
		{Title: "Project", Width: projectWidth},
		{Title: "Msg", Width: msgWidth},
		{Title: "In", Width: inputWidth},
		{Title: "Out", Width: outputWidth},
		{Title: "Dur", Width: durationWidth},
		{Title: "State", Width: stateWidth},
		{Title: "Updated", Width: updatedWidth},
	}
}

func dashboardColumnWidth(columns []table.Column, title string) int {
	for _, column := range columns {
		if column.Title == title {
			return column.Width
		}
	}
	return 0
}

func tableWidth(width int) int {
	if width <= 4 {
		return width
	}
	return width - 4
}

func agentLabel(agent string, width int) string {
	label := agentIcon(agent) + " " + compactAgentName(agent)
	if width > 0 && lipgloss.Width(label) > width {
		label = compactAgentName(agent)
	}
	if width > 0 && lipgloss.Width(label) > width {
		label = agentIcon(agent)
	}
	return label
}

func statusText(session *models.Session, width int) string {
	return plainStatusText(sessionStatus(session), width)
}

func formatLastUpdated(lastUpdated time.Time, width int) string {
	if lastUpdated.IsZero() {
		return "-"
	}
	if time.Since(lastUpdated) > 24*time.Hour {
		if width > 0 && width < 6 {
			return lastUpdated.Format("Jan02")
		}
		return lastUpdated.Format("Jan 02")
	}
	if width > 0 && width < 8 {
		return lastUpdated.Format("15:04")
	}
	return lastUpdated.Format("15:04:05")
}

func formatTableDuration(duration time.Duration) string {
	if duration <= 0 {
		return "-"
	}
	return models.FormatDuration(duration)
}

func sessionInputTokens(session *models.Session) string {
	if inputTokensUnavailable(session) {
		return "-"
	}
	return compactInt(session.TotalInputTokens())
}

func inputTokensUnavailable(session *models.Session) bool {
	return session != nil &&
		session.AgentType == models.AgentCopilot &&
		session.IsActive &&
		session.TotalInputTokens() == 0 &&
		session.TotalOutputTokens() > 0
}

func compactInt(value int) string {
	if value < 1_000 {
		return strconv.Itoa(value)
	}
	if value < 1_000_000 {
		return fmt.Sprintf("%.1fk", float64(value)/1_000)
	}
	return fmt.Sprintf("%.1fm", float64(value)/1_000_000)
}

func truncateStart(value string, width int) string {
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}
	runes := []rune(value)
	if width <= 3 {
		return string(runes[len(runes)-width:])
	}
	return "..." + string(runes[len(runes)-(width-3):])
}

func (d *DashboardView) SelectedIndex() int {
	return d.table.Cursor()
}

func (d *DashboardView) SelectedSession() *models.Session {
	idx := d.SelectedIndex()
	if idx < 0 || idx >= len(d.rowSessions) {
		return nil
	}
	return d.rowSessions[idx]
}

func (d *DashboardView) MoveUp() {
	d.table.MoveUp(1)
	d.ensureCursorOnSession(-1)
}

func (d *DashboardView) MoveDown() {
	d.table.MoveDown(1)
	d.ensureCursorOnSession(1)
}

func (d *DashboardView) ensureCursorOnSession(direction int) {
	if len(d.rowSessions) == 0 || d.SelectedSession() != nil {
		return
	}
	if direction == 0 {
		direction = 1
	}
	start := d.table.Cursor()
	for idx := start; idx >= 0 && idx < len(d.rowSessions); idx += direction {
		if d.rowSessions[idx] != nil {
			d.table.SetCursor(idx)
			return
		}
	}
	for idx := start; idx >= 0 && idx < len(d.rowSessions); idx -= direction {
		if d.rowSessions[idx] != nil {
			d.table.SetCursor(idx)
			return
		}
	}
}

func (d *DashboardView) View(agentFilter string) string {
	var sb strings.Builder

	activeCount := 0
	recentCount := 0
	totalTokens := 0
	for _, s := range d.sessions {
		if s.IsActive {
			activeCount++
		}
		if !s.LastUpdated.IsZero() && time.Since(s.LastUpdated) < 10*time.Minute {
			recentCount++
		}
		totalTokens += s.TotalInputTokens() + s.TotalOutputTokens()
	}

	cardWidth := clampInt((d.width-20)/4, 12, 18)
	stats := lipgloss.JoinHorizontal(lipgloss.Top,
		metricCardWidth("Sessions", fmt.Sprintf("%d", len(d.sessions)), "☷", styleAccent, cardWidth),
		"  ",
		metricCardWidth("Active", fmt.Sprintf("%d", activeCount), "✦", styleSuccess, cardWidth),
		"  ",
		metricCardWidth("Recent", fmt.Sprintf("%d", recentCount), "◌", styleInfo, cardWidth),
		"  ",
		metricCardWidth("Tokens", compactInt(totalTokens), "◇", styleAccent, cardWidth),
	)

	sb.WriteString(sectionHeader("Telemetry cockpit", "live agent sessions", d.width-4) + "\n\n")
	sb.WriteString(stats)
	sb.WriteString("\n\n")

	if d.filterMode {
		sb.WriteString(commandPanel(d.width-6, "Filter", d.filterInput, "Type to narrow by project or agent. Press enter to apply, esc to close.") + "\n\n")
	} else if d.filter != "" {
		sb.WriteString(quietPill("filter: "+d.filter) + "\n\n")
	}

	sb.WriteString(sectionHeader("Session grid", agentFilter, d.width-4) + "\n")
	if len(d.sessions) == 0 {
		sb.WriteString(emptyState(
			d.width-4,
			"No sessions found",
			"Run Claude Code, Codex CLI, Copilot CLI, Copilot Chat, or Amazon Q to see sessions appear here.",
		))
	} else {
		sb.WriteString(d.table.View())
	}

	return sb.String()
}
