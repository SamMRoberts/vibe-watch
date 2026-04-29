package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

const maxAnalyticsSectionWidth = 120

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
		return emptyState(a.width-4, "No analytics yet", "Start using agentic coding tools to populate session and token charts.")
	}

	// Aggregate by agent
	agentCounts := make(map[string]int)
	agentTokens := make(map[string]int)
	projectCounts := make(map[string]int)
	totalTokens := 0

	for _, s := range a.sessions {
		agentKey := string(s.AgentType)
		agentCounts[agentKey]++
		tok := s.TotalInputTokens() + s.TotalOutputTokens()
		agentTokens[agentKey] += tok
		totalTokens += tok
		if s.ProjectPath != "" {
			projectCounts[s.ProjectPath]++
		}
	}

	agentNames := []string{"Claude Code", "Codex CLI", "Copilot CLI", "Copilot Chat", "Amazon Q"}
	maxCount := 0
	for _, name := range agentNames {
		if agentCounts[name] > maxCount {
			maxCount = agentCounts[name]
		}
	}

	sectionWidth := minInt(a.width-4, maxAnalyticsSectionWidth)
	barWidth := clampInt(sectionWidth-46, 18, 54)

	statsRow := lipgloss.JoinHorizontal(lipgloss.Top,
		metricCardWidth("Sessions", fmt.Sprintf("%d", len(a.sessions)), "☷", styleAccent, clampInt((a.width-8)/2, 16, 22)),
		"  ",
		metricCardWidth("Tokens", compactInt(totalTokens), "◇", styleAccent, clampInt((a.width-8)/2, 16, 22)),
	)
	sb.WriteString(sectionHeader("Observatory analytics", "session distribution and token load", sectionWidth) + "\n\n")
	sb.WriteString(statsRow + "\n\n")

	var agentChart strings.Builder
	agentChart.WriteString(sectionHeader("Sessions by agent", "", sectionWidth-4) + "\n")

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

		agentChart.WriteString(fmt.Sprintf("  %s %s%s  %s  %s\n",
			label,
			sty.Render(filledBar),
			lipgloss.NewStyle().Foreground(colorSubtle).Render(emptyBar),
			styleText(fmt.Sprintf("%d sessions", count)),
			styleMuted.Render(compactInt(agentTokens[agentName])+" tokens"),
		))
	}

	sb.WriteString(quietPanel(sectionWidth, agentChart.String()) + "\n\n")

	var projects strings.Builder
	projects.WriteString(sectionHeader("Most active projects", "", sectionWidth-4) + "\n")

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
		projects.WriteString(fmt.Sprintf("  %s  %s %s\n",
			rank,
			projectName,
			styleMuted.Render(fmt.Sprintf("%d sessions", p.count)),
		))
	}

	sb.WriteString(quietPanel(sectionWidth, projects.String()) + "\n\n")

	var tokens strings.Builder
	tokens.WriteString(sectionHeader("Token usage by agent", "", sectionWidth-4) + "\n")
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
		tokens.WriteString(fmt.Sprintf("  %s %s%s  %s  %s\n",
			label,
			sty.Render(bar),
			lipgloss.NewStyle().Foreground(colorSubtle).Render(emptyBar),
			formatPercent(pct),
			styleText(compactInt(tok)+" tokens"),
		))
	}

	sb.WriteString(quietPanel(sectionWidth, tokens.String()))

	return sb.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
