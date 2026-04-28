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

func (d *DetailView) renderContent() {
	if d.session == nil {
		d.viewport.SetContent(styleMuted.Render("No session selected"))
		return
	}

	s := d.session
	var sb strings.Builder
	line := 0
	d.userLineOffsets = make(map[int]int)
	write := func(text string) {
		sb.WriteString(text)
		line += strings.Count(text, "\n")
	}

	// Header
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		agentBadge(string(s.AgentType)),
		styleMuted.Render("  ·  "),
		styleText(s.ProjectPath),
	)
	write(header + "\n")

	startStr := "unknown"
	if !s.StartTime.IsZero() {
		startStr = s.StartTime.Format("2006-01-02 15:04:05")
	}
	write(styleMuted.Render(fmt.Sprintf("Started: %s  Duration: %s", startStr, models.FormatDuration(s.Duration()))) + "\n")

	write(statusPill(s.IsActive) + "\n\n")

	// Token usage panel
	tokenPanel := lipgloss.JoinHorizontal(lipgloss.Top,
		metricCard("Input Tokens", fmt.Sprintf("%d", s.TotalInputTokens()), "↘", styleAccent),
		"  ",
		metricCard("Output Tokens", fmt.Sprintf("%d", s.TotalOutputTokens()), "↗", styleAccent),
		"  ",
		metricCard("Cache Reads", fmt.Sprintf("%d", s.TotalTokens.CacheReads), "◌", styleAccent),
	)
	write(tokenPanel + "\n\n")
	write(divider(d.width-6) + "\n\n")

	// Messages
	write(styleAccent.Render(fmt.Sprintf("╭─ Messages (%d)", len(s.Messages))) + "\n\n")

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
	write(styleAccent.Render(fmt.Sprintf("╰─ Messages (%d)", len(s.Messages))) + "\n")

	d.viewport.SetContent(sb.String())
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

func (d *DetailView) View() string {
	if d.session == nil {
		return styleCard.
			Width(d.width - 4).
			Height(d.height - 6).
			Align(lipgloss.Center).
			Render(styleMuted.Render("Select a session from the dashboard to view details.\n\nPress esc to go back."))
	}

	footer := styleMuted.Render(fmt.Sprintf("  %d%%  ↑/↓ user prompt  space collapse  pgup/pgdown scroll  esc back",
		int(d.viewport.ScrollPercent()*100)))

	return d.viewport.View() + "\n" + footer
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
