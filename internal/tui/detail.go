package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

type focusedDetailMode int

const (
	focusNone focusedDetailMode = iota
	focusThread
	focusEvent
)

type activityRowKind int

const (
	activityRowMessage activityRowKind = iota
	activityRowCollapsed
)

type activityRow struct {
	kind           activityRowKind
	messageIndex   int
	threadStart    int
	collapsedCount int
}

type DetailView struct {
	viewport         viewport.Model
	session          *models.Session
	header           string
	collapsedThreads map[int]bool
	selectedUser     int
	selectedRow      int
	rows             []activityRow
	rowLineOffsets   map[int]int
	focusedMode      focusedDetailMode
	focusedMessage   int
	follow           bool
	width            int
	height           int
}

func NewDetailView(width, height int) *DetailView {
	vp := viewport.New(width-4, height-10)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1)
	return &DetailView{
		viewport:         vp,
		collapsedThreads: make(map[int]bool),
		selectedUser:     -1,
		selectedRow:      -1,
		focusedMessage:   -1,
		follow:           true,
		width:            width,
		height:           height,
	}
}

func (d *DetailView) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.viewport.Width = width - 4
	d.viewport.Height = height - 10
	if d.session != nil {
		d.renderCurrentContent()
	}
}

func (d *DetailView) SetSession(s *models.Session) {
	same := sameSession(d.session, s)
	selectedMessage := d.selectedMessageIndex()
	if !same {
		d.collapsedThreads = make(map[int]bool)
		d.selectedRow = -1
		d.selectedUser = firstUserIndex(s)
		d.focusedMode = focusNone
		d.focusedMessage = -1
		d.follow = s != nil && s.IsActive
	}

	d.session = s
	d.rebuildRows()

	switch {
	case !same:
		d.selectInitialRow()
	case d.follow && s != nil && s.IsActive:
		d.selectLastRow()
	case selectedMessage >= 0:
		d.selectMessageRow(selectedMessage)
	default:
		d.ensureSelectedRow()
	}

	if d.focusedMode == focusThread && !isUserMessage(d.session, d.focusedMessage) {
		d.focusedMode = focusNone
		d.focusedMessage = -1
	}
	if d.focusedMode == focusEvent && !isMessageIndex(d.session, d.focusedMessage) {
		d.focusedMode = focusNone
		d.focusedMessage = -1
	}
	d.renderCurrentContent()
}

func (d *DetailView) ScrollToBottom() {
	d.viewport.GotoBottom()
}

func (d *DetailView) FollowLatest() {
	d.follow = true
	d.renderContent()
	d.selectLastRow()
	d.scrollSelectedRowIntoView()
	d.viewport.GotoBottom()
}

func (d *DetailView) ToggleFollow() {
	d.follow = !d.follow
	if d.follow {
		d.FollowLatest()
		return
	}
	d.renderContent()
}

func (d *DetailView) Following() bool {
	return d.follow
}

func (d *DetailView) SelectLastUser() {
	if d.session == nil {
		return
	}
	for i := len(d.session.Messages) - 1; i >= 0; i-- {
		if d.session.Messages[i].Role == "user" {
			d.selectMessageRow(i)
			d.renderContent()
			d.scrollSelectedRowIntoView()
			return
		}
	}
}

func (d *DetailView) SelectPreviousRow() {
	d.selectRow(-1)
}

func (d *DetailView) SelectNextRow() {
	d.selectRow(1)
}

func (d *DetailView) SelectPreviousUser() {
	d.selectUser(-1)
}

func (d *DetailView) SelectNextUser() {
	d.selectUser(1)
}

func (d *DetailView) SelectFirstRow() {
	d.pauseFollow()
	d.rebuildRows()
	if len(d.rows) == 0 {
		return
	}
	d.selectedRow = 0
	d.updateSelectedUser()
	d.renderContent()
	d.viewport.GotoTop()
}

func (d *DetailView) SelectLastRow() {
	d.rebuildRows()
	if len(d.rows) == 0 {
		return
	}
	d.selectedRow = len(d.rows) - 1
	d.updateSelectedUser()
	d.renderContent()
	d.scrollSelectedRowIntoView()
}

