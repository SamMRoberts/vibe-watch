package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"
)

const (
	spaceXS = 1
	spaceSM = 2
)

var (
	colorBackground  = lipgloss.Color("#06110f")
	colorSurface     = lipgloss.Color("#0d1f1b")
	colorSurfaceAlt  = lipgloss.Color("#15342e")
	colorSurfaceGlow = lipgloss.Color("#1d4941")
	colorPrimary     = lipgloss.Color("#2dd4bf")
	colorSecondary   = lipgloss.Color("#f97316")
	colorAccent      = lipgloss.Color("#e11d48")
	colorGlow        = lipgloss.Color("#f4d35e")
	colorText        = lipgloss.Color("#f7fee7")
	colorMuted       = lipgloss.Color("#9fb8ad")
	colorSubtle      = lipgloss.Color("#41645a")
	colorSuccess     = lipgloss.Color("#84cc16")
	colorWarning     = lipgloss.Color("#facc15")
	colorError       = lipgloss.Color("#fb7185")
	colorInfo        = lipgloss.Color("#38bdf8")
	colorClaude      = lipgloss.Color("#ff9f6e")
	colorCodex       = lipgloss.Color("#38bdf8")
	colorCopilot     = lipgloss.Color("#c084fc")
	colorCopilotChat = lipgloss.Color("#2dd4bf")
	colorAmazonQ     = lipgloss.Color("#facc15")
)

var (
	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorGlow).
			PaddingLeft(spaceXS)

	styleSubtitle = lipgloss.NewStyle().
			Foreground(colorMuted).
			PaddingLeft(spaceXS)

	styleHeader = lipgloss.NewStyle().
			Background(colorSurface).
			Foreground(colorText).
			Bold(true).
			Padding(0, spaceXS)

	styleSelected = lipgloss.NewStyle().
			Background(colorSurfaceGlow).
			Foreground(colorText).
			Bold(true)

	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary)

	styleCard = lipgloss.NewStyle().
			Background(colorSurface).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSubtle).
			Padding(0, spaceXS)

	stylePanel = lipgloss.NewStyle().
			Background(colorSurface).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, spaceSM)

	styleDivider = lipgloss.NewStyle().
			Foreground(colorSubtle)

	styleMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleAccent = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	styleSuccess = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	styleError = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	styleWarning = lipgloss.NewStyle().
			Foreground(colorWarning).
			Bold(true)

	styleInfo = lipgloss.NewStyle().
			Foreground(colorInfo).
			Bold(true)

	styleTab = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, spaceSM).
			MarginRight(spaceXS)

	styleActiveTab = lipgloss.NewStyle().
			Background(colorPrimary).
			Foreground(colorBackground).
			Bold(true).
			Padding(0, spaceSM).
			MarginRight(spaceXS)

	styleHelp = lipgloss.NewStyle().
			Foreground(colorMuted).
			PaddingTop(spaceXS)

	styleUserMsg = lipgloss.NewStyle().
			Foreground(colorCopilot).
			Bold(true)

	styleAssistantMsg = lipgloss.NewStyle().
				Foreground(colorClaude).
				Bold(true)

	styleMessageContent = lipgloss.NewStyle().
				Foreground(colorText)

	styleAgentBadge = map[string]lipgloss.Style{
		"Claude Code":  lipgloss.NewStyle().Foreground(colorClaude).Bold(true),
		"Codex CLI":    lipgloss.NewStyle().Foreground(colorCodex).Bold(true),
		"Copilot CLI":  lipgloss.NewStyle().Foreground(colorCopilot).Bold(true),
		"Copilot Chat": lipgloss.NewStyle().Foreground(colorCopilotChat).Bold(true),
		"Amazon Q":     lipgloss.NewStyle().Foreground(colorAmazonQ).Bold(true),
	}
)

func agentStyle(agent string) lipgloss.Style {
	if s, ok := styleAgentBadge[agent]; ok {
		return s
	}
	return lipgloss.NewStyle().Foreground(colorText)
}

func agentBadge(agent string) string {
	sty := agentStyle(agent).
		Background(colorSurfaceAlt).
		Padding(0, spaceXS)
	return sty.Render(agentIcon(agent) + " " + agent)
}

func compactAgentBadge(agent string, width int) string {
	label := agentIcon(agent) + " " + agent
	if width > 0 && lipgloss.Width(label) > width {
		label = agentIcon(agent) + " " + compactAgentName(agent)
	}
	if width > 0 && lipgloss.Width(label) > width {
		label = agentIcon(agent)
	}
	return agentStyle(agent).Render(label)
}

