package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

type focusedDetailMode int

const (
	focusNone focusedDetailMode = iota
	focusThread
	focusEvent
)

type timelineDetailLevel int

const (
	timelineDetailCompact timelineDetailLevel = iota
	timelineDetailStandard
	timelineDetailExpanded
)

type activityRowKind int

const (
	activityRowMessage activityRowKind = iota
	activityRowCollapsed
	activityRowActionGroup
)

type activityRow struct {
	kind            activityRowKind
	messageIndex    int
	endMessageIndex int
	threadStart     int
	collapsedCount  int
}

type timelineTreeNode struct {
	rowIndex int
	label    string
	children []*timelineTreeNode
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
	userPausedFollow bool // tracks if user explicitly paused follow vs it being false for other reasons
	detailLevel      timelineDetailLevel
	showTimestamps   bool
	animationFrame   int
	width            int
	height           int
}

func NewDetailView(width, height int) *DetailView {
	vp := viewport.New(width-4, height-8)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorSubtle).
		Padding(0, 1)
	return &DetailView{
		viewport:         vp,
		collapsedThreads: make(map[int]bool),
		selectedUser:     -1,
		selectedRow:      -1,
		focusedMessage:   -1,
		follow:           true,
		detailLevel:      timelineDetailStandard,
		width:            width,
		height:           height,
	}
}

func (d *DetailView) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.viewport.Width = width - 4
	d.viewport.Height = height - 8
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
		d.userPausedFollow = false
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
	d.userPausedFollow = false
	d.renderContent()
	d.selectLastRow()
	d.scrollSelectedRowIntoView()
	d.viewport.GotoBottom()
}

func (d *DetailView) ToggleFollow() {
	d.follow = !d.follow
	if d.follow {
		d.userPausedFollow = false
		d.FollowLatest()
		return
	}
	d.userPausedFollow = true
	d.renderContent()
}

func (d *DetailView) Following() bool {
	return d.follow
}

