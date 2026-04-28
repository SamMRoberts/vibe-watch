package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorBackground = lipgloss.Color("#101426")
	colorSurface    = lipgloss.Color("#1a2040")
	colorSurfaceAlt = lipgloss.Color("#222a52")
	colorPrimary    = lipgloss.Color("#7c3aed")
	colorSecondary  = lipgloss.Color("#06b6d4")
	colorAccent     = lipgloss.Color("#f43f5e")
	colorGlow       = lipgloss.Color("#facc15")
	colorText       = lipgloss.Color("#f8fafc")
	colorMuted      = lipgloss.Color("#94a3b8")
	colorSubtle     = lipgloss.Color("#475569")
	colorSuccess    = lipgloss.Color("#22c55e")
	colorWarning    = lipgloss.Color("#f59e0b")
	colorError      = lipgloss.Color("#ef4444")
	colorClaude     = lipgloss.Color("#ff8a65")
	colorCodex      = lipgloss.Color("#34d399")
	colorCopilot    = lipgloss.Color("#a78bfa")
	colorAmazonQ    = lipgloss.Color("#fb923c")
)

var (
	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorGlow).
			PaddingLeft(1)

	styleSubtitle = lipgloss.NewStyle().
			Foreground(colorMuted).
			PaddingLeft(1)

	styleHeader = lipgloss.NewStyle().
			Background(colorSurface).
			Foreground(colorText).
			Bold(true).
			Padding(0, 1)

	styleSelected = lipgloss.NewStyle().
			Background(colorPrimary).
			Foreground(colorText).
			Bold(true)

	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary)

	styleCard = lipgloss.NewStyle().
			Background(colorSurface).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1)

	stylePanel = lipgloss.NewStyle().
			Background(colorSurface).
			Border(lipgloss.ThickBorder()).
			BorderForeground(colorSecondary).
			Padding(1, 2)

	styleDivider = lipgloss.NewStyle().
			Foreground(colorSubtle)

	styleMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleAccent = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	styleSuccess = lipgloss.NewStyle().
			Foreground(colorSuccess)

	styleWarning = lipgloss.NewStyle().
			Foreground(colorWarning)

	styleError = lipgloss.NewStyle().
			Foreground(colorError)

	styleTab = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 2).
			MarginRight(1)

	styleActiveTab = lipgloss.NewStyle().
			Background(colorPrimary).
			Foreground(colorText).
			Bold(true).
			Padding(0, 2).
			MarginRight(1)

	styleHelp = lipgloss.NewStyle().
			Foreground(colorMuted).
			PaddingTop(1)

	styleUserMsg = lipgloss.NewStyle().
			Foreground(colorCopilot).
			Bold(true)

	styleAssistantMsg = lipgloss.NewStyle().
				Foreground(colorClaude).
				Bold(true)

	styleMessageContent = lipgloss.NewStyle().
				Foreground(colorText)

	styleAgentBadge = map[string]lipgloss.Style{
		"Claude Code": lipgloss.NewStyle().Foreground(colorClaude).Bold(true),
		"Codex CLI":   lipgloss.NewStyle().Foreground(colorCodex).Bold(true),
		"Copilot CLI": lipgloss.NewStyle().Foreground(colorCopilot).Bold(true),
		"Amazon Q":    lipgloss.NewStyle().Foreground(colorAmazonQ).Bold(true),
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
		Padding(0, 1)
	return sty.Render(agentIcon(agent) + " " + agent)
}

func agentIcon(agent string) string {
	switch agent {
	case "Claude Code":
		return "✹"
	case "Codex CLI":
		return "◆"
	case "Copilot CLI":
		return "◈"
	case "Amazon Q":
		return "⬢"
	default:
		return "●"
	}
}

func metricCard(label, value, icon string, valueStyle lipgloss.Style) string {
	return styleCard.Width(20).Render(
		styleMuted.Render(icon+" "+label) + "\n" +
			valueStyle.Render(value),
	)
}

func statusPill(active bool) string {
	if active {
		return lipgloss.NewStyle().
			Background(colorSuccess).
			Foreground(colorBackground).
			Bold(true).
			Padding(0, 1).
			Render("● ACTIVE")
	}
	return lipgloss.NewStyle().
		Background(colorSurfaceAlt).
		Foreground(colorMuted).
		Padding(0, 1).
		Render("○ IDLE")
}

func styleGlowCursor() string {
	return lipgloss.NewStyle().
		Foreground(colorGlow).
		Render("█")
}

func divider(width int) string {
	if width < 1 {
		width = 1
	}
	return styleDivider.Render(strings.Repeat("━", width))
}
