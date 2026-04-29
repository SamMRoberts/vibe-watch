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

const (
	dashboardGroupColumnTitle   = "Date/Agent"
	dashboardSessionColumnTitle = "Session"
	selectionGutterWidth        = 2
)

type dashboardRowKind int

const (
	dashboardRowSession dashboardRowKind = iota
	dashboardRowDate
	dashboardRowAgent
)

type DashboardView struct {
	table       table.Model
	sessions    []*models.Session
	rowSessions []*models.Session
	rowKinds    []dashboardRowKind
	rowAgents   []string
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
	var rowKinds []dashboardRowKind
	var rowAgents []string
	filtered := d.filteredSessions(agentFilter)
	sort.SliceStable(filtered, func(i, j int) bool {
		return dashboardSessionLess(filtered[i], filtered[j])
	})

	groupRows := len(filtered) > 1
	lastDate := ""
	lastGroup := ""
	for i, s := range filtered {
		date := dashboardGroupDate(s)
		group := dashboardGroupKey(s)
		if groupRows && date != lastDate {
			rows = append(rows, d.dashboardDateRow(s, dashboardDateRunCount(filtered, i)))
			rowSessions = append(rowSessions, nil)
			rowKinds = append(rowKinds, dashboardRowDate)
			rowAgents = append(rowAgents, "")
			lastDate = date
			lastGroup = ""
		}
		if groupRows && group != lastGroup {
			runCount := dashboardAgentRunCount(filtered, i)
			lastAgentInDate := i+runCount >= len(filtered) || dashboardGroupDate(filtered[i+runCount]) != date
			rows = append(rows, d.dashboardAgentRow(s, runCount, lastAgentInDate))
			rowSessions = append(rowSessions, nil)
			rowKinds = append(rowKinds, dashboardRowAgent)
			rowAgents = append(rowAgents, string(s.AgentType))
			lastGroup = group
		}

		lastSessionInAgent := true
		if groupRows && i+1 < len(filtered) {
			lastSessionInAgent = dashboardGroupKey(filtered[i+1]) != group
		}
		rows = append(rows, d.dashboardSessionRow(s, groupRows, lastSessionInAgent))
		rowSessions = append(rowSessions, s)
		rowKinds = append(rowKinds, dashboardRowSession)
		rowAgents = append(rowAgents, string(s.AgentType))
	}

	d.table.SetRows(rows)
	d.rowSessions = rowSessions
	d.rowKinds = rowKinds
	d.rowAgents = rowAgents
	d.ensureCursorOnSession(1)
}

func dashboardDateRunCount(sessions []*models.Session, start int) int {
	if start < 0 || start >= len(sessions) {
		return 0
	}
	date := dashboardGroupDate(sessions[start])
	count := 0
	for i := start; i < len(sessions) && dashboardGroupDate(sessions[i]) == date; i++ {
		count++
	}
	return count
}

func dashboardAgentRunCount(sessions []*models.Session, start int) int {
	if start < 0 || start >= len(sessions) {
		return 0
	}
	group := dashboardGroupKey(sessions[start])
	count := 0
	for i := start; i < len(sessions) && dashboardGroupKey(sessions[i]) == group; i++ {
		count++
	}
	return count
}

func sessionCountLabel(count int) string {
	if count == 1 {
		return "1 session"
	}
	return fmt.Sprintf("%d sessions", count)
}

func (d *DashboardView) dashboardSessionRow(s *models.Session, grouped, lastInAgent bool) table.Row {
	sessionWidth := dashboardColumnWidth(d.table.Columns(), dashboardSessionColumnTitle)
	agentWidth := dashboardColumnWidth(d.table.Columns(), dashboardGroupColumnTitle)
	stateWidth := dashboardColumnWidth(d.table.Columns(), "State")
	updatedWidth := dashboardColumnWidth(d.table.Columns(), "Updated")
	agentCell := agentLabel(string(s.AgentType), agentWidth)
	if grouped {
		agentCell = sessionBranchLabel(agentWidth, lastInAgent)
	}
	return table.Row{
		agentCell,
		truncateStart(s.ProjectPath, sessionWidth),
		compactInt(s.MessageCount()),
		sessionInputTokens(s),
		compactInt(s.TotalOutputTokens()),
		formatTableDuration(s.Duration()),
		statusText(s, stateWidth),
		formatLastUpdated(s.LastUpdated, updatedWidth),
	}
}

