package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

type DetailView struct {
	viewport viewport.Model
	session  *models.Session
	width    int
	height   int
}

func NewDetailView(width, height int) *DetailView {
	vp := viewport.New(width-4, height-10)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1)
	return &DetailView{
		viewport: vp,
		width:    width,
		height:   height,
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
	d.session = s
	d.renderContent()
}

func (d *DetailView) ScrollToBottom() {
	d.viewport.GotoBottom()
}

func (d *DetailView) renderContent() {
	if d.session == nil {
		d.viewport.SetContent(styleMuted.Render("No session selected"))
		return
	}

	s := d.session
	var sb strings.Builder

	// Header
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		agentBadge(string(s.AgentType)),
		styleMuted.Render("  ·  "),
		styleText(s.ProjectPath),
	)
	sb.WriteString(header + "\n")

	startStr := "unknown"
	if !s.StartTime.IsZero() {
		startStr = s.StartTime.Format("2006-01-02 15:04:05")
	}
	sb.WriteString(styleMuted.Render(fmt.Sprintf("Started: %s  Duration: %s", startStr, models.FormatDuration(s.Duration()))) + "\n")

	sb.WriteString(statusPill(s.IsActive) + "\n\n")

	// Token usage panel
	tokenPanel := lipgloss.JoinHorizontal(lipgloss.Top,
		metricCard("Input Tokens", fmt.Sprintf("%d", s.TotalInputTokens()), "↘", styleAccent),
		"  ",
		metricCard("Output Tokens", fmt.Sprintf("%d", s.TotalOutputTokens()), "↗", styleAccent),
		"  ",
		metricCard("Cache Reads", fmt.Sprintf("%d", s.TotalTokens.CacheReads), "◌", styleAccent),
	)
	sb.WriteString(tokenPanel + "\n\n")
	sb.WriteString(divider(d.width-6) + "\n\n")

	// Messages
	sb.WriteString(styleAccent.Render(fmt.Sprintf("╭─ Messages (%d)", len(s.Messages))) + "\n\n")

	if len(s.Messages) == 0 {
		sb.WriteString(styleMuted.Render("No messages found in this session.\n"))
	}

	for i, msg := range s.Messages {
		roleLabel := ""
		switch msg.Role {
		case "user":
			roleLabel = styleUserMsg.Render("▶ User")
		case "assistant":
			roleLabel = styleAssistantMsg.Render("◆ Assistant")
		default:
			roleLabel = styleMuted.Render("● " + msg.Role)
		}

		tsStr := ""
		if !msg.Timestamp.IsZero() {
			tsStr = styleMuted.Render("  " + msg.Timestamp.Format("15:04:05"))
		}

		sb.WriteString(fmt.Sprintf("%s %s%s\n", styleMuted.Render(fmt.Sprintf("%02d", i+1)), roleLabel, tsStr))

		content := msg.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		if content == "" {
			content = styleMuted.Render("(empty)")
		}

		lines := strings.Split(content, "\n")
		for _, line := range lines {
			sb.WriteString(styleMuted.Render("   │ ") + styleMessageContent.Render(line) + "\n")
		}

		if msg.Tokens.InputTokens > 0 || msg.Tokens.OutputTokens > 0 {
			sb.WriteString(styleMuted.Render(fmt.Sprintf("   ╰─ tokens in:%d out:%d\n", msg.Tokens.InputTokens, msg.Tokens.OutputTokens)))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(styleAccent.Render(fmt.Sprintf("╰─ Messages (%d)", len(s.Messages))) + "\n")

	d.viewport.SetContent(sb.String())
}

func styleText(s string) string {
	return lipgloss.NewStyle().Foreground(colorText).Render(s)
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

	footer := styleMuted.Render(fmt.Sprintf("  %d%%  ↑/↓ scroll  pgup/pgdown page  esc back",
		int(d.viewport.ScrollPercent()*100)))

	return d.viewport.View() + "\n" + footer
}