func (d *DetailView) UserPausedFollow() bool {
	return d.userPausedFollow
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

func (d *DetailView) ToggleAllThreadsCollapsed() {
	if d.session == nil {
		return
	}
	if d.allThreadsCollapsed() {
		d.collapsedThreads = make(map[int]bool)
	} else {
		if d.collapsedThreads == nil {
			d.collapsedThreads = make(map[int]bool)
		}
		for i, msg := range d.session.Messages {
			if msg.Role == "user" && relatedAssistantCount(d.session.Messages, i) > 0 {
				d.collapsedThreads[i] = true
			}
		}
	}
	d.pauseFollow()
	d.rebuildRows()
	d.ensureSelectedRow()
	d.renderContent()
	d.scrollSelectedRowIntoView()
}

func (d *DetailView) ToggleTimelineDetailLevel() {
	d.detailLevel = (d.detailLevel + 1) % 3
	d.renderContent()
	d.scrollSelectedRowIntoView()
}

func (d *DetailView) ToggleTimestamps() {
	d.showTimestamps = !d.showTimestamps
	d.renderContent()
	d.scrollSelectedRowIntoView()
}

func (d *DetailView) AdvanceAnimation() {
	d.animationFrame++
	if d.focusedMode == focusNone {
		d.renderContent()
	}
}

func (d *DetailView) timelineDetailLabel() string {
	switch d.detailLevel {
	case timelineDetailCompact:
		return "compact"
	case timelineDetailExpanded:
		return "expanded"
	default:
		return "standard"
	}
}

func (d *DetailView) timestampLabel() string {
	if d.showTimestamps {
		return "time on"
	}
	return "time off"
}

func (d *DetailView) allThreadsCollapsed() bool {
	if d.session == nil {
		return false
	}
	foundCollapsible := false
	for i, msg := range d.session.Messages {
		if msg.Role != "user" || relatedAssistantCount(d.session.Messages, i) == 0 {
			continue
		}
		foundCollapsible = true
		if d.collapsedThreads == nil || !d.collapsedThreads[i] {
			return false
		}
	}
	return foundCollapsible
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

	for i := 0; i < len(d.rows); {
		row := d.rows[i]
		if !rowStartsUserContainer(d.session, row) {
			d.rowLineOffsets[i] = line
			write(d.renderTimelineRow(i, row) + "\n")
			if i < len(d.rows)-1 {
				write("\n")
			}
			i++
			continue
		}

		end := i + 1
		for end < len(d.rows) && !rowStartsUserContainer(d.session, d.rows[end]) {
			end++
		}
		node := d.timelineTree(i, end)
		d.recordTimelineTreeOffsets(node, line)
		selectedLine := -1
		if selectedOffset, ok := d.rowLineOffsets[d.selectedRow]; ok {
			selectedLine = selectedOffset - line
		}
		rendered := renderTimelineTreeWithSelection(renderTimelineTree(node), selectedLine)
		write(rendered + "\n")
		if end < len(d.rows) {
			write("\n")
		}
		i = end
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
	d.setContent(header, d.renderFocusedEventBody(d.focusedMessage))
}

func (d *DetailView) renderFocusedEventBody(index int) string {
	if d.session == nil || !isMessageIndex(d.session, index) {
		return styleMuted.Render("No activity selected")
	}

	var content strings.Builder
	content.WriteString(d.renderFocusedEventMeta(index))
	content.WriteString(renderVerboseMessage(index, d.session.Messages[index], d.width-10))
	if endIndex, ok := matchingActionEnd(d.session.Messages, index); ok {
		content.WriteString(styleMuted.Render("Grouped completion\n"))
		content.WriteString(d.renderFocusedEventMeta(endIndex))
		content.WriteString(renderVerboseMessage(endIndex, d.session.Messages[endIndex], d.width-10))
		return content.String()
	}
	if isActionStart(d.session.Messages[index]) {
		content.WriteString(styleMuted.Render("Waiting for completion for this action.\n"))
	}
	return content.String()
}

func (d *DetailView) renderTimelineRow(rowIndex int, row activityRow) string {
	selected := rowIndex == d.selectedRow
	var rendered string
	switch row.kind {
	case activityRowCollapsed:
		rendered = d.renderCollapsedRow(row)
	case activityRowActionGroup:
		rendered = d.renderActionGroupRow(row)
	default:
		rendered = d.renderMessageRow(rowIndex, row)
	}
	if selected {
		return selectionGutter(true) + styleSelected.Render(rendered)
	}
	return selectionGutter(false) + rendered
}

func (d *DetailView) timelineTree(start, end int) *timelineTreeNode {
	root := d.newTimelineTreeNode(start, d.rows[start])
	nodesByID := make(map[string]*timelineTreeNode)
	eventAliases := make(map[string]string)
	if id := d.timelineRowEventID(d.rows[start]); id != "" {
		nodesByID[id] = root
	}

	type pendingTimelineNode struct {
		rowIndex int
		row      activityRow
		node     *timelineTreeNode
	}
	pending := make([]pendingTimelineNode, 0, end-start-1)
	for i := start + 1; i < end; i++ {
		row := d.rows[i]
		node := d.newTimelineTreeNode(i, row)
		pending = append(pending, pendingTimelineNode{rowIndex: i, row: row, node: node})
		if id := d.timelineRowEventID(row); id != "" {
			nodesByID[id] = node
			if endID := d.timelineActionEndEventID(row); endID != "" {
				eventAliases[endID] = id
			}
		}
	}

	var currentAssistant *timelineTreeNode
	for _, item := range pending {
		parentID := timelineResolveEventAlias(d.timelineRowParentEventID(item.row), eventAliases)
		if parentID != "" {
			if parent := nodesByID[parentID]; parent != nil && parent != item.node {
				parent.children = append(parent.children, item.node)
				if d.rowIsAssistantMessage(item.row) {
					currentAssistant = item.node
				}
				continue
			}
		}
		if d.rowIsAssistantMessage(item.row) {
			root.children = append(root.children, item.node)
			currentAssistant = item.node
			continue
		}
		if currentAssistant != nil && item.row.kind != activityRowCollapsed {
			currentAssistant.children = append(currentAssistant.children, item.node)
			continue
		}
		root.children = append(root.children, item.node)
	}
	return root
}

func (d *DetailView) newTimelineTreeNode(rowIndex int, row activityRow) *timelineTreeNode {
	node := &timelineTreeNode{
		rowIndex: rowIndex,
		label:    d.renderTimelineTreeLabel(rowIndex, row),
	}
	if detail := d.timelineActionDetail(row); detail != "" && d.detailLevel != timelineDetailCompact {
		node.children = append(node.children, &timelineTreeNode{
			rowIndex: -1,
			label:    styleMuted.Render(summarizeActivityContent(detail, d.width-24)),
		})
	}
	for _, detail := range d.timelineExpandedRowDetails(row) {
		node.children = append(node.children, &timelineTreeNode{
			rowIndex: -1,
			label:    styleMuted.Render(detail),
		})
	}
	return node
}

func (d *DetailView) timelineRowEventID(row activityRow) string {
	if d.session == nil || !isMessageIndex(d.session, row.messageIndex) {
		return ""
	}
	return d.session.Messages[row.messageIndex].Meta.EventID
}

func (d *DetailView) timelineRowParentEventID(row activityRow) string {
	if d.session == nil || !isMessageIndex(d.session, row.messageIndex) {
		return ""
	}
	return d.session.Messages[row.messageIndex].Meta.EventParentID
}

func (d *DetailView) timelineActionEndEventID(row activityRow) string {
	if d.session == nil || row.kind != activityRowActionGroup || !isMessageIndex(d.session, row.endMessageIndex) {
		return ""
	}
	return d.session.Messages[row.endMessageIndex].Meta.EventID
}

func timelineResolveEventAlias(id string, aliases map[string]string) string {
	seen := make(map[string]bool)
	for id != "" && aliases[id] != "" && !seen[id] {
		seen[id] = true
		id = aliases[id]
	}
	return id
}

func (d *DetailView) renderTimelineTreeLabel(rowIndex int, row activityRow) string {
	var rendered string
	switch row.kind {
	case activityRowCollapsed:
		rendered = d.renderCollapsedTreeLabel(row)
	case activityRowActionGroup:
		rendered = d.renderActionGroupTreeLabel(row)
	default:
		rendered = d.renderMessageTreeLabel(rowIndex, row)
	}
	return rendered
}

func renderTimelineTreeWithSelection(rendered string, selectedLine int) string {
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		lines[i] = selectionGutter(i == selectedLine) + line
	}
	return strings.Join(lines, "\n")
}

func (d *DetailView) rowIsAssistantMessage(row activityRow) bool {
	return row.kind == activityRowMessage &&
		isMessageIndex(d.session, row.messageIndex) &&
		d.session.Messages[row.messageIndex].Role == "assistant"
}

func (d *DetailView) timelineActionDetail(row activityRow) string {
	if row.kind != activityRowActionGroup {
		return ""
	}
	start := d.session.Messages[row.messageIndex]
	end, hasEnd := messageAt(d.session.Messages, row.endMessageIndex)
	return actionLifecycleDetail(start, end, hasEnd)
}

func (d *DetailView) timelineExpandedRowDetails(row activityRow) []string {
	if d.detailLevel != timelineDetailExpanded || d.session == nil {
		return nil
	}
	switch row.kind {
	case activityRowMessage:
		if !isMessageIndex(d.session, row.messageIndex) {
			return nil
		}
		return d.expandedMessageDetails(d.session.Messages[row.messageIndex])
	case activityRowActionGroup:
		return d.expandedActionDetails(row)
	default:
		return nil
	}
}

func (d *DetailView) expandedMessageDetails(msg models.Message) []string {
	details := make([]string, 0, 4)
	meta := []string{"role " + msg.Role}
	if d.showTimestamps && !msg.Timestamp.IsZero() {
		meta = append(meta, "time "+msg.Timestamp.Format("2006-01-02 15:04:05"))
	}
	details = append(details, summarizeActivityContent(strings.Join(meta, " · "), d.width-24))

	if content := summarizeActivityContent(msg.Content, maxInt(32, d.width-24)); content != "" {
		details = append(details, "content "+content)
	}

	if total := tokenUsageTotal(msg.Tokens); total > 0 {
		details = append(details, fmt.Sprintf("tokens total:%d input:%d output:%d cache-read:%d cache-write:%d",
			total,
			msg.Tokens.InputTokens,
			msg.Tokens.OutputTokens,
			msg.Tokens.CacheReads,
			msg.Tokens.CacheWrites,
		))
	}

	if meta := expandedActivityMeta(msg.Meta); meta != "" {
		details = append(details, summarizeActivityContent(meta, d.width-24))
	}
	return details
}

func (d *DetailView) expandedActionDetails(row activityRow) []string {
	start := d.session.Messages[row.messageIndex]
	end, hasEnd := messageAt(d.session.Messages, row.endMessageIndex)
	state := groupedActionState(start, end, hasEnd)
	details := make([]string, 0, 5)

	timing := []string{"state " + lowerStatusLabel(indicatorSpec(lifecycleIndicatorState(state)).Label)}
	if d.showTimestamps && !start.Timestamp.IsZero() {
		timing = append(timing, "started "+start.Timestamp.Format("2006-01-02 15:04:05"))
	}
	if d.showTimestamps && hasEnd && !end.Timestamp.IsZero() {
		timing = append(timing, "ended "+end.Timestamp.Format("2006-01-02 15:04:05"))
	}
	if duration := actionLifecycleDuration(start, end, hasEnd, time.Now()); duration != "" {
		timing = append(timing, "duration "+duration)
	}
	details = append(details, summarizeActivityContent(strings.Join(timing, " · "), d.width-24))

	if meta := expandedActivityMeta(start.Meta); meta != "" {
		details = append(details, summarizeActivityContent(meta, d.width-24))
	}
	if hasEnd {
		if meta := expandedActivityMeta(end.Meta); meta != "" && meta != expandedActivityMeta(start.Meta) {
			details = append(details, summarizeActivityContent("completion "+meta, d.width-24))
		}
	}
	if content := summarizeActivityContent(start.Content, maxInt(32, d.width-24)); content != "" {
		details = append(details, "start "+content)
	}
	if hasEnd {
		if content := summarizeActivityContent(end.Content, maxInt(32, d.width-24)); content != "" {
			details = append(details, "end "+content)
		}
	}
	return details
}

func expandedActivityMeta(meta models.ActivityMeta) string {
	parts := make([]string, 0, 6)
	if meta.Kind != "" {
		parts = append(parts, "kind "+meta.Kind)
	}
	if meta.Lifecycle != "" {
		parts = append(parts, "lifecycle "+meta.Lifecycle)
	}
	if meta.Label != "" {
		parts = append(parts, "label "+meta.Label)
	}
	if meta.ID != "" {
		parts = append(parts, "id "+meta.ID)
	}
	if meta.ParentID != "" {
		parts = append(parts, "parent "+meta.ParentID)
	}
	if meta.InteractionID != "" {
		parts = append(parts, "interaction "+meta.InteractionID)
	}
	if meta.EventID != "" {
		parts = append(parts, "event "+meta.EventID)
	}
	if meta.EventParentID != "" {
		parts = append(parts, "event-parent "+meta.EventParentID)
	}
	if meta.RawParentID != "" && meta.RawParentID != meta.EventParentID {
		parts = append(parts, "raw-parent "+meta.RawParentID)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ")
}

func (d *DetailView) recordTimelineTreeOffsets(node *timelineTreeNode, line int) {
	if node == nil {
		return
	}
	if node.rowIndex >= 0 {
		d.rowLineOffsets[node.rowIndex] = line
	}
	childLine := line + 1
	for _, child := range node.children {
		d.recordTimelineTreeOffsets(child, childLine)
		childLine += timelineTreeLineCount(child)
	}
}

func timelineTreeLineCount(node *timelineTreeNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.children {
		count += timelineTreeLineCount(child)
	}
	return count
}

func renderTimelineTree(node *timelineTreeNode) string {
	if node == nil {
		return ""
	}
	return timelineLipglossTree(node).
		Enumerator(timelineTreeEnumerator).
		Indenter(timelineTreeIndenter).
		EnumeratorStyle(lipgloss.NewStyle().Foreground(colorSubtle).MarginRight(1)).
		RootStyle(lipgloss.NewStyle().Foreground(colorCopilot).Bold(true)).
		ItemStyle(lipgloss.NewStyle().Foreground(colorText)).
		String()
}

func timelineTreeEnumerator(children tree.Children, index int) string {
	if children.Length()-1 == index {
		return "╰─"
	}
	return "├─"
}

func timelineTreeIndenter(children tree.Children, index int) string {
	if children.Length()-1 == index {
		return "  "
	}
	return "│ "
}

func timelineLipglossTree(node *timelineTreeNode) *tree.Tree {
	t := tree.Root(node.label)
	for _, child := range node.children {
		t.Child(timelineLipglossTree(child))
	}
	return t
}

func (d *DetailView) renderMessageRow(rowIndex int, row activityRow) string {
	msg := d.session.Messages[row.messageIndex]
	line := d.renderMessageTreeLabel(rowIndex, row)
	prefix := threadPrefix(row, msg.Role)
	if msg.Role == "user" {
		return styleUserMsg.Render(prefix + " " + line)
	}
	return prefix + " " + line
}

func (d *DetailView) renderMessageTreeLabel(rowIndex int, row activityRow) string {
	msg := d.session.Messages[row.messageIndex]
	role := d.timelineMessageRoleLabel(rowIndex, row)
	if msg.Role == "user" && relatedAssistantCount(d.session.Messages, row.messageIndex) > 0 {
		if d.collapsedThreads[row.messageIndex] {
			role = styleUserMsg.Render("[+] PROMPT")
		} else {
			role = styleUserMsg.Render("[-] PROMPT")
		}
	}
	summaryWidth := d.messageSummaryWidth()
	summary := summarizeActivityContent(msg.Content, summaryWidth)
	if summary == "" {
		summary = "(empty)"
	}
	tokenBadge := timelineTokenBadge(msg.Tokens)
	if msg.Role == "user" {
		tokenBadge = ""
	} else if tokenBadge != "" && d.detailLevel != timelineDetailCompact {
		tokenBadge = "  " + tokenBadge
	}
	fields := make([]string, 0, 6)
	if d.showTimestamps {
		fields = append(fields, styleMuted.Render(d.messageTimestamp(msg)))
	}
	if msg.Role == "user" {
		fields = append(fields, fmt.Sprintf("%-10s", d.threadTokenBadge(row.messageIndex)))
	}
	fields = append(fields,
		fmt.Sprintf("%-14s", role),
		styleMuted.Render("│"),
		styleMessageContent.Render(summary)+tokenBadge,
	)
	line := strings.Join(fields, " ")
	if msg.Role == "user" {
		return styleUserMsg.Render(line)
	}
	return line
}

func (d *DetailView) messageTimestamp(msg models.Message) string {
	if msg.Timestamp.IsZero() {
		return "--:--:--"
	}
	return msg.Timestamp.Format("15:04:05")
}

func (d *DetailView) messageSummaryWidth() int {
	timestampWidth := 0
	if d.showTimestamps {
		timestampWidth = 9
	}
	switch d.detailLevel {
	case timelineDetailCompact:
		return maxInt(18, d.width-63-timestampWidth)
	case timelineDetailExpanded:
		return maxInt(32, d.width-27-timestampWidth)
	default:
		return maxInt(24, d.width-37-timestampWidth)
	}
}

func (d *DetailView) renderActionGroupRow(row activityRow) string {
	firstLine := d.renderActionGroupTreeLabel(row)
	detail := d.timelineActionDetail(row)
	if detail == "" || d.detailLevel == timelineDetailCompact {
		return actionThreadPrefix(row) + " " + firstLine
	}
	detailStyle := styleMuted
	start := d.session.Messages[row.messageIndex]
	end, hasEnd := messageAt(d.session.Messages, row.endMessageIndex)
	if lifecycleIndicatorState(groupedActionState(start, end, hasEnd)) == statusFailed {
		detailStyle = styleError
	}
	return actionThreadPrefix(row) + " " + firstLine + "\n" + detailStyle.Render("│     ╰─ "+summarizeActivityContent(detail, d.width-24))
}

func (d *DetailView) renderActionGroupTreeLabel(row activityRow) string {
	start := d.session.Messages[row.messageIndex]
	end, hasEnd := messageAt(d.session.Messages, row.endMessageIndex)
	state := groupedActionState(start, end, hasEnd)
	icon := d.actionStateIcon(state)
	role := d.timelineRoleLabel(start.Role)
	duration := actionLifecycleDuration(start, end, hasEnd, time.Now())

	summaryWidth := d.actionSummaryWidth(duration)
	if summaryWidth < 24 {
		summaryWidth = 24
	}
	summary := summarizeActivityContent(actionLifecycleSummary(start, end, hasEnd), summaryWidth)

	firstLine := fmt.Sprintf(
		"%s %s%-14s %s %s",
		actionStateStyle(state).Render(icon),
		d.actionTimestampColumn(start),
		role,
		styleMuted.Render("│"),
		styleMessageContent.Render(summary),
	)
	firstLine = appendRightAligned(firstLine, styleMuted.Render(duration), d.width-2)
	return firstLine
}

func (d *DetailView) actionTimestampColumn(start models.Message) string {
	if !d.showTimestamps {
		return ""
	}
	return fmt.Sprintf("%-8s ", styleMuted.Render(actionLifecycleStartTime(start)))
}

func (d *DetailView) actionSummaryWidth(duration string) int {
	timestampWidth := 0
	if d.showTimestamps {
		timestampWidth = 9
	}
	switch d.detailLevel {
	case timelineDetailCompact:
		return maxInt(18, d.width-51-timestampWidth-lipgloss.Width(duration))
	case timelineDetailExpanded:
		return maxInt(32, d.width-19-timestampWidth-lipgloss.Width(duration))
	default:
		return maxInt(24, d.width-26-timestampWidth-lipgloss.Width(duration))
	}
}

func (d *DetailView) renderCollapsedRow(row activityRow) string {
	return threadPrefix(row, "assistant") + " " + d.renderCollapsedTreeLabel(row)
}

func (d *DetailView) renderCollapsedTreeLabel(row activityRow) string {
	return fmt.Sprintf(
		"%s %d activity entries folded %s",
		styleMuted.Render("╰─◇"),
		row.collapsedCount,
		styleMuted.Render("(space to expand)"),
	)
}

func (d *DetailView) selectedRowContextLabel() string {
	row, ok := d.selectedActivityRow()
	if !ok {
		return ""
	}
	return d.rowContextLabel(row)
}

func (d *DetailView) rowContextLabel(row activityRow) string {
	if d.session == nil {
		return ""
	}
	switch row.kind {
	case activityRowCollapsed:
		return "enter opens folded prompt"
	case activityRowActionGroup:
		start, ok := messageAt(d.session.Messages, row.messageIndex)
		if !ok {
			return ""
		}
		end, hasEnd := messageAt(d.session.Messages, row.endMessageIndex)
		state := lifecycleIndicatorState(groupedActionState(start, end, hasEnd))
		return "enter opens " + lowerStatusLabel(indicatorSpec(state).Label) + " activity"
	default:
		msg, ok := messageAt(d.session.Messages, row.messageIndex)
		if !ok {
			return ""
		}
		if msg.Role == "user" {
			return "enter opens prompt thread"
		}
		if msg.Meta.Kind != "" {
			return activityKindLabel(msg) + " details"
		}
		return "enter opens activity"
	}
}

func threadPrefix(row activityRow, role string) string {
	if role == "user" {
		return styleUserMsg.Render("╭─")
	}
	if row.threadStart >= 0 {
		if role == "assistant" {
			return styleMuted.Render("│  ") + styleAssistantMsg.Render("├─")
		}
		return styleMuted.Render("│  │")
	}
	return styleMuted.Render("  ")
}

func actionThreadPrefix(row activityRow) string {
	if row.threadStart >= 0 {
		return styleMuted.Render("│  │")
	}
	return styleMuted.Render("├─")
}

func detailCacheTokens(tokens models.TokenUsage) string {
	switch {
	case tokens.CacheReads > 0 && tokens.CacheWrites > 0:
		return fmt.Sprintf("%d/%d", tokens.CacheReads, tokens.CacheWrites)
	case tokens.CacheReads > 0:
		return fmt.Sprintf("%d", tokens.CacheReads)
	case tokens.CacheWrites > 0:
		return fmt.Sprintf("w:%d", tokens.CacheWrites)
	default:
		return "0"
	}
}

func (d *DetailView) renderFocusedEventMeta(index int) string {
	msg, ok := messageAt(d.session.Messages, index)
	if !ok {
		return ""
	}
	parts := []string{
		"row " + fmt.Sprintf("%03d", index+1),
		"role " + msg.Role,
	}
	if !msg.Timestamp.IsZero() {
		parts = append(parts, "time "+msg.Timestamp.Format("2006-01-02 15:04:05"))
	}
	if label := activityKindLabel(msg); label != "" {
		parts = append(parts, label)
	}
	if msg.Meta.ID != "" {
		parts = append(parts, "id "+msg.Meta.ID)
	}
	if msg.Meta.ParentID != "" {
		parts = append(parts, "parent "+msg.Meta.ParentID)
	}
	if msg.Meta.InteractionID != "" {
		parts = append(parts, "interaction "+msg.Meta.InteractionID)
	}
	if msg.Meta.EventID != "" {
		parts = append(parts, "event "+msg.Meta.EventID)
	}
	if msg.Meta.EventParentID != "" {
		parts = append(parts, "event-parent "+msg.Meta.EventParentID)
	}
	if msg.Meta.RawParentID != "" && msg.Meta.RawParentID != msg.Meta.EventParentID {
		parts = append(parts, "raw-parent "+msg.Meta.RawParentID)
	}
	tokens := fmt.Sprintf(
		"tokens in:%d out:%d cache-read:%d cache-write:%d",
		msg.Tokens.InputTokens,
		msg.Tokens.OutputTokens,
		msg.Tokens.CacheReads,
		msg.Tokens.CacheWrites,
	)
	return styleMuted.Render(strings.Join(parts, "  · ")) + "\n" + styleMuted.Render(tokens) + "\n\n"
}

func activityKindLabel(msg models.Message) string {
	if msg.Meta.Kind == "" {
		return ""
	}
	parts := []string{msg.Meta.Kind}
	if msg.Meta.Lifecycle != "" {
		parts = append(parts, msg.Meta.Lifecycle)
	}
	if msg.Meta.Label != "" {
		parts = append(parts, msg.Meta.Label)
	}
	return strings.Join(parts, " ")
}

func (d *DetailView) sessionHeader(s *models.Session) string {
	projectWidth := d.width - 34
	if projectWidth < 12 {
		projectWidth = 12
	}
	title := lipgloss.JoinHorizontal(lipgloss.Top,
		agentBadge(string(s.AgentType)),
		styleMuted.Render("  ·  "),
		styleText(truncateEnd(s.ProjectPath, projectWidth)),
	)

	startStr := "unknown"
	if !s.StartTime.IsZero() {
		startStr = s.StartTime.Format("2006-01-02 15:04:05")
	}

	summaryMetrics := detailSummaryMetrics(s, d.width)

	statusLine := lipgloss.JoinHorizontal(lipgloss.Top,
		detailStatusLine(s, d.follow),
		styleMuted.Render("  ·  "),
		styleMuted.Render(fmt.Sprintf("started %s", startStr)),
		styleMuted.Render("  ·  "),
		styleMuted.Render("duration "+models.FormatDuration(s.Duration())),
	)

	return title + "\n" +
		statusLine + "\n" +
		summaryMetrics + "\n" +
		thinDivider(d.width-6)
}

func detailSummaryMetrics(s *models.Session, width int) string {
	metrics := []string{
		metricChip("Messages", fmt.Sprintf("%d", len(s.Messages)), "☷", styleAccent),
		metricChip("Input", detailInputTokens(s), "↘", styleAccent),
		metricChip("Output", fmt.Sprintf("%d", s.TotalOutputTokens()), "↗", styleAccent),
		metricChip("Cache", detailCacheTokens(s.TotalTokens), "◌", styleAccent),
	}
	if width < 72 {
		first := lipgloss.JoinHorizontal(lipgloss.Top, metrics[0], " ", metrics[1])
		second := lipgloss.JoinHorizontal(lipgloss.Top, metrics[2], " ", metrics[3])
		return first + "\n" + second
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, metrics[0], " ", metrics[1], " ", metrics[2], " ", metrics[3])
}

func (d *DetailView) threadHeader(start, messageCount int, threadTokens models.TokenUsage) string {
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		agentBadge(string(d.session.AgentType)),
		styleMuted.Render("  ·  "),
		styleAccent.Render(fmt.Sprintf("Prompt detail %d", start+1)),
	)

	tokenPanel := lipgloss.JoinHorizontal(lipgloss.Top,
		metricChip("Messages", fmt.Sprintf("%d", messageCount), "☷", styleAccent),
		" ",
		metricChip("Input", fmt.Sprintf("%d", threadTokens.InputTokens), "↘", styleAccent),
		" ",
		metricChip("Output", fmt.Sprintf("%d", threadTokens.OutputTokens), "↗", styleAccent),
		" ",
		metricChip("Cache", detailCacheTokens(threadTokens), "◌", styleAccent),
	)

	return header + "\n" +
		styleMuted.Render(d.session.ProjectPath) + "\n" +
		tokenPanel + "\n" +
		thinDivider(d.width-6)
}

func (d *DetailView) eventHeader(index int, msg models.Message) string {
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		agentBadge(string(d.session.AgentType)),
		styleMuted.Render("  ·  "),
		styleAccent.Render(fmt.Sprintf("Focused activity %d", index+1)),
	)
	tokenPanel := lipgloss.JoinHorizontal(lipgloss.Top,
		metricChip("Role", msg.Role, "●", styleAccent),
		" ",
		d.focusedStatusChip(index, msg),
		" ",
		metricChip("Input", fmt.Sprintf("%d", msg.Tokens.InputTokens), "↘", styleAccent),
		" ",
		metricChip("Output", fmt.Sprintf("%d", msg.Tokens.OutputTokens), "↗", styleAccent),
		" ",
		metricChip("Cache", detailCacheTokens(msg.Tokens), "◌", styleAccent),
	)
	return header + "\n" +
		styleMuted.Render(d.session.ProjectPath) + "\n" +
		tokenPanel + "\n" +
		thinDivider(d.width-6)
}

