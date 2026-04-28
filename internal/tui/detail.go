package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

type DetailView struct {
	viewport         viewport.Model
	session          *models.Session
	header           string
	collapsedThreads map[int]bool
	selectedUser     int
	userLineOffsets  map[int]int
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
		d.renderContent()
	}
}

func (d *DetailView) SetSession(s *models.Session) {
	if !sameSession(d.session, s) {
		d.collapsedThreads = make(map[int]bool)
		d.selectedUser = firstUserIndex(s)
	} else if !isUserMessage(s, d.selectedUser) {
		d.selectedUser = firstUserIndex(s)
	}
	d.session = s
	d.renderContent()
}

func (d *DetailView) ScrollToBottom() {
	d.viewport.GotoBottom()
}

func (d *DetailView) SelectLastUser() {
	if d.session == nil {
		return
	}
	for i := len(d.session.Messages) - 1; i >= 0; i-- {
		if d.session.Messages[i].Role == "user" {
			d.selectedUser = i
			d.renderContent()
			d.scrollSelectedUserIntoView()
			return
		}
	}
}

func (d *DetailView) SelectPreviousUser() {
	d.selectUser(-1)
}

func (d *DetailView) SelectNextUser() {
	d.selectUser(1)
}

func (d *DetailView) ToggleSelectedThread() {
	if !isUserMessage(d.session, d.selectedUser) {
		return
	}
	if d.collapsedThreads == nil {
		d.collapsedThreads = make(map[int]bool)
	}
	d.collapsedThreads[d.selectedUser] = !d.collapsedThreads[d.selectedUser]
	d.renderContent()
	d.scrollSelectedUserIntoView()
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
	d.renderContent()
	d.scrollSelectedUserIntoView()
}

func (d *DetailView) OpenSelectedThread() bool {
	if !isUserMessage(d.session, d.selectedUser) {
		return false
	}
	d.renderThreadContent()
	d.viewport.GotoTop()
	return true
}

func (d *DetailView) RefreshSelectedThread() bool {
	if !isUserMessage(d.session, d.selectedUser) {
		return false
	}
	d.renderThreadContent()
	return true
}

func (d *DetailView) ShowSessionDetail() {
	d.renderContent()
	d.scrollSelectedUserIntoView()
}

func (d *DetailView) renderContent() {
	if d.session == nil {
		d.header = ""
		d.viewport.SetContent(styleMuted.Render("No session selected"))
		return
	}

	s := d.session
	header := d.sessionHeader(s)
	var content strings.Builder
	line := 0
	d.userLineOffsets = make(map[int]int)
	write := func(text string) {
		content.WriteString(text)
		line += strings.Count(text, "\n")
	}

	if len(s.Messages) == 0 {
		write(styleMuted.Render("No messages found in this session.\n"))
	}

	for i := 0; i < len(s.Messages); i++ {
		msg := s.Messages[i]
		if msg.Role == "user" {
			d.userLineOffsets[i] = line
		}
		roleLabel := ""
		switch msg.Role {
		case "user":
			roleLabel = d.userRoleLabel(i)
		case "assistant":
			roleLabel = styleAssistantMsg.Render("◆ Assistant")
		case "tool":
			roleLabel = styleAccent.Render("▣ Tool")
		case "subagent":
			roleLabel = styleAccent.Render("◈ Subagent")
		case "session":
			roleLabel = styleMuted.Render("◇ Session")
		case "error":
			roleLabel = styleError.Render("⚠ Error")
		case "system":
			roleLabel = styleMuted.Render("● System")
		default:
			roleLabel = styleMuted.Render("● " + msg.Role)
		}

		tsStr := ""
		if !msg.Timestamp.IsZero() {
			tsStr = styleMuted.Render("  " + msg.Timestamp.Format("15:04:05"))
		}

		messageHeader := fmt.Sprintf("%s %s%s", styleMuted.Render(fmt.Sprintf("%02d", i+1)), roleLabel, tsStr)
		if msg.Role == "user" && i == d.selectedUser {
			messageHeader = styleSelected.Render(messageHeader)
		}
		write(messageHeader + "\n")

		content := msg.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		if content == "" {
			content = styleMuted.Render("(empty)")
		}

		lines := strings.Split(content, "\n")
		for _, line := range lines {
			write(styleMuted.Render("   │ ") + styleMessageContent.Render(line) + "\n")
		}

		if msg.Tokens.InputTokens > 0 || msg.Tokens.OutputTokens > 0 {
			write(styleMuted.Render(fmt.Sprintf("   ╰─ tokens in:%d out:%d\n", msg.Tokens.InputTokens, msg.Tokens.OutputTokens)))
		}
		if msg.Role == "user" && d.collapsedThreads[i] {
			collapsedCount := relatedAssistantCount(s.Messages, i)
			if collapsedCount > 0 {
				write(styleMuted.Render(fmt.Sprintf("   ╰─ %d assistant messages collapsed (space to expand)\n", collapsedCount)))
				i += collapsedCount
			}
		}
		write("\n")
	}

	d.setContent(header, content.String())
}