func (d *DashboardView) dashboardDateRow(s *models.Session, count int) table.Row {
	agentWidth := dashboardColumnWidth(d.table.Columns(), dashboardGroupColumnTitle)
	sessionWidth := dashboardColumnWidth(d.table.Columns(), dashboardSessionColumnTitle)
	return table.Row{
		dateGroupLabel(dashboardGroupDate(s), agentWidth),
		truncateEnd(sessionCountLabel(count), sessionWidth),
		"",
		"",
		"",
		"",
		"",
		"",
	}
}

func (d *DashboardView) dashboardAgentRow(s *models.Session, count int, lastInDate bool) table.Row {
	agentWidth := dashboardColumnWidth(d.table.Columns(), dashboardGroupColumnTitle)
	sessionWidth := dashboardColumnWidth(d.table.Columns(), dashboardSessionColumnTitle)
	return table.Row{
		agentBranchLabel(string(s.AgentType), agentWidth, lastInDate),
		truncateEnd(sessionCountLabel(count), sessionWidth),
		"",
		"",
		"",
		"",
		"",
		"",
	}
}

func agentBranchLabel(agent string, width int, lastInDate bool) string {
	prefix := "├─ "
	if lastInDate {
		prefix = "└─ "
	}
	return prefixedAgentLabel(prefix, agent, width)
}

func sessionBranchLabel(width int, lastInAgent bool) string {
	prefix := "  ├─"
	if lastInAgent {
		prefix = "  └─"
	}
	return truncateEnd(prefix, width)
}

func prefixedAgentLabel(prefix, agent string, width int) string {
	if width <= 0 {
		return prefix + agentLabel(agent, 0)
	}
	available := width - lipgloss.Width(prefix)
	if available < 1 {
		return truncateEnd(prefix, width)
	}
	return prefix + agentLabel(agent, available)
}

func dateGroupLabel(date string, width int) string {
	label := "◷ " + date
	if width > 0 && lipgloss.Width(label) > width {
		label = "◷ " + compactDateLabel(date)
	}
	if width > 0 && lipgloss.Width(label) > width {
		label = "◷"
	}
	return truncateEnd(label, width)
}

func compactDateLabel(date string) string {
	if date == "Unknown" {
		return date
	}
	t, err := time.Parse("Jan 02, 2006", date)
	if err != nil {
		return date
	}
	return t.Format("Jan 02")
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
	if !session.StartTime.IsZero() {
		return session.StartTime
	}
	return session.LastUpdated
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

	agentWidth, msgWidth, inputWidth, outputWidth, durationWidth, stateWidth, updatedWidth := 15, 4, 6, 6, 7, 8, 8
	fixedWidth := agentWidth + msgWidth + inputWidth + outputWidth + durationWidth + stateWidth + updatedWidth
	if contentBudget < fixedWidth+4 {
		agentWidth, msgWidth, inputWidth, outputWidth, durationWidth, stateWidth, updatedWidth = 8, 3, 4, 4, 5, 4, 5
		fixedWidth = agentWidth + msgWidth + inputWidth + outputWidth + durationWidth + stateWidth + updatedWidth
	}

	projectWidth := contentBudget - fixedWidth
	if projectWidth < 4 {
		projectWidth = 4
	}

	return []table.Column{
		{Title: dashboardGroupColumnTitle, Width: agentWidth},
		{Title: dashboardSessionColumnTitle, Width: projectWidth},
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
	if width <= 4+selectionGutterWidth {
		return width
	}
	return width - 4 - selectionGutterWidth
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

func (d *DashboardView) dashboardTableView() string {
	rows := d.table.Rows()
	if len(rows) == 0 {
		return d.dashboardHeaderView()
	}

	visibleHeight := d.table.Height()
	if visibleHeight < 1 || visibleHeight > len(rows) {
		visibleHeight = len(rows)
	}
	cursor := d.table.Cursor()
	start := 0
	if len(rows) > visibleHeight {
		start = clampInt(cursor-visibleHeight/2, 0, len(rows)-visibleHeight)
	}
	end := start + visibleHeight

	renderedRows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		renderedRows = append(renderedRows, d.dashboardRenderRow(i))
	}
	return d.dashboardHeaderView() + "\n" + lipgloss.JoinVertical(lipgloss.Left, renderedRows...)
}

func (d *DashboardView) dashboardHeaderView() string {
	cells := make([]string, 0, len(d.table.Columns()))
	for _, column := range d.table.Columns() {
		if column.Width <= 0 {
			continue
		}
		content := lipgloss.NewStyle().
			Width(column.Width).
			MaxWidth(column.Width).
			Inline(true).
			Render(truncateEnd(column.Title, column.Width))
		cells = append(cells, lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorPrimary).
			BorderBottom(true).
			Bold(true).
			Foreground(colorGlow).
			Padding(0, 1).
			Render(content))
	}
	return selectionGutter(false) + lipgloss.JoinHorizontal(lipgloss.Top, cells...)
}