func (d *DetailView) ToggleSelectedThread() {
	threadStart, ok := d.selectedThreadStart()
	if !ok {
		return
	}
	if d.collapsedThreads == nil {
		d.collapsedThreads = make(map[int]bool)
	}
	d.collapsedThreads[threadStart] = !d.collapsedThreads[threadStart]
	d.selectedUser = threadStart
	d.pauseFollow()
	d.rebuildRows()
	d.selectMessageRow(threadStart)
	d.renderContent()
	d.scrollSelectedRowIntoView()
}

func (d *DetailView) CollapseAllThreads() {
	if d.session == nil {
		return
	}
	if d.collapsedThreads == nil {
		d.collapsedThreads = make(map[int]bool)
	}
	for i, msg := range d.session.Messages {
		if msg.Role == "user" && relatedAssistantCount(d.session.Messages, i) > 0 {
			d.collapsedThreads[i] = true
		}
	}
	d.pauseFollow()
	d.rebuildRows()
	d.ensureSelectedRow()
	d.renderContent()
	d.scrollSelectedRowIntoView()
}

func (d *DetailView) OpenSelectedDetail() bool {
	row, ok := d.selectedActivityRow()
	if !ok || d.session == nil {
		return false
	}
	if row.kind == activityRowCollapsed {
		d.focusedMode = focusThread
		d.focusedMessage = row.threadStart
		d.renderThreadContent()
		d.viewport.GotoTop()
		return true
	}
	if !isMessageIndex(d.session, row.messageIndex) {
		return false
	}
	if d.session.Messages[row.messageIndex].Role == "user" {
		d.focusedMode = focusThread
		d.focusedMessage = row.messageIndex
		d.renderThreadContent()
		d.viewport.GotoTop()
		return true
	}
	d.focusedMode = focusEvent
	d.focusedMessage = row.messageIndex
	d.renderEventContent()
	d.viewport.GotoTop()
	return true
}

func (d *DetailView) OpenSelectedThread() bool {
	threadStart, ok := d.selectedThreadStart()
	if !ok {
		return false
	}
	d.focusedMode = focusThread
	d.focusedMessage = threadStart
	d.renderThreadContent()
	d.viewport.GotoTop()
	return true
}

func (d *DetailView) RefreshSelectedThread() bool {
	return d.RefreshFocusedDetail()
}

func (d *DetailView) RefreshFocusedDetail() bool {
	switch d.focusedMode {
	case focusThread:
		if !isUserMessage(d.session, d.focusedMessage) {
			return false
		}
		d.renderThreadContent()
		return true
	case focusEvent:
		if !isMessageIndex(d.session, d.focusedMessage) {
			return false
		}
		d.renderEventContent()
		return true
	}
	return false
}

func (d *DetailView) ShowSessionDetail() {
	d.focusedMode = focusNone
	d.focusedMessage = -1
	d.renderContent()
	d.scrollSelectedRowIntoView()
}

func (d *DetailView) renderCurrentContent() {
	if d.focusedMode != focusNone {
		if d.RefreshFocusedDetail() {
			return
		}
		d.focusedMode = focusNone
		d.focusedMessage = -1
	}
	d.renderContent()
}

func (d *DetailView) renderContent() {
	if d.session == nil {
		d.header = ""
		d.rows = nil
		d.rowLineOffsets = nil
		d.viewport.SetContent(styleMuted.Render("No session selected"))
		return
	}

	d.rebuildRows()
	d.ensureSelectedRow()

	s := d.session
	header := d.sessionHeader(s)
	var content strings.Builder
	line := 0
	d.rowLineOffsets = make(map[int]int)
	write := func(text string) {
		content.WriteString(text)
		line += strings.Count(text, "\n")
	}

	if len(s.Messages) == 0 {
		write(styleMuted.Render("No activity found in this session.\n"))
	}

	for i, row := range d.rows {
		d.rowLineOffsets[i] = line
		write(d.renderTimelineRow(i, row) + "\n")
	}

	d.setContent(header, content.String())
}