func (d *DetailView) renderThreadContent() {
	if d.session == nil {
		d.header = ""
		d.viewport.SetContent(styleMuted.Render("No session selected"))
		return
	}

	start, end, ok := selectedThreadRange(d.session.Messages, d.selectedUser)
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
		statusPill(s.IsActive) + "\n\n" +
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

func (d *DetailView) userRoleLabel(messageIndex int) string {
	label := "▶ User"
	if relatedAssistantCount(d.session.Messages, messageIndex) > 0 {
		if d.collapsedThreads[messageIndex] {
			label = "[+] User"
		} else {
			label = "[-] User"
		}
	}
	if messageIndex == d.selectedUser {
		label = "▸ " + label
	}
	return styleUserMsg.Render(label)
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

func (d *DetailView) selectUser(direction int) {
	if d.session == nil {
		return
	}
	messages := d.session.Messages
	if d.selectedUser < 0 || !isUserMessage(d.session, d.selectedUser) {
		d.selectedUser = firstUserIndex(d.session)
		d.renderContent()
		d.scrollSelectedUserIntoView()
		return
	}
	for i := d.selectedUser + direction; i >= 0 && i < len(messages); i += direction {
		if messages[i].Role == "user" {
			d.selectedUser = i
			d.renderContent()
			d.scrollSelectedUserIntoView()
			return
		}
	}
}

func (d *DetailView) scrollSelectedUserIntoView() {
	line, ok := d.userLineOffsets[d.selectedUser]
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
	d.viewport.ScrollDown(3)
}

func (d *DetailView) ScrollUp() {
	d.viewport.ScrollUp(3)
}

func (d *DetailView) PageDown() {
	d.viewport.HalfViewDown()
}

func (d *DetailView) PageUp() {
	d.viewport.HalfViewUp()
}

func (d *DetailView) AtBottom() bool {
	return d.viewport.AtBottom()
}

func (d *DetailView) View() string {
	if d.session == nil {
		return styleCard.
			Width(d.width - 4).
			Height(d.height - 6).
			Align(lipgloss.Center).
			Render(styleMuted.Render("Select a session from the dashboard to view details.\n\nPress esc to go back."))
	}

	footer := styleMuted.Render(fmt.Sprintf("  %d%%  ↑/↓ user prompt  space toggle  c collapse all  pgup/pgdown scroll  esc back",
		int(d.viewport.ScrollPercent()*100)))

	return d.header + "\n" + d.viewport.View() + "\n" + footer
}

func (d *DetailView) ThreadView() string {
	if d.session == nil {
		return d.View()
	}

	footer := styleMuted.Render(fmt.Sprintf("  %d%%  verbose prompt detail  ↑↓/pgup/pgdown scroll  esc back",
		int(d.viewport.ScrollPercent()*100)))

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
