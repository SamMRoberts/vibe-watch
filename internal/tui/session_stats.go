package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

type sessionDataStats struct {
	Prompts                 int
	AvgFirstResponseLatency time.Duration
	AvgTokensPerPrompt      int
	ToolSuccessRate         float64
	Threads                 []promptThread
	Categories              []categorySummary
	Tools                   []toolSummary
	MessageCounts           map[string]int
	TokenTrend              []int
	HourlyActivity          [hoursPerDay]int
}

func analyzeSessionData(session *models.Session) sessionDataStats {
	if session == nil {
		return sessionDataStats{ToolSuccessRate: -1}
	}
	threads := buildPromptThreads([]*models.Session{session})
	categories := summarizePromptCategories(threads)
	tools, _ := summarizeToolActivity([]*models.Session{session})
	stats := sessionDataStats{
		Prompts:                 len(threads),
		AvgFirstResponseLatency: averageThreadLatency(threads),
		ToolSuccessRate:         toolSuccessRate(tools),
		Threads:                 threads,
		Categories:              categories,
		Tools:                   tools,
		MessageCounts:           make(map[string]int),
		TokenTrend:              make([]int, 0, len(threads)),
	}

	totalThreadTokens := 0
	for _, thread := range threads {
		stats.TokenTrend = append(stats.TokenTrend, thread.Tokens)
		totalThreadTokens += thread.Tokens
	}
	if len(threads) > 0 {
		stats.AvgTokensPerPrompt = totalThreadTokens / len(threads)
	}

	for _, message := range session.Messages {
		role := message.Role
		if role == "" {
			role = "unknown"
		}
		stats.MessageCounts[role]++
		if !message.Timestamp.IsZero() {
			stats.HourlyActivity[message.Timestamp.Hour()]++
		}
	}

	return stats
}

func renderSessionAnalyticsPanel(session *models.Session, width int) string {
	if session == nil {
		return ""
	}
	width = clampInt(width, 44, maxAnalyticsSectionWidth)
	stats := analyzeSessionData(session)

	var b strings.Builder
	b.WriteString(sectionHeader("Session data analytics", "press a to hide/show", width-4) + "\n")
	b.WriteString(renderSessionAnalyticsKPI(stats) + "\n\n")
	b.WriteString(renderSessionTokenTrend(stats, width) + "\n")
	b.WriteString(renderSessionMessageMix(stats, width) + "\n")
	if categoryRows := renderSessionCategoryMix(stats, width); categoryRows != "" {
		b.WriteString(categoryRows + "\n")
	}
	if hourly := renderSessionHourlyActivity(stats); hourly != "" {
		b.WriteString(hourly + "\n")
	}
	if tools := renderSessionToolStats(stats); tools != "" {
		b.WriteString(tools)
	}
	return quietPanel(width, b.String())
}

func renderSessionAnalyticsKPI(stats sessionDataStats) string {
	latency := "—"
	if stats.AvgFirstResponseLatency > 0 {
		latency = models.FormatDuration(stats.AvgFirstResponseLatency)
	}
	success := "—"
	successStyle := styleAccent
	if stats.ToolSuccessRate >= 0 {
		success = fmt.Sprintf("%.0f%%", stats.ToolSuccessRate)
		successStyle = toolSuccessStyle(stats.ToolSuccessRate)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		metricChip("Prompts", fmt.Sprintf("%d", stats.Prompts), "✎", styleInfo),
		" ",
		metricChip("Avg latency", latency, "⏱", styleAccent),
		" ",
		metricChip("Avg tokens", compactInt(stats.AvgTokensPerPrompt), "◇", styleSuccess),
		" ",
		metricChip("Tool success", success, "✓", successStyle),
	)
}

func renderSessionTokenTrend(stats sessionDataStats, width int) string {
	if len(stats.TokenTrend) == 0 {
		return "  " + styleMuted.Render("Prompt token trend: no user prompt spans yet.")
	}
	chartWidth := clampInt(width-44, 10, 48)
	values := trimIntSeries(stats.TokenTrend, chartWidth)
	peak := 0
	for _, value := range stats.TokenTrend {
		if value > peak {
			peak = value
		}
	}
	return fmt.Sprintf("  %s %s  %s",
		styleMuted.Render("Prompt token trend"),
		styleSuccess.Render(sparkline(values)),
		styleMuted.Render("peak "+compactInt(peak)),
	)
}