func (d *DetailView) renderThreadContent() {
	if d.session == nil {
		d.header = ""
		d.viewport.SetContent(styleMuted.Render("No session selected"))
		return
	}

	userIndex := d.focusedMessage
	if userIndex < 0 {
		userIndex = d.selectedUser
	}
	start, end, ok := selectedThreadRange(d.session.Messages, userIndex)
	if !ok {
		d.viewport.SetContent(styleMuted.Render("No user message selected"))
		return
	}

	threadMessages := d.session.Messages[start:end]
	var threadTokens models.TokenUsage
	for _, msg := range threadMessages {
		threadTokens.InputTokens += msg.Tokens.InputTokens
		threadTokens.OutputTokens += msg.Tokens.OutputTokens
		threadTokens.CacheReads += msg.Tokens.CacheReads
		threadTokens.CacheWrites += msg.Tokens.CacheWrites
	}

	header := d.threadHeader(start, len(threadMessages), threadTokens)
	var content strings.Builder

	for i, msg := range threadMessages {
		absoluteIndex := start + i
		content.WriteString(renderVerboseMessage(absoluteIndex, msg, d.width-10))
	}
	if len(threadMessages) == 1 {
		content.WriteString(styleMuted.Render("No assistant activity has been recorded for this prompt yet.\n"))
	}

	d.setContent(header, content.String())
}

func (d *DetailView) renderEventContent() {
	if d.session == nil || !isMessageIndex(d.session, d.focusedMessage) {
		d.header = ""
		d.viewport.SetContent(styleMuted.Render("No activity selected"))
		return
	}
	msg := d.session.Messages[d.focusedMessage]
	header := d.eventHeader(d.focusedMessage, msg)
	d.setContent(header, renderVerboseMessage(d.focusedMessage, msg, d.width-10))
}

func (d *DetailView) renderTimelineRow(rowIndex int, row activityRow) string {
	selected := rowIndex == d.selectedRow
	var rendered string
	if row.kind == activityRowCollapsed {
		rendered = d.renderCollapsedRow(row)
	} else {
		rendered = d.renderMessageRow(row)
	}
	if selected {
		return styleSelected.Render(rendered)
	}
	return rendered
}

func (d *DetailView) renderMessageRow(row activityRow) string {
	msg := d.session.Messages[row.messageIndex]
	ts := "--:--:--"
	if !msg.Timestamp.IsZero() {
		ts = msg.Timestamp.Format("15:04:05")
	}
	role := timelineRoleLabel(msg.Role)
	if msg.Role == "user" && relatedAssistantCount(d.session.Messages, row.messageIndex) > 0 {
		if d.collapsedThreads[row.messageIndex] {
			role = styleUserMsg.Render("[+] User")
		} else {
			role = styleUserMsg.Render("[-] User")
		}
	}
	summaryWidth := d.width - 44
	if summaryWidth < 24 {
		summaryWidth = 24
	}
	summary := summarizeActivityContent(msg.Content, summaryWidth)
	if summary == "" {
		summary = "(empty)"
	}
	tokenBadge := timelineTokenBadge(msg.Tokens)
	if tokenBadge != "" {
		tokenBadge = "  " + tokenBadge
	}
	return fmt.Sprintf(
		"%s %s %-14s %s%s",
		styleMuted.Render(fmt.Sprintf("%03d", row.messageIndex+1)),
		styleMuted.Render(ts),
		role,
		styleMessageContent.Render(summary),
		tokenBadge,
	)
}

func (d *DetailView) renderCollapsedRow(row activityRow) string {
	return fmt.Sprintf(
		"    %s %d activity entries collapsed %s",
		styleMuted.Render("╰─"),
		row.collapsedCount,
		styleMuted.Render("(space to expand)"),
	)
}