func detailStatusLine(session *models.Session, follow bool) string {
	parts := []string{statusChip(sessionStatus(session))}
	if session != nil && session.IsActive {
		if follow {
			parts = append(parts, statusChip(statusFollow))
		} else {
			parts = append(parts, statusChip(statusPaused))
		}
	}
	if session != nil {
		if summary := activityStatusSummaryChips(activityStatusCounts(session.Messages)); summary != "" {
			parts = append(parts, summary)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, joinWithSpaces(parts)...)
}

func (d *DetailView) focusedStatusChip(index int, msg models.Message) string {
	if !isActionStart(msg) {
		spec := indicatorSpec(sessionStatus(d.session))
		return metricChip("Status", spec.Label, spec.Icon, spec.Style)
	}
	end, hasEnd := messageAt(d.session.Messages, -1)
	if endIndex, ok := matchingActionEnd(d.session.Messages, index); ok {
		end, hasEnd = d.session.Messages[endIndex], true
	}
	state := lifecycleIndicatorState(groupedActionState(msg, end, hasEnd))
	spec := indicatorSpec(state)
	return metricChip("Status", spec.Label, spec.Icon, spec.Style)
}

func (d *DetailView) setContent(header, content string) {
	d.header = header
	d.viewport.Height = d.viewportHeight()
	d.viewport.SetContent(content)
}

func (d *DetailView) viewportHeight() int {
	height := d.height - 8 - lipgloss.Height(d.header)
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

var timelineSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (d *DetailView) spinnerFrame() string {
	if len(timelineSpinnerFrames) == 0 {
		return "⠋"
	}
	return timelineSpinnerFrames[d.animationFrame%len(timelineSpinnerFrames)]
}

func (d *DetailView) timelineRoleLabel(role string) string {
	switch role {
	case "user":
		return styleUserMsg.Render("✦ PROMPT")
	case "assistant":
		return styleAssistantMsg.Render(d.spinnerFrame() + " ASSIST")
	case "tool":
		return styleToolMsg.Render("⚙ TOOL")
	case "subagent":
		return styleInfo.Render("✧ AGENT")
	case "session":
		return styleMuted.Render("⬡ SESSION")
	case "error":
		return styleError.Render("⚠ ERROR")
	case "system":
		return styleMuted.Render("◍ SYSTEM")
	default:
		return styleMuted.Render("• " + strings.ToUpper(role))
	}
}

func isActionStart(msg models.Message) bool {
	return msg.Meta.Kind != "" &&
		(msg.Meta.Lifecycle == models.ActivityLifecycleStarted ||
			msg.Meta.Lifecycle == models.ActivityLifecycleRequested)
}

func isActionEnd(msg models.Message) bool {
	return msg.Meta.Kind != "" &&
		(msg.Meta.Lifecycle == models.ActivityLifecycleCompleted ||
			msg.Meta.Lifecycle == models.ActivityLifecycleFailed)
}

func matchingActionEnd(messages []models.Message, start int) (int, bool) {
	if start < 0 || start >= len(messages) || !isActionStart(messages[start]) || messages[start].Meta.ID == "" {
		return 0, false
	}
	id := messages[start].Meta.ID
	kind := messages[start].Meta.Kind
	for i := start + 1; i < len(messages); i++ {
		if messages[i].Meta.ID == id && messages[i].Meta.Kind == kind && isActionEnd(messages[i]) {
			return i, true
		}
	}
	return 0, false
}

func messageAt(messages []models.Message, index int) (models.Message, bool) {
	if index < 0 || index >= len(messages) {
		return models.Message{}, false
	}
	return messages[index], true
}

func groupedActionState(start, end models.Message, hasEnd bool) string {
	if hasEnd {
		return end.Meta.Lifecycle
	}
	if start.Meta.Lifecycle == models.ActivityLifecycleRequested {
		return models.ActivityLifecycleRequested
	}
	return "in_progress"
}

func (d *DetailView) actionStateIcon(state string) string {
	switch lifecycleIndicatorState(state) {
	case statusRequested:
		return "◇"
	case statusRunning:
		return d.spinnerFrame()
	case statusDone:
		return "✔"
	case statusFailed:
		return "✖"
	default:
		return indicatorSpec(lifecycleIndicatorState(state)).Icon
	}
}

func (d *DetailView) timelineMessageRoleLabel(rowIndex int, row activityRow) string {
	msg := d.session.Messages[row.messageIndex]
	if msg.Role == "assistant" {
		return styleAssistantMsg.Render(d.assistantStateIcon(rowIndex) + " ASSIST")
	}
	return d.timelineRoleLabel(msg.Role)
}

func (d *DetailView) assistantStateIcon(rowIndex int) string {
	switch d.assistantRowState(rowIndex) {
	case statusRequested:
		return "◇"
	case statusRunning:
		return d.spinnerFrame()
	case statusFailed:
		return "✖"
	default:
		return "✔"
	}
}

func (d *DetailView) assistantRowState(rowIndex int) string {
	if d.session == nil || rowIndex < 0 || rowIndex >= len(d.rows) {
		return statusDone
	}
	row := d.rows[rowIndex]
	msg, ok := messageAt(d.session.Messages, row.messageIndex)
	if !ok {
		return statusDone
	}
	if msg.Meta.Lifecycle != "" {
		return lifecycleIndicatorState(msg.Meta.Lifecycle)
	}

	seenRequested := false
	for i := rowIndex + 1; i < len(d.rows); i++ {
		child := d.rows[i]
		if rowStartsUserContainer(d.session, child) || d.rowIsAssistantMessage(child) {
			break
		}
		if child.kind != activityRowActionGroup {
			continue
		}
		state := d.actionGroupIndicatorState(child)
		switch state {
		case statusFailed:
			return statusFailed
		case statusRunning:
			return statusRunning
		case statusRequested:
			seenRequested = true
		}
	}
	if seenRequested {
		return statusRequested
	}
	if strings.TrimSpace(msg.Content) == "" && d.session.IsActive && rowIndex == len(d.rows)-1 {
		return statusRunning
	}
	return statusDone
}

func (d *DetailView) actionGroupIndicatorState(row activityRow) string {
	start := d.session.Messages[row.messageIndex]
	end, hasEnd := messageAt(d.session.Messages, row.endMessageIndex)
	return lifecycleIndicatorState(groupedActionState(start, end, hasEnd))
}

func actionStateStyle(state string) lipgloss.Style {
	return indicatorSpec(lifecycleIndicatorState(state)).Style
}

func actionLifecycleSummary(start, end models.Message, hasEnd bool) string {
	label := actionLifecycleLabel(start)
	state := groupedActionState(start, end, hasEnd)
	stateText := lowerStatusLabel(indicatorSpec(lifecycleIndicatorState(state)).Label)

	parts := []string{label}
	if start.Meta.Kind == models.ActivityKindTool {
		if intent := toolIntentSummary(start.Content); intent != "" {
			parts = append(parts, intent)
		}
	}
	parts = append(parts, stateText)
	return strings.Join(parts, " · ")
}

func actionLifecycleLabel(start models.Message) string {
	if start.Meta.Label != "" {
		return start.Meta.Label
	}
	content := summarizeActivityContent(start.Content, 48)
	if content == "" {
		return "action"
	}
	for _, prefix := range []string{"Started tool: ", "User requested tool: ", "Started subagent: "} {
		if strings.HasPrefix(content, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(content, prefix))
		}
	}
	return content
}

func actionLifecycleStartTime(start models.Message) string {
	if !start.Timestamp.IsZero() {
		return start.Timestamp.Format("15:04:05")
	}
	return "--:--:--"
}

func actionLifecycleDuration(start, end models.Message, hasEnd bool, now time.Time) string {
	if start.Timestamp.IsZero() {
		return ""
	}
	if !hasEnd {
		if now.IsZero() || now.Before(start.Timestamp) {
			return ""
		}
		return models.FormatDuration(now.Sub(start.Timestamp))
	}
	if end.Timestamp.IsZero() || end.Timestamp.Before(start.Timestamp) {
		return ""
	}
	return models.FormatDuration(end.Timestamp.Sub(start.Timestamp))
}

func appendRightAligned(line, suffix string, width int) string {
	if suffix == "" {
		return line
	}
	padding := width - lipgloss.Width(line) - lipgloss.Width(suffix)
	if padding < 1 {
		padding = 1
	}
	return line + strings.Repeat(" ", padding) + suffix
}

func actionLifecycleDetail(start, end models.Message, hasEnd bool) string {
	if start.Meta.Kind == models.ActivityKindTool {
		if hasEnd {
			if detail := lifecycleErrorDetail(end.Content); detail != "" {
				return detail
			}
		}
		if detail := toolArgumentDetail(start.Content); detail != "" {
			return detail
		}
		if toolIntentSummary(start.Content) != "" {
			return ""
		}
		if !hasEnd {
			return usefulLifecycleDetail(start.Content)
		}
	}
	if hasEnd {
		if detail := usefulLifecycleDetail(end.Content); detail != "" {
			return detail
		}
	}
	return usefulLifecycleDetail(start.Content)
}

func lifecycleErrorDetail(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "error:") {
			return strings.TrimSpace(line[len("error:"):])
		}
	}
	return ""
}