func (d *DashboardView) dashboardRenderRow(rowIndex int) string {
	row := d.table.Rows()[rowIndex]
	columns := d.table.Columns()
	cells := make([]string, 0, len(columns))
	for i, column := range columns {
		if column.Width <= 0 || i >= len(row) {
			continue
		}
		if i == 0 {
			cells = append(cells, d.dashboardGroupCell(rowIndex, row[i], column.Width))
			continue
		}
		cells = append(cells, dashboardCell(row[i], column.Width, d.dashboardCellStyle(rowIndex, i)))
	}

	rendered := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
	if rowIndex == d.table.Cursor() {
		return selectionGutter(true) + lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorSurfaceGlow).
			Bold(true).
			Render(rendered)
	}
	return selectionGutter(false) + rendered
}

func selectionGutter(selected bool) string {
	if selected {
		return styleAccent.Render("▌ ")
	}
	return "  "
}

func (d *DashboardView) dashboardCellStyle(rowIndex, columnIndex int) lipgloss.Style {
	switch d.dashboardRowKind(rowIndex) {
	case dashboardRowDate:
		if columnIndex == 1 {
			return styleGlow()
		}
	case dashboardRowAgent:
		if columnIndex == 1 {
			return styleMuted
		}
	}
	return lipgloss.NewStyle()
}

func (d *DashboardView) dashboardGroupCell(rowIndex int, value string, width int) string {
	plain := truncateEnd(value, width)
	switch d.dashboardRowKind(rowIndex) {
	case dashboardRowDate:
		return dashboardRenderedCell(styleGlow().Render(plain), width)
	case dashboardRowAgent:
		prefix, label := splitBranchLabel(plain)
		agent := d.dashboardRowAgent(rowIndex)
		return dashboardRenderedCell(styleMuted.Render(prefix)+agentStyle(agent).Render(label), width)
	case dashboardRowSession:
		agent := d.dashboardRowAgent(rowIndex)
		if strings.HasPrefix(plain, "  ") {
			return dashboardRenderedCell(styleMuted.Render(plain), width)
		}
		return dashboardRenderedCell(agentStyle(agent).Render(plain), width)
	default:
		return dashboardCell(plain, width, lipgloss.NewStyle())
	}
}

func (d *DashboardView) dashboardRowKind(rowIndex int) dashboardRowKind {
	if rowIndex < 0 || rowIndex >= len(d.rowKinds) {
		return dashboardRowSession
	}
	return d.rowKinds[rowIndex]
}

func (d *DashboardView) dashboardRowAgent(rowIndex int) string {
	if rowIndex < 0 || rowIndex >= len(d.rowAgents) {
		return ""
	}
	return d.rowAgents[rowIndex]
}

func dashboardCell(value string, width int, style lipgloss.Style) string {
	return dashboardRenderedCell(style.Render(truncateEnd(value, width)), width)
}

func dashboardRenderedCell(rendered string, width int) string {
	if width > 0 {
		if padding := width - lipgloss.Width(rendered); padding > 0 {
			rendered += strings.Repeat(" ", padding)
		}
	}
	return " " + rendered + " "
}

func splitBranchLabel(value string) (string, string) {
	for _, prefix := range []string{"├─ ", "└─ "} {
		if strings.HasPrefix(value, prefix) {
			return prefix, strings.TrimPrefix(value, prefix)
		}
	}
	return "", value
}

func styleGlow() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(colorGlow).
		Bold(true)
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
		sb.WriteString(d.dashboardTableView())
	}

	return sb.String()
}