func (d *DetailView) sessionHeader(s *models.Session) string {
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		agentBadge(string(s.AgentType)),
		styleMuted.Render("  ·  "),
		styleText(s.ProjectPath),
	)

	startStr := "unknown"
	if !s.StartTime.IsZero() {
		startStr = s.StartTime.Format("2006-01-02 15:04:05")
	}

	tokenPanel := lipgloss.JoinHorizontal(lipgloss.Top,
		metricCard("Messages", fmt.Sprintf("%d", len(s.Messages)), "☷", styleAccent),
		"  ",
		metricCard("Input Tokens", detailInputTokens(s), "↘", styleAccent),
		"  ",
		metricCard("Output Tokens", fmt.Sprintf("%d", s.TotalOutputTokens()), "↗", styleAccent),
		"  ",
		metricCard("Cache Reads", fmt.Sprintf("%d", s.TotalTokens.CacheReads), "◌", styleAccent),
	)

	return header + "\n" +
		styleMuted.Render(fmt.Sprintf("Started: %s  Duration: %s", startStr, models.FormatDuration(s.Duration()))) + "\n" +
		statusPill(s.IsActive) + " " + followPill(d.follow, s.IsActive) + "\n\n" +
		tokenPanel + "\n\n" +
		divider(d.width-6)
}

func (d *DetailView) threadHeader(start, messageCount int, threadTokens models.TokenUsage) string {
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		agentBadge(string(d.session.AgentType)),
		styleMuted.Render("  ·  "),
		styleAccent.Render(fmt.Sprintf("Prompt detail %d", start+1)),
	)

	tokenPanel := lipgloss.JoinHorizontal(lipgloss.Top,
		metricCard("Messages", fmt.Sprintf("%d", messageCount), "☷", styleAccent),
		"  ",
		metricCard("Input Tokens", fmt.Sprintf("%d", threadTokens.InputTokens), "↘", styleAccent),
		"  ",
		metricCard("Output Tokens", fmt.Sprintf("%d", threadTokens.OutputTokens), "↗", styleAccent),
		"  ",
		metricCard("Cache Reads", fmt.Sprintf("%d", threadTokens.CacheReads), "◌", styleAccent),
	)

	return header + "\n" +
		styleMuted.Render(d.session.ProjectPath) + "\n\n" +
		tokenPanel + "\n\n" +
		divider(d.width-6)
}

func (d *DetailView) eventHeader(index int, msg models.Message) string {
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		agentBadge(string(d.session.AgentType)),
		styleMuted.Render("  ·  "),
		styleAccent.Render(fmt.Sprintf("Focused activity %d", index+1)),
	)
	tokenPanel := lipgloss.JoinHorizontal(lipgloss.Top,
		metricCard("Role", msg.Role, "●", styleAccent),
		"  ",
		metricCard("Input Tokens", fmt.Sprintf("%d", msg.Tokens.InputTokens), "↘", styleAccent),
		"  ",
		metricCard("Output Tokens", fmt.Sprintf("%d", msg.Tokens.OutputTokens), "↗", styleAccent),
		"  ",
		metricCard("Cache Reads", fmt.Sprintf("%d", msg.Tokens.CacheReads), "◌", styleAccent),
	)
	return header + "\n" +
		styleMuted.Render(d.session.ProjectPath) + "\n\n" +
		tokenPanel + "\n\n" +
		divider(d.width-6)
}

func (d *DetailView) setContent(header, content string) {
	d.header = header
	d.viewport.Height = d.viewportHeight()
	d.viewport.SetContent(content)
}

func (d *DetailView) viewportHeight() int {
	height := d.height - 10 - lipgloss.Height(d.header)
	if height < 3 {
		return 3
	}
	return height
}

func renderVerboseMessage(index int, msg models.Message, contentWidth int) string {
	var sb strings.Builder
	roleLabel := styleMuted.Render("● " + msg.Role)
	switch msg.Role {
	case "user":
		roleLabel = styleUserMsg.Render("▶ User prompt")
	case "assistant":
		roleLabel = styleAssistantMsg.Render("◆ Assistant activity")
	case "tool":
		roleLabel = styleAccent.Render("▣ Tool activity")
	case "subagent":
		roleLabel = styleAccent.Render("◈ Subagent activity")
	case "session":
		roleLabel = styleMuted.Render("◇ Session activity")
	case "error":
		roleLabel = styleError.Render("⚠ Error activity")
	case "system":
		roleLabel = styleMuted.Render("● System activity")
	}

	timestamp := ""
	if !msg.Timestamp.IsZero() {
		timestamp = styleMuted.Render("  " + msg.Timestamp.Format("2006-01-02 15:04:05"))
	}
	sb.WriteString(fmt.Sprintf("%s %s%s\n", styleMuted.Render(fmt.Sprintf("%02d", index+1)), roleLabel, timestamp))

	content := msg.Content
	if content == "" {
		content = "(empty)"
	}
	for _, line := range strings.Split(content, "\n") {
		for _, wrapped := range wrapContentLine(line, contentWidth) {
			sb.WriteString(styleMuted.Render("   │ ") + styleMessageContent.Render(wrapped) + "\n")
		}
	}

	if msg.Tokens.InputTokens > 0 ||
		msg.Tokens.OutputTokens > 0 ||
		msg.Tokens.CacheReads > 0 ||
		msg.Tokens.CacheWrites > 0 {
		sb.WriteString(styleMuted.Render(fmt.Sprintf(
			"   ╰─ tokens in:%d out:%d cache-read:%d cache-write:%d\n",
			msg.Tokens.InputTokens,
			msg.Tokens.OutputTokens,
			msg.Tokens.CacheReads,
			msg.Tokens.CacheWrites,
		)))
	}
	sb.WriteString("\n")
	return sb.String()
}