func toolIntentSummary(content string) string {
	for _, line := range usefulLifecycleLines(content) {
		if isToolIntentLine(line) {
			return line
		}
	}
	return ""
}

func toolArgumentDetail(content string) string {
	lines := make([]string, 0, 4)
	for _, line := range usefulLifecycleLines(content) {
		if isToolArgumentLine(line) {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, " · ")
}

func usefulLifecycleDetail(content string) string {
	for _, line := range usefulLifecycleLines(content) {
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "error:") {
			return strings.TrimSpace(line[len("error:"):])
		}
		if strings.HasPrefix(lower, "telemetry:") ||
			strings.HasPrefix(lower, "parent:") ||
			strings.HasPrefix(lower, "reasoning:") ||
			strings.HasPrefix(lower, "intent:") ||
			strings.HasPrefix(lower, "goal:") ||
			strings.HasPrefix(lower, "description:") ||
			strings.HasPrefix(lower, "explanation:") ||
			strings.HasPrefix(lower, "summary:") ||
			strings.HasPrefix(lower, "query:") ||
			strings.HasPrefix(lower, "pattern:") ||
			strings.HasPrefix(lower, "prompt:") ||
			strings.HasPrefix(lower, "input:") ||
			strings.HasPrefix(lower, "command:") ||
			strings.HasPrefix(lower, "filepath:") ||
			strings.HasPrefix(lower, "path:") ||
			strings.HasPrefix(lower, "result:") ||
			strings.HasPrefix(lower, "output:") {
			return line
		}
	}
	return ""
}

