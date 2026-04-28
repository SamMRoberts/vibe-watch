package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

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
			Render(styleMuted.Render("No data yet.\n\nStart using agentic coding tools to see analytics."))
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
	sb.WriteString(styleAccent.Render("Sessions by Agent") + "\n\n")

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

		sb.WriteString(fmt.Sprintf("  %-12s %s%s %d sessions, $%.4f\n",
			sty.Render(agentName),
			sty.Render(filledBar),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#444")).Render(emptyBar),
			count,
			agentCosts[agentName],
		))
	}

	sb.WriteString("\n")

	// Summary stats
	statsRow := lipgloss.JoinHorizontal(lipgloss.Top,
		styleCard.Width(24).Render(
			styleMuted.Render("Total Sessions")+"\n"+
				styleAccent.Render(fmt.Sprintf("%d", len(a.sessions))),
		),
		"  ",
		styleCard.Width(24).Render(
			styleMuted.Render("Total Tokens")+"\n"+
				styleAccent.Render(fmt.Sprintf("%d", totalTokens)),
		),
		"  ",
		styleCard.Width(24).Render(
			styleMuted.Render("Total Cost")+"\n"+
				styleWarning.Render(fmt.Sprintf("$%.4f", totalCost)),
		),
	)
	sb.WriteString(statsRow + "\n\n")

	// Top projects
	sb.WriteString(styleAccent.Render("Most Active Projects") + "\n\n")

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
		sb.WriteString(fmt.Sprintf("  %2d. %-42s %d sessions\n",
			i+1,
			styleText(name),
			p.count,
		))
	}

	sb.WriteString("\n")

	// Token breakdown
	sb.WriteString(styleAccent.Render("Token Usage by Agent") + "\n\n")
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
		sb.WriteString(fmt.Sprintf("  %-12s %s %.1f%% (%d tokens)\n",
			sty.Render(agentName),
			sty.Render(bar),
			pct,
			tok,
		))
	}

	return sb.String()
}