func wrapContentLine(line string, width int) []string {
	if width < 20 {
		width = 20
	}
	if line == "" {
		return []string{""}
	}
	runes := []rune(line)
	lines := make([]string, 0, len(runes)/width+1)
	for len(runes) > width {
		lines = append(lines, string(runes[:width]))
		runes = runes[width:]
	}
	lines = append(lines, string(runes))
	return lines
}

func timelineRoleLabel(role string) string {
	switch role {
	case "user":
		return styleUserMsg.Render("▶ User")
	case "assistant":
		return styleAssistantMsg.Render("◆ Assistant")
	case "tool":
		return styleAccent.Render("▣ Tool")
	case "subagent":
		return styleAccent.Render("◈ Subagent")
	case "session":
		return styleMuted.Render("◇ Session")
	case "error":
		return styleError.Render("⚠ Error")
	case "system":
		return styleMuted.Render("● System")
	default:
		return styleMuted.Render("● " + role)
	}
}

func timelineTokenBadge(tokens models.TokenUsage) string {
	parts := make([]string, 0, 4)
	if tokens.InputTokens > 0 {
		parts = append(parts, fmt.Sprintf("in:%d", tokens.InputTokens))
	}
	if tokens.OutputTokens > 0 {
		parts = append(parts, fmt.Sprintf("out:%d", tokens.OutputTokens))
	}
	if tokens.CacheReads > 0 {
		parts = append(parts, fmt.Sprintf("cache:%d", tokens.CacheReads))
	}
	if len(parts) == 0 {
		return ""
	}
	return styleMuted.Render(strings.Join(parts, " "))
}

func summarizeActivityContent(content string, width int) string {
	content = strings.TrimSpace(strings.ReplaceAll(content, "\n", " · "))
	content = strings.Join(strings.Fields(content), " ")
	if content == "" || lipgloss.Width(content) <= width {
		return content
	}
	runes := []rune(content)
	if width <= 1 {
		return "…"
	}
	if len(runes) <= width {
		return content
	}
	return string(runes[:width-1]) + "…"
}

func followPill(follow, active bool) string {
	if !active {
		return lipgloss.NewStyle().
			Background(colorSurfaceAlt).
			Foreground(colorMuted).
			Padding(0, 1).
			Render("follow idle")
	}
	if follow {
		return lipgloss.NewStyle().
			Background(colorPrimary).
			Foreground(colorBackground).
			Bold(true).
			Padding(0, 1).
			Render("FOLLOW")
	}
	return lipgloss.NewStyle().
		Background(colorSurfaceAlt).
		Foreground(colorMuted).
		Padding(0, 1).
		Render("PAUSED")
}

func styleText(s string) string {
	return lipgloss.NewStyle().Foreground(colorText).Render(s)
}

func detailInputTokens(session *models.Session) string {
	if inputTokensUnavailable(session) {
		return "-"
	}
	return fmt.Sprintf("%d", session.TotalInputTokens())
}

func (d *DetailView) rebuildRows() {
	if d.session == nil {
		d.rows = nil
		return
	}
	d.rows = buildActivityRows(d.session.Messages, d.collapsedThreads)
}

