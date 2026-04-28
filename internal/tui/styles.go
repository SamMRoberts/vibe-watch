package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorBackground = lipgloss.Color("#1a1a2e")
	colorSurface    = lipgloss.Color("#16213e")
	colorPrimary    = lipgloss.Color("#0f3460")
	colorAccent     = lipgloss.Color("#e94560")
	colorText       = lipgloss.Color("#eaeaea")
	colorMuted      = lipgloss.Color("#888888")
	colorSuccess    = lipgloss.Color("#4caf50")
	colorWarning    = lipgloss.Color("#ff9800")
	colorError      = lipgloss.Color("#f44336")
	colorClaude     = lipgloss.Color("#cc785c")
	colorCodex      = lipgloss.Color("#74aa9c")
	colorCopilot    = lipgloss.Color("#6e40c9")
	colorAmazonQ    = lipgloss.Color("#ff9900")
)

var (
	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			PaddingLeft(1)

	styleSubtitle = lipgloss.NewStyle().
			Foreground(colorMuted).
			PaddingLeft(1)

	styleHeader = lipgloss.NewStyle().
			Background(colorPrimary).
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
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1)

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
			Padding(0, 2)

	styleActiveTab = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true).
			Padding(0, 2).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(colorAccent)

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
