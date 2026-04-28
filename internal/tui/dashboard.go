package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

type DashboardView struct {
	table       table.Model
	sessions    []*models.Session
	filter      string
	width       int
	height      int
	filterMode  bool
	filterInput string
}

func NewDashboardView(width, height int) *DashboardView {
	cols := []table.Column{
		{Title: "Agent", Width: 18},
		{Title: "Project", Width: 20},
		{Title: "Msgs", Width: 6},
		{Title: "In Tok", Width: 8},
		{Title: "Out Tok", Width: 8},
		{Title: "Cost", Width: 9},
		{Title: "Duration", Width: 10},
		{Title: "Status", Width: 10},
		{Title: "Last Updated", Width: 14},
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(height-8),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorSecondary).
		BorderBottom(true).
		Bold(true).
		Foreground(colorGlow)
	s.Selected = s.Selected.
		Foreground(colorText).
		Background(colorPrimary).
		Bold(true)
	t.SetStyles(s)

	return &DashboardView{
		table:  t,
		width:  width,
		height: height,
	}
}

func (d *DashboardView) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.table.SetHeight(height - 8)
}

func (d *DashboardView) SetSessions(sessions []*models.Session, agentFilter string) {
	d.sessions = sessions
	d.updateTable(agentFilter)
}

func (d *DashboardView) updateTable(agentFilter string) {
	var rows []table.Row
	filterLower := strings.ToLower(d.filter)

	for _, s := range d.sessions {
		if agentFilter != "" {
			agentLower := strings.ToLower(string(s.AgentType))
			if !strings.Contains(agentLower, strings.ToLower(agentFilter)) {
				continue
			}
		}
		if filterLower != "" {
			proj := strings.ToLower(s.ProjectPath)
			agent := strings.ToLower(string(s.AgentType))
			if !strings.Contains(proj, filterLower) && !strings.Contains(agent, filterLower) {
				continue
			}
		}

		lastUpdated := s.LastUpdated.Format("15:04:05")
		if time.Since(s.LastUpdated) > 24*time.Hour {
			lastUpdated = s.LastUpdated.Format("Jan 02")
		}

		proj := s.ProjectPath
		if len(proj) > 19 {
			proj = "..." + proj[len(proj)-16:]
		}

		rows = append(rows, table.Row{
			agentBadge(string(s.AgentType)),
			proj,
			fmt.Sprintf("%d", s.MessageCount()),
			fmt.Sprintf("%d", s.TotalInputTokens()),
			fmt.Sprintf("%d", s.TotalOutputTokens()),
			styleWarning.Render(fmt.Sprintf("$%.4f", s.EstimatedCost())),
			models.FormatDuration(s.Duration()),
			statusPill(s.IsActive),
			lastUpdated,
		})
	}

	d.table.SetRows(rows)
}

func (d *DashboardView) SelectedIndex() int {
	return d.table.Cursor()
}

func (d *DashboardView) View(agentFilter string) string {
	var sb strings.Builder

	// Stats bar
	activeCount := 0
	totalCost := 0.0
	totalTokens := 0
	for _, s := range d.sessions {
		if s.IsActive {
			activeCount++
		}
		totalCost += s.EstimatedCost()
		totalTokens += s.TotalInputTokens() + s.TotalOutputTokens()
	}

	stats := lipgloss.JoinHorizontal(lipgloss.Top,
		metricCard("Sessions", fmt.Sprintf("%d", len(d.sessions)), "☷", styleAccent),
		"  ",
		metricCard("Active", fmt.Sprintf("%d", activeCount), "✦", styleSuccess),
		"  ",
		metricCard("Total Tokens", fmt.Sprintf("%d", totalTokens), "◇", styleAccent),
		"  ",
		metricCard("Total Cost", fmt.Sprintf("$%.4f", totalCost), "◉", styleWarning),
	)

	sb.WriteString(styleMuted.Render("╭─ live session telemetry") + "\n")
	sb.WriteString(stats)
	sb.WriteString("\n" + styleMuted.Render("╰─ tabular view") + "\n\n")

	if d.filterMode {
		sb.WriteString(stylePanel.Width(d.width-6).Render(styleAccent.Render("Filter: ")+d.filterInput+styleGlowCursor()) + "\n\n")
	}

	if len(d.sessions) == 0 {
		empty := styleCard.
			Width(d.width-4).
			Align(lipgloss.Center).
			Padding(3, 0).
			Render(
				styleMuted.Render("No sessions found.\n\n") +
					styleSubtitle.Render("✦ Run Claude Code, Codex CLI, Copilot CLI, or Amazon Q\nto see sessions appear here."),
			)
		sb.WriteString(empty)
	} else {
		sb.WriteString(d.table.View())
	}

	return sb.String()
}