func renderSessionMessageMix(stats sessionDataStats, width int) string {
	roles := []string{"user", "assistant", "tool", "subagent", "error", "session", "system"}
	maxCount := 0
	for _, count := range stats.MessageCounts {
		if count > maxCount {
			maxCount = count
		}
	}
	if maxCount == 0 {
		return "  " + styleMuted.Render("Message mix: no messages recorded.")
	}
	barWidth := clampInt(width-28, 8, 36)
	var b strings.Builder
	b.WriteString("  " + styleMuted.Render("Message mix") + "\n")
	for _, role := range roles {
		count := stats.MessageCounts[role]
		if count == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("  %s %s %s\n",
			lipgloss.NewStyle().Width(10).Render(role),
			sessionStatBar(count, maxCount, barWidth, roleStyle(role)),
			styleMuted.Render(fmt.Sprintf("%d", count)),
		))
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderSessionCategoryMix(stats sessionDataStats, width int) string {
	if len(stats.Categories) == 0 {
		return ""
	}
	maxCount := 0
	for _, category := range stats.Categories {
		if category.Count > maxCount {
			maxCount = category.Count
		}
	}
	barWidth := clampInt(width-38, 8, 34)
	var b strings.Builder
	b.WriteString("  " + styleMuted.Render("Prompt categories") + "\n")
	for i, category := range stats.Categories {
		if i >= 4 {
			break
		}
		b.WriteString(fmt.Sprintf("  %s %s %s\n",
			lipgloss.NewStyle().Width(16).Render(category.Name),
			sessionStatBar(category.Count, maxCount, barWidth, styleInfo),
			styleMuted.Render(fmt.Sprintf("%d prompts · %s tokens", category.Count, compactInt(category.Tokens))),
		))
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderSessionHourlyActivity(stats sessionDataStats) string {
	total := 0
	peakHour, peakValue := 0, 0
	for hour, value := range stats.HourlyActivity {
		total += value
		if value > peakValue {
			peakHour = hour
			peakValue = value
		}
	}
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("  %s %s\n  %s",
		styleMuted.Render("Hour-of-day"),
		styleAccent.Render(sparkline(stats.HourlyActivity[:])),
		styleMuted.Render(fmt.Sprintf("00          06          12          18          peak %02d:00", peakHour)),
	)
}

func renderSessionToolStats(stats sessionDataStats) string {
	if len(stats.Tools) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("  " + styleMuted.Render("Tool outcomes") + "\n")
	for i, tool := range stats.Tools {
		if i >= 5 {
			break
		}
		rate := "—"
		finished := tool.Completed + tool.Failed
		if finished > 0 {
			rate = fmt.Sprintf("%.0f%%", float64(tool.Completed)*100.0/float64(finished))
		}
		b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
			lipgloss.NewStyle().Width(18).Render(truncateEnd(tool.Name, 18)),
			styleAccent.Render(fmt.Sprintf("%d events", tool.Count)),
			styleSuccess.Render(fmt.Sprintf("%d ok", tool.Completed)),
			toolSuccessStyleForCounts(tool.Failed, finished).Render(rate+" success"),
		))
	}
	return strings.TrimRight(b.String(), "\n")
}

func sessionStatBar(value, maxValue, width int, sty lipgloss.Style) string {
	if width < 1 {
		width = 1
	}
	filled := 0
	if maxValue > 0 {
		filled = value * width / maxValue
	}
	if value > 0 && filled == 0 {
		filled = 1
	}
	if filled > width {
		filled = width
	}
	return sty.Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(colorSubtle).Render(strings.Repeat("░", width-filled))
}

func trimIntSeries(values []int, maxLen int) []int {
	if maxLen <= 0 || len(values) <= maxLen {
		return values
	}
	return values[len(values)-maxLen:]
}

func roleStyle(role string) lipgloss.Style {
	switch role {
	case "user":
		return styleUserMsg
	case "assistant":
		return styleAssistantMsg
	case "tool", "subagent":
		return styleToolMsg
	case "error":
		return styleError
	default:
		return styleAccent
	}
}

func toolSuccessStyle(rate float64) lipgloss.Style {
	switch {
	case rate < 50:
		return styleError
	case rate < 80:
		return styleWarning
	default:
		return styleSuccess
	}
}

func toolSuccessStyleForCounts(failed, finished int) lipgloss.Style {
	if finished == 0 {
		return styleAccent
	}
	return toolSuccessStyle(float64(finished-failed) * 100.0 / float64(finished))
}