func buildActivityRows(messages []models.Message, collapsed map[int]bool) []activityRow {
	rows := make([]activityRow, 0, len(messages))
	for i := 0; i < len(messages); i++ {
		rows = append(rows, activityRow{kind: activityRowMessage, messageIndex: i, threadStart: nearestThreadStart(messages, i)})
		if messages[i].Role == "user" && collapsed[i] {
			count := relatedAssistantCount(messages, i)
			if count > 0 {
				rows = append(rows, activityRow{
					kind:           activityRowCollapsed,
					messageIndex:   i,
					threadStart:    i,
					collapsedCount: count,
				})
				i += count
			}
		}
	}
	return rows
}

func (d *DetailView) selectInitialRow() {
	if len(d.rows) == 0 {
		d.selectedRow = -1
		d.selectedUser = -1
		return
	}
	if d.follow && d.session != nil && d.session.IsActive {
		d.selectLastRow()
		return
	}
	d.selectedRow = 0
	d.updateSelectedUser()
}

func (d *DetailView) ensureSelectedRow() {
	if len(d.rows) == 0 {
		d.selectedRow = -1
		d.selectedUser = -1
		return
	}
	if d.selectedRow < 0 {
		d.selectedRow = 0
	}
	if d.selectedRow >= len(d.rows) {
		d.selectedRow = len(d.rows) - 1
	}
	d.updateSelectedUser()
}

func (d *DetailView) selectRow(direction int) {
	if d.session == nil {
		return
	}
	d.pauseFollow()
	d.rebuildRows()
	d.ensureSelectedRow()
	if len(d.rows) == 0 {
		return
	}
	next := d.selectedRow + direction
	if next < 0 {
		next = 0
	}
	if next >= len(d.rows) {
		next = len(d.rows) - 1
	}
	d.selectedRow = next
	d.updateSelectedUser()
	d.renderContent()
	d.scrollSelectedRowIntoView()
}

func (d *DetailView) selectUser(direction int) {
	if d.session == nil {
		return
	}
	d.pauseFollow()
	current := d.currentMessageIndex()
	if current < 0 {
		current = d.selectedUser
	}
	start := current + direction
	for i := start; i >= 0 && i < len(d.session.Messages); i += direction {
		if d.session.Messages[i].Role == "user" {
			d.selectedUser = i
			d.rebuildRows()
			d.selectMessageRow(i)
			d.renderContent()
			d.scrollSelectedRowIntoView()
			return
		}
	}
}

func (d *DetailView) selectMessageRow(messageIndex int) {
	for i, row := range d.rows {
		if row.kind == activityRowMessage && row.messageIndex == messageIndex {
			d.selectedRow = i
			d.updateSelectedUser()
			return
		}
	}
	d.ensureSelectedRow()
}

func (d *DetailView) selectLastRow() {
	if len(d.rows) == 0 {
		d.selectedRow = -1
		d.selectedUser = -1
		return
	}
	d.selectedRow = len(d.rows) - 1
	d.updateSelectedUser()
}

func (d *DetailView) updateSelectedUser() {
	row, ok := d.selectedActivityRow()
	if !ok {
		d.selectedUser = -1
		return
	}
	if row.kind == activityRowCollapsed {
		d.selectedUser = row.threadStart
		return
	}
	d.selectedUser = nearestThreadStart(d.session.Messages, row.messageIndex)
}

func (d *DetailView) selectedActivityRow() (activityRow, bool) {
	if d.selectedRow < 0 || d.selectedRow >= len(d.rows) {
		return activityRow{}, false
	}
	return d.rows[d.selectedRow], true
}

func (d *DetailView) currentMessageIndex() int {
	row, ok := d.selectedActivityRow()
	if !ok {
		return -1
	}
	if row.kind == activityRowCollapsed {
		return row.threadStart
	}
	return row.messageIndex
}

func (d *DetailView) selectedMessageIndex() int {
	if d.focusedMode != focusNone && d.focusedMessage >= 0 {
		return d.focusedMessage
	}
	return d.currentMessageIndex()
}

func (d *DetailView) selectedThreadStart() (int, bool) {
	row, ok := d.selectedActivityRow()
	if !ok {
		return 0, false
	}
	if row.kind == activityRowCollapsed {
		return row.threadStart, true
	}
	if isUserMessage(d.session, row.messageIndex) {
		return row.messageIndex, true
	}
	return 0, false
}