func usefulLifecycleLines(content string) []string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || isLifecycleSummaryLine(line) {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func isToolIntentLine(line string) bool {
	lower := strings.ToLower(line)
	for _, prefix := range []string{
		"reasoning:",
		"intent:",
		"goal:",
		"description:",
		"explanation:",
		"summary:",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func isToolArgumentLine(line string) bool {
	lower := strings.ToLower(line)
	if isToolIntentLine(line) || strings.HasPrefix(lower, "parent:") {
		return false
	}
	for _, prefix := range []string{
		"arguments:",
		"input:",
		"command:",
		"filepath:",
		"path:",
		"query:",
		"pattern:",
		"prompt:",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func isLifecycleSummaryLine(line string) bool {
	lower := strings.ToLower(line)
	for _, prefix := range []string{
		"started tool:",
		"user requested tool:",
		"tool completed:",
		"tool failed:",
		"started subagent:",
		"completed subagent:",
		"failed subagent:",
		"model:",
		"duration:",
		"tokens:",
		"tool calls:",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
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

type tokenLoadSpec struct {
	Level string
	Icon  string
	Style lipgloss.Style
}

func (d *DetailView) threadTokenBadge(userIndex int) string {
	if d.session == nil {
		return ""
	}
	_, _, ok := selectedThreadRange(d.session.Messages, userIndex)
	if !ok {
		return ""
	}
	tokens := threadTokenUsage(d.session.Messages, userIndex)
	total := tokenUsageTotal(tokens)
	if total == 0 {
		return ""
	}
	spec := tokenLoadIndicator(total)
	return spec.Style.Render(fmt.Sprintf("%s %s", spec.Icon, compactInt(total)))
}

func threadTokenUsage(messages []models.Message, userIndex int) models.TokenUsage {
	start, end, ok := selectedThreadRange(messages, userIndex)
	if !ok {
		return models.TokenUsage{}
	}
	var tokens models.TokenUsage
	for _, msg := range messages[start:end] {
		tokens.InputTokens += msg.Tokens.InputTokens
		tokens.OutputTokens += msg.Tokens.OutputTokens
		tokens.CacheReads += msg.Tokens.CacheReads
		tokens.CacheWrites += msg.Tokens.CacheWrites
	}
	return tokens
}

func tokenUsageTotal(tokens models.TokenUsage) int {
	return tokens.InputTokens + tokens.OutputTokens + tokens.CacheReads + tokens.CacheWrites
}

func tokenLoadIndicator(total int) tokenLoadSpec {
	switch {
	case total >= 60_000:
		return tokenLoadSpec{"critical", "⚠", styleError}
	case total >= 30_000:
		return tokenLoadSpec{"elevated", "◆", lipgloss.NewStyle().Foreground(colorSecondary).Bold(true)}
	case total >= 10_000:
		return tokenLoadSpec{"moderate", "◐", styleWarning}
	default:
		return tokenLoadSpec{"optimal", "◌", styleSuccess}
	}
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
	groups := actionGroups(messages)
	for i := 0; i < len(messages); i++ {
		if group, ok := groups.byEnd[i]; ok && group.endMessageIndex == i {
			continue
		}

		if group, ok := groups.byStart[i]; ok {
			rows = append(rows, group)
		} else {
			rows = append(rows, activityRow{kind: activityRowMessage, messageIndex: i, threadStart: nearestThreadStart(messages, i)})
		}

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

func rowStartsUserContainer(session *models.Session, row activityRow) bool {
	if session == nil || row.kind != activityRowMessage || !isMessageIndex(session, row.messageIndex) {
		return false
	}
	return session.Messages[row.messageIndex].Role == "user"
}

type actionGroupIndex struct {
	byStart map[int]activityRow
	byEnd   map[int]activityRow
}

type actionGroupKey struct {
	kind string
	id   string
}

func actionGroups(messages []models.Message) actionGroupIndex {
	rootByID := make(map[actionGroupKey]int)
	for i, msg := range messages {
		if !isActionStart(msg) || msg.Meta.ID == "" {
			continue
		}
		key := actionGroupKey{kind: msg.Meta.Kind, id: msg.Meta.ID}
		existing, seen := rootByID[key]
		if !seen || preferActionRoot(msg, messages[existing]) {
			rootByID[key] = i
		}
	}

	groups := actionGroupIndex{
		byStart: make(map[int]activityRow),
		byEnd:   make(map[int]activityRow),
	}
	for key, start := range rootByID {
		row := activityRow{
			kind:            activityRowActionGroup,
			messageIndex:    start,
			endMessageIndex: -1,
			threadStart:     nearestThreadStart(messages, start),
		}
		for i := start + 1; i < len(messages); i++ {
			if messages[i].Meta.ID == key.id && messages[i].Meta.Kind == key.kind && isActionEnd(messages[i]) {
				row.endMessageIndex = i
				break
			}
		}
		groups.byStart[start] = row
		if row.endMessageIndex >= 0 {
			groups.byEnd[row.endMessageIndex] = row
		}
	}
	return groups
}

func preferActionRoot(candidate, existing models.Message) bool {
	return candidate.Meta.Lifecycle == models.ActivityLifecycleStarted &&
		existing.Meta.Lifecycle == models.ActivityLifecycleRequested
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
		if row.kind != activityRowCollapsed &&
			(row.messageIndex == messageIndex || row.endMessageIndex == messageIndex) {
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
	d.selectedUser = row.threadStart
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
		d.userPausedFollow = true
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

	return d.header + "\n" + d.viewport.View()
}

func (d *DetailView) ThreadView() string {
	if d.session == nil {
		return d.View()
	}

	return d.header + "\n" + d.viewport.View()
}

func (d *DetailView) FooterStatus() string {
	if d == nil || d.session == nil {
		return ""
	}
	rowLabel := "0/0"
	if len(d.rows) > 0 && d.selectedRow >= 0 {
		rowLabel = fmt.Sprintf("%d/%d", d.selectedRow+1, len(d.rows))
	}
	followLabel := "paused"
	if d.follow {
		followLabel = "live"
	}
	parts := []string{
		fmt.Sprintf("%d%%", int(d.viewport.ScrollPercent()*100)),
		"row " + rowLabel,
		"detail " + d.timelineDetailLabel(),
		d.timestampLabel(),
		followLabel,
	}
	if context := d.selectedRowContextLabel(); context != "" {
		parts = append(parts, context)
	}
	return strings.Join(parts, "  ")
}

func (d *DetailView) FocusedFooterStatus() string {
	if d == nil || d.session == nil {
		return ""
	}
	label := "focused activity"
	if d.focusedMode == focusThread {
		label = "prompt detail"
	}
	return fmt.Sprintf("%d%%  %s", int(d.viewport.ScrollPercent()*100), label)
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
