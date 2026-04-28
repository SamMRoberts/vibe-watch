package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

// maxAnalyticsSectionWidth keeps chart dividers comfortably within standard 80-column terminals.
const maxAnalyticsSectionWidth = 74

type AnalyticsView struct {
	sessions []*models.Session
	width    int
	height   int
}

func NewAnalyticsView(width, height int) *AnalyticsView {
	return &AnalyticsView{width: width, height: height}
}

func (a *AnalyticsView) SetSize(width, height int) {
	a.width = width
	a.height = height
}

func (a *AnalyticsView) SetSessions(sessions []*models.Session) {
	a.sessions = sessions
}

func (a *AnalyticsView) View() string {
	var sb strings.Builder

	if len(a.sessions) == 0 {
		return styleCard.
			Width(a.width - 4).
			Height(a.height - 6).
			Align(lipgloss.Center).
			Render(styleMuted.Render("No data yet.\n\n✦ Start using agentic coding tools to see analytics."))
	}

	// Aggregate by agent
	agentCounts := make(map[string]int)
	agentTokens := make(map[string]int)
	agentCosts := make(map[string]float64)
	projectCounts := make(map[string]int)
	totalTokens := 0
	totalCost := 0.0

	for _, s := range a.sessions {
		agentKey := string(s.AgentType)
		agentCounts[agentKey]++
		tok := s.TotalInputTokens() + s.TotalOutputTokens()
		agentTokens[agentKey] += tok
		agentCosts[agentKey] += s.EstimatedCost()
		totalTokens += tok
		totalCost += s.EstimatedCost()
		if s.ProjectPath != "" {
			projectCounts[s.ProjectPath]++
		}
	}

	// Sessions by agent bar chart
	sb.WriteString(styleAccent.Render("╭─ Sessions by Agent") + "\n")
	sb.WriteString(divider(minInt(a.width-4, maxAnalyticsSectionWidth)) + "\n\n")

	agentNames := []string{"Claude Code", "Codex CLI", "Copilot CLI", "Amazon Q"}
	maxCount := 0
	for _, name := range agentNames {
		if agentCounts[name] > maxCount {
			maxCount = agentCounts[name]
		}
	}

	barWidth := 30
	if a.width > 80 {
		barWidth = 50
	}

	for _, agentName := range agentNames {
		count := agentCounts[agentName]
		barLen := 0
		if maxCount > 0 {
			barLen = count * barWidth / maxCount
		}
		filledBar := strings.Repeat("█", barLen)
		emptyBar := strings.Repeat("░", barWidth-barLen)
		sty := agentStyle(agentName)
		label := lipgloss.NewStyle().Width(22).Render(agentBadge(agentName))

		sb.WriteString(fmt.Sprintf("  %s %s%s  %s  %s\n",
			label,
			sty.Render(filledBar),
			lipgloss.NewStyle().Foreground(colorSubtle).Render(emptyBar),
			styleText(fmt.Sprintf("%d sessions", count)),
			styleWarning.Render(fmt.Sprintf("$%.4f", agentCosts[agentName])),
		))
	}

	sb.WriteString("\n")

	// Summary stats
	statsRow := lipgloss.JoinHorizontal(lipgloss.Top,
		metricCard("Total Sessions", fmt.Sprintf("%d", len(a.sessions)), "☷", styleAccent),
		"  ",
		metricCard("Total Tokens", fmt.Sprintf("%d", totalTokens), "◇", styleAccent),
		"  ",
		metricCard("Total Cost", fmt.Sprintf("$%.4f", totalCost), "◉", styleWarning),
	)
	sb.WriteString(statsRow + "\n\n")

	// Top projects
	sb.WriteString(styleAccent.Render("╭─ Most Active Projects") + "\n")
	sb.WriteString(divider(minInt(a.width-4, maxAnalyticsSectionWidth)) + "\n\n")

	type projEntry struct {
		name  string
		count int
	}
	var projs []projEntry
	for k, v := range projectCounts {
		projs = append(projs, projEntry{k, v})
	}
	sort.Slice(projs, func(i, j int) bool {
		return projs[i].count > projs[j].count
	})
	if len(projs) > 10 {
		projs = projs[:10]
	}
	for i, p := range projs {
		name := p.name
		if len(name) > 40 {
			name = "..." + name[len(name)-37:]
		}
		rank := styleAccent.Render(fmt.Sprintf("%2d", i+1))
		projectName := lipgloss.NewStyle().Width(42).Render(styleText(name))
		sb.WriteString(fmt.Sprintf("  %s  %s %s\n",
			rank,
			projectName,
			styleMuted.Render(fmt.Sprintf("%d sessions", p.count)),
		))
	}

	sb.WriteString("\n")

	// Token breakdown
	sb.WriteString(styleAccent.Render("╭─ Token Usage by Agent") + "\n")
	sb.WriteString(divider(minInt(a.width-4, maxAnalyticsSectionWidth)) + "\n\n")
	for _, agentName := range agentNames {
		tok := agentTokens[agentName]
		if tok == 0 {
			continue
		}
		sty := agentStyle(agentName)
		pct := 0.0
		if totalTokens > 0 {
			pct = float64(tok) * 100.0 / float64(totalTokens)
		}
		barLen := int(pct * float64(barWidth) / 100.0)
		if barLen > barWidth {
			barLen = barWidth
		}
		bar := strings.Repeat("█", barLen)
		emptyBar := strings.Repeat("░", barWidth-barLen)
		label := lipgloss.NewStyle().Width(22).Render(agentBadge(agentName))
		sb.WriteString(fmt.Sprintf("  %s %s%s  %.1f%%  %s\n",
			label,
			sty.Render(bar),
			lipgloss.NewStyle().Foreground(colorSubtle).Render(emptyBar),
			pct,
			styleText(fmt.Sprintf("%d tokens", tok)),
		))
	}

	return sb.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