func compactAgentName(agent string) string {
	switch agent {
	case "Claude Code":
		return "Claude"
	case "Codex CLI":
		return "Codex"
	case "Copilot CLI":
		return "Copilot"
	case "Copilot Chat":
		return "Chat"
	case "Amazon Q":
		return "Q"
	default:
		return agent
	}
}

func agentIcon(agent string) string {
	switch agent {
	case "Claude Code":
		return "✹"
	case "Codex CLI":
		return "◆"
	case "Copilot CLI":
		return "◈"
	case "Copilot Chat":
		return "◇"
	case "Amazon Q":
		return "⬢"
	default:
		return "●"
	}
}

func metricCard(label, value, icon string, valueStyle lipgloss.Style) string {
	return metricCardWidth(label, value, icon, valueStyle, 20)
}

func metricCardWidth(label, value, icon string, valueStyle lipgloss.Style, width int) string {
	if width < 12 {
		width = 12
	}
	return lipgloss.NewStyle().
		Background(colorSurface).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorSubtle).
		Padding(0, spaceXS).
		Width(width).
		Render(
			styleMuted.Render(icon+" "+label) + "\n" +
				valueStyle.Render(value),
		)
}

func metricChip(label, value, icon string, valueStyle lipgloss.Style) string {
	return lipgloss.NewStyle().
		Background(colorSurfaceAlt).
		Foreground(colorText).
		Padding(0, spaceXS).
		Render(styleMuted.Render(icon+" "+label+": ") + valueStyle.Render(value))
}

func semanticPill(label string, bg lipgloss.Color) string {
	return pill(label, bg, colorBackground, true)
}

func quietPill(label string) string {
	return pill(label, colorSurfaceAlt, colorMuted, false)
}

func pill(label string, bg, fg lipgloss.Color, bold bool) string {
	style := lipgloss.NewStyle().
		Background(bg).
		Foreground(fg).
		Padding(0, spaceXS)
	if bold {
		style = style.Bold(true)
	}
	return style.Render(label)
}

func sectionHeader(title, subtitle string, width int) string {
	if width < 10 {
		width = 10
	}
	label := styleAccent.Render("╭─ " + title)
	if subtitle != "" {
		label += styleMuted.Render("  " + subtitle)
	}
	return label + "\n" + divider(width)
}

func observatoryPanel(width int, content string) string {
	return panelWithBorder(width, content, colorPrimary)
}

func quietPanel(width int, content string) string {
	return panelWithBorder(width, content, colorSubtle)
}

func dangerPanel(width int, content string) string {
	return panelWithBorder(width, content, colorError)
}

func panelWithBorder(width int, content string, border lipgloss.Color) string {
	if width < 8 {
		width = 8
	}
	return lipgloss.NewStyle().
		Background(colorSurface).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(1, spaceSM).
		Width(width).
		Render(content)
}

func emptyState(width int, title, body string) string {
	if width < 20 {
		width = 20
	}
	return quietPanel(width, lipgloss.JoinVertical(lipgloss.Center,
		styleAccent.Render("✦ "+title),
		"",
		styleMuted.Render(body),
	))
}

func commandPanel(width int, label, value, hint string) string {
	line := styleAccent.Render(label+": ") + styleText(value) + styleGlowCursor()
	if hint != "" {
		line += "\n" + styleMuted.Render(hint)
	}
	return panelWithBorder(width, line, colorSecondary)
}

func styleGlowCursor() string {
	return lipgloss.NewStyle().
		Foreground(colorGlow).
		Render("█")
}

func divider(width int) string {
	if width < 3 {
		width = 3
	}
	return styleDivider.Render(strings.Repeat("━", width))
}

func thinDivider(width int) string {
	if width < 3 {
		width = 3
	}
	return styleDivider.Render(strings.Repeat("─", width))
}

func styleText(s string) string {
	return lipgloss.NewStyle().Foreground(colorText).Render(s)
}

func truncateEnd(value string, width int) string {
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}
	runes := []rune(value)
	if width <= 1 {
		return "…"
	}
	if len(runes) <= width {
		return value
	}
	return string(runes[:width-1]) + "…"
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatPercent(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

func newHelpModel(width int) help.Model {
	h := help.New()
	h.Width = width
	h.ShortSeparator = "  │  "
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(colorGlow).Bold(true)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(colorMuted)
	h.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(colorSubtle)
	h.Styles.Ellipsis = lipgloss.NewStyle().Foreground(colorSubtle)
	h.Styles.FullKey = h.Styles.ShortKey
	h.Styles.FullDesc = h.Styles.ShortDesc
	h.Styles.FullSeparator = h.Styles.ShortSeparator
	return h
}