func (d *DetailView) scrollSelectedRowIntoView() {
	line, ok := d.rowLineOffsets[d.selectedRow]
	if !ok {
		return
	}
	if line < d.viewport.YOffset {
		d.viewport.SetYOffset(line)
		return
	}
	visibleHeight := d.viewport.Height - d.viewport.Style.GetVerticalFrameSize()
	if visibleHeight < 1 {
		visibleHeight = 1
	}
	if line >= d.viewport.YOffset+visibleHeight {
		d.viewport.SetYOffset(line - visibleHeight + 1)
	}
}

func (d *DetailView) ScrollDown() {
	d.pauseFollow()
	d.viewport.ScrollDown(3)
}

func (d *DetailView) ScrollUp() {
	d.pauseFollow()
	d.viewport.ScrollUp(3)
}

func (d *DetailView) PageDown() {
	d.pauseFollow()
	d.viewport.HalfViewDown()
}

func (d *DetailView) PageUp() {
	d.pauseFollow()
	d.viewport.HalfViewUp()
}

func (d *DetailView) AtBottom() bool {
	return d.viewport.AtBottom()
}

func (d *DetailView) pauseFollow() {
	if d.session != nil && d.session.IsActive {
		d.follow = false
	}
}

func (d *DetailView) View() string {
	if d.session == nil {
		return styleCard.
			Width(d.width - 4).
			Height(d.height - 6).
			Align(lipgloss.Center).
			Render(styleMuted.Render("Select a session from the dashboard to view details.\n\nPress esc to go back."))
	}

	rowLabel := "0/0"
	if len(d.rows) > 0 && d.selectedRow >= 0 {
		rowLabel = fmt.Sprintf("%d/%d", d.selectedRow+1, len(d.rows))
	}
	followLabel := "paused"
	if d.follow {
		followLabel = "follow"
	}
	footer := styleMuted.Render(fmt.Sprintf(
		"  %d%%  row %s  %s  ↑/↓ activity  [/ ] prompts  enter detail  space collapse  f follow  esc back",
		int(d.viewport.ScrollPercent()*100),
		rowLabel,
		followLabel,
	))

	return d.header + "\n" + d.viewport.View() + "\n" + footer
}

func (d *DetailView) ThreadView() string {
	if d.session == nil {
		return d.View()
	}

	label := "focused activity"
	if d.focusedMode == focusThread {
		label = "verbose prompt detail"
	}
	footer := styleMuted.Render(fmt.Sprintf("  %d%%  %s  ↑↓/pgup/pgdown scroll  esc back",
		int(d.viewport.ScrollPercent()*100),
		label,
	))

	return d.header + "\n" + d.viewport.View() + "\n" + footer
}

func firstUserIndex(session *models.Session) int {
	if session == nil {
		return -1
	}
	for i, msg := range session.Messages {
		if msg.Role == "user" {
			return i
		}
	}
	return -1
}

func isUserMessage(session *models.Session, index int) bool {
	return session != nil &&
		index >= 0 &&
		index < len(session.Messages) &&
		session.Messages[index].Role == "user"
}

func isMessageIndex(session *models.Session, index int) bool {
	return session != nil &&
		index >= 0 &&
		index < len(session.Messages)
}

func nearestThreadStart(messages []models.Message, index int) int {
	for i := index; i >= 0 && i < len(messages); i-- {
		if messages[i].Role == "user" {
			return i
		}
	}
	return -1
}

func relatedAssistantCount(messages []models.Message, userIndex int) int {
	count := 0
	for i := userIndex + 1; i < len(messages); i++ {
		if messages[i].Role == "user" {
			break
		}
		count++
	}
	return count
}

func selectedThreadRange(messages []models.Message, userIndex int) (int, int, bool) {
	if userIndex < 0 || userIndex >= len(messages) || messages[userIndex].Role != "user" {
		return 0, 0, false
	}
	end := len(messages)
	for i := userIndex + 1; i < len(messages); i++ {
		if messages[i].Role == "user" {
			end = i
			break
		}
	}
	return userIndex, end, true
}
