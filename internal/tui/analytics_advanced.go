package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

const (
	activityTrendDays = 14
	hoursPerDay       = 24
)

// dayBucket aggregates per-day activity counters.
type dayBucket struct {
	Date     time.Time
	Sessions int
	Prompts  int
	Tokens   int
}

// activityTimeline holds per-day buckets ordered oldest -> newest plus a 24-hour histogram.
type activityTimeline struct {
	Days       []dayBucket
	Hours      [hoursPerDay]int
	HasData    bool
	StartDate  time.Time
	EndDate    time.Time
	TotalDays  int
	TotalHours int
}

// cacheUsageEntry summarizes cache token reads/writes per agent.
type cacheUsageEntry struct {
	Agent       string
	Reads       int
	Writes      int
	TotalInput  int
	TotalOutput int
}

// projectInsight ranks projects by both session count and token volume.
type projectInsight struct {
	Path     string
	Sessions int
	Tokens   int
	LastSeen time.Time
}

// sessionStateBreakdown counts sessions by lifecycle state.
type sessionStateBreakdown struct {
	Active int
	Idle   int
	Failed int
	Total  int
}

// efficiencySummary captures token efficiency at the agent level.
type efficiencySummary struct {
	Agent           string
	Prompts         int
	InputTokens     int
	OutputTokens    int
	TokensPerPrompt int
	IORatio         float64 // output / input; 0 when input is 0
}

// buildActivityTimeline aggregates per-day and per-hour activity over the
// trailing activityTrendDays window relative to the most recent session activity.
func buildActivityTimeline(sessions []*models.Session) activityTimeline {
	var timeline activityTimeline
	if len(sessions) == 0 {
		return timeline
	}
	// Determine the latest activity timestamp anchoring the trailing window.
	var newest time.Time
	for _, session := range sessions {
		if session == nil {
			continue
		}
		ts := session.LastUpdated
		if ts.IsZero() {
			ts = session.StartTime
		}
		if !ts.IsZero() && ts.After(newest) {
			newest = ts
		}
	}
	if newest.IsZero() {
		return timeline
	}
	endDate := time.Date(newest.Year(), newest.Month(), newest.Day(), 0, 0, 0, 0, newest.Location())
	startDate := endDate.AddDate(0, 0, -(activityTrendDays - 1))
	days := make([]dayBucket, activityTrendDays)
	for i := range days {
		days[i] = dayBucket{Date: startDate.AddDate(0, 0, i)}
	}
	dayIndex := func(ts time.Time) int {
		if ts.IsZero() {
			return -1
		}
		t := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, ts.Location())
		diff := int(t.Sub(startDate).Hours() / 24)
		if diff < 0 || diff >= activityTrendDays {
			return -1
		}
		return diff
	}

	for _, session := range sessions {
		if session == nil {
			continue
		}
		anchor := session.StartTime
		if anchor.IsZero() {
			anchor = session.LastUpdated
		}
		if idx := dayIndex(anchor); idx >= 0 {
			days[idx].Sessions++
			days[idx].Tokens += sessionTokenTotal(session)
		}
		for _, message := range session.Messages {
			if message.Timestamp.IsZero() {
				continue
			}
			timeline.Hours[message.Timestamp.Hour()]++
			timeline.TotalHours++
			if message.Role == "user" {
				if idx := dayIndex(message.Timestamp); idx >= 0 {
					days[idx].Prompts++
				}
			}
		}
	}

	hasAny := false
	totalDays := 0
	for _, day := range days {
		if day.Sessions > 0 || day.Prompts > 0 || day.Tokens > 0 {
			hasAny = true
			totalDays++
		}
	}
	timeline.Days = days
	timeline.StartDate = startDate
	timeline.EndDate = endDate
	timeline.HasData = hasAny
	timeline.TotalDays = totalDays
	return timeline
}

// summarizeCacheUsage returns one entry per agent that reported any cache token activity.
func summarizeCacheUsage(sessions []*models.Session) []cacheUsageEntry {
	byAgent := make(map[string]*cacheUsageEntry)
	for _, session := range sessions {
		if session == nil {
			continue
		}
		entry := byAgent[string(session.AgentType)]
		if entry == nil {
			entry = &cacheUsageEntry{Agent: string(session.AgentType)}
			byAgent[string(session.AgentType)] = entry
		}
		entry.Reads += session.TotalTokens.CacheReads
		entry.Writes += session.TotalTokens.CacheWrites
		entry.TotalInput += session.TotalTokens.InputTokens
		entry.TotalOutput += session.TotalTokens.OutputTokens
	}
	var out []cacheUsageEntry
	for _, entry := range byAgent {
		if entry.Reads == 0 && entry.Writes == 0 {
			continue
		}
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Reads != out[j].Reads {
			return out[i].Reads > out[j].Reads
		}
		return out[i].Agent < out[j].Agent
	})
	return out
}

// summarizeProjectsByTokens ranks projects by total tokens across all sessions.
func summarizeProjectsByTokens(sessions []*models.Session) []projectInsight {
	byProject := make(map[string]*projectInsight)
	for _, session := range sessions {
		if session == nil || session.ProjectPath == "" {
			continue
		}
		entry := byProject[session.ProjectPath]
		if entry == nil {
			entry = &projectInsight{Path: session.ProjectPath}
			byProject[session.ProjectPath] = entry
		}
		entry.Sessions++
		entry.Tokens += sessionTokenTotal(session)
		ts := session.LastUpdated
		if ts.IsZero() {
			ts = session.StartTime
		}
		if ts.After(entry.LastSeen) {
			entry.LastSeen = ts
		}
	}
	var out []projectInsight
	for _, entry := range byProject {
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tokens != out[j].Tokens {
			return out[i].Tokens > out[j].Tokens
		}
		if out[i].Sessions != out[j].Sessions {
			return out[i].Sessions > out[j].Sessions
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// sessionStateCounts groups sessions into active / idle / failed buckets.
func sessionStateCounts(sessions []*models.Session) sessionStateBreakdown {
	var b sessionStateBreakdown
	for _, session := range sessions {
		if session == nil {
			continue
		}
		b.Total++
		switch sessionStatus(session) {
		case statusActive:
			b.Active++
		case statusFailed:
			b.Failed++
		default:
			b.Idle++
		}
	}
	return b
}

// summarizeAgentEfficiency aggregates per-agent token efficiency from prompt threads.
func summarizeAgentEfficiency(threads []promptThread) []efficiencySummary {
	byAgent := make(map[string]*efficiencySummary)
	for _, thread := range threads {
		entry := byAgent[thread.Agent]
		if entry == nil {
			entry = &efficiencySummary{Agent: thread.Agent}
			byAgent[thread.Agent] = entry
		}
		entry.Prompts++
		entry.InputTokens += thread.InputTokens
		entry.OutputTokens += thread.OutputTokens
	}
	var out []efficiencySummary
	for _, entry := range byAgent {
		total := entry.InputTokens + entry.OutputTokens
		if entry.Prompts > 0 {
			entry.TokensPerPrompt = total / entry.Prompts
		}
		if entry.InputTokens > 0 {
			entry.IORatio = float64(entry.OutputTokens) / float64(entry.InputTokens)
		}
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool {
		left := out[i].InputTokens + out[i].OutputTokens
		right := out[j].InputTokens + out[j].OutputTokens
		if left != right {
			return left > right
		}
		return out[i].Agent < out[j].Agent
	})
	return out
}

// averageThreadLatency returns the mean first-response latency across all threads
// that recorded a positive latency value.
func averageThreadLatency(threads []promptThread) time.Duration {
	var sum time.Duration
	count := 0
	for _, thread := range threads {
		if thread.FirstResponseLatency > 0 {
			sum += thread.FirstResponseLatency
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / time.Duration(count)
}

// toolSuccessRate returns the overall tool success percentage across the summary.
// Returns -1 when no completed/failed events have been recorded.
func toolSuccessRate(tools []toolSummary) float64 {
	completed, finished := 0, 0
	for _, tool := range tools {
		completed += tool.Completed
		finished += tool.Completed + tool.Failed
	}
	if finished == 0 {
		return -1
	}
	return float64(completed) * 100.0 / float64(finished)
}

// sparkline renders an inline sparkline using block characters.
func sparkline(values []int) string {
	if len(values) == 0 {
		return ""
	}
	blocks := []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	maxVal := 0
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == 0 {
		return strings.Repeat(string(blocks[0]), len(values))
	}
	var sb strings.Builder
	for _, v := range values {
		idx := v * (len(blocks) - 1) / maxVal
		if idx < 0 {
			idx = 0
		}
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		sb.WriteRune(blocks[idx])
	}
	return sb.String()
}

func renderActivityTrend(width int, timeline activityTimeline) string {
	var b strings.Builder
	b.WriteString(sectionHeader("Activity trend", "rolling 14-day session and prompt cadence", width-4) + "\n")
	if !timeline.HasData {
		b.WriteString("  " + styleMuted.Render("Not enough timestamped activity to plot a trend yet.") + "\n")
		return quietPanel(width, b.String())
	}

	sessions := make([]int, len(timeline.Days))
	prompts := make([]int, len(timeline.Days))
	tokens := make([]int, len(timeline.Days))
	maxSessions, maxPrompts, totalSessions, totalPrompts, totalTokens := 0, 0, 0, 0, 0
	for i, day := range timeline.Days {
		sessions[i] = day.Sessions
		prompts[i] = day.Prompts
		tokens[i] = day.Tokens
		totalSessions += day.Sessions
		totalPrompts += day.Prompts
		totalTokens += day.Tokens
		if day.Sessions > maxSessions {
			maxSessions = day.Sessions
		}
		if day.Prompts > maxPrompts {
			maxPrompts = day.Prompts
		}
	}

	label := lipgloss.NewStyle().Width(10)
	b.WriteString(fmt.Sprintf("  %s %s  %s\n",
		label.Render(styleMuted.Render("sessions")),
		styleAccent.Render(sparkline(sessions)),
		styleMuted.Render(fmt.Sprintf("peak %d · total %d", maxSessions, totalSessions)),
	))
	b.WriteString(fmt.Sprintf("  %s %s  %s\n",
		label.Render(styleMuted.Render("prompts")),
		styleInfo.Render(sparkline(prompts)),
		styleMuted.Render(fmt.Sprintf("peak %d · total %d", maxPrompts, totalPrompts)),
	))
	b.WriteString(fmt.Sprintf("  %s %s  %s\n",
		label.Render(styleMuted.Render("tokens")),
		styleSuccess.Render(sparkline(tokens)),
		styleMuted.Render(fmt.Sprintf("total %s", compactInt(totalTokens))),
	))
	if !timeline.StartDate.IsZero() && !timeline.EndDate.IsZero() {
		b.WriteString("  " + styleMuted.Render(fmt.Sprintf("range %s → %s", timeline.StartDate.Format("Jan 02"), timeline.EndDate.Format("Jan 02"))) + "\n")
	}

	if timeline.TotalHours > 0 {
		b.WriteString("\n")
		b.WriteString("  " + styleMuted.Render("hour-of-day  ") + styleAccent.Render(sparkline(timeline.Hours[:])) + "\n")
		peakHour, peakValue := 0, 0
		for hour, value := range timeline.Hours {
			if value > peakValue {
				peakValue = value
				peakHour = hour
			}
		}
		b.WriteString("  " + styleMuted.Render(fmt.Sprintf("00          06          12          18          peak %02d:00", peakHour)) + "\n")
	}

	return quietPanel(width, b.String())
}

func renderEfficiencyPanel(width int, threads []promptThread, agents []efficiencySummary) string {
	var b strings.Builder
	b.WriteString(sectionHeader("Latency & token efficiency", "first-response latency and IO ratios", width-4) + "\n")
	if len(threads) == 0 {
		b.WriteString("  " + styleMuted.Render("No prompt threads available for efficiency analysis.") + "\n")
		return quietPanel(width, b.String())
	}

	overallLatency := averageThreadLatency(threads)
	latencyLabel := "—"
	if overallLatency > 0 {
		latencyLabel = models.FormatDuration(overallLatency)
	}
	totalIn, totalOut := 0, 0
	for _, thread := range threads {
		totalIn += thread.InputTokens
		totalOut += thread.OutputTokens
	}
	ratioLabel := "—"
	if totalIn > 0 {
		ratioLabel = fmt.Sprintf("%.2f×", float64(totalOut)/float64(totalIn))
	}
	avgPromptTokens := (totalIn + totalOut) / maxInt(1, len(threads))

	b.WriteString(fmt.Sprintf("  %s  %s  %s\n",
		metricChip("Avg first-response", latencyLabel, "⏱", styleInfo),
		metricChip("Output ÷ input", ratioLabel, "Σ", styleAccent),
		metricChip("Avg tokens/prompt", compactInt(avgPromptTokens), "◇", styleSuccess),
	))

	if len(agents) > 0 {
		b.WriteString("\n")
		for _, agent := range agents {
			ratio := "—"
			if agent.InputTokens > 0 {
				ratio = fmt.Sprintf("%.2f×", agent.IORatio)
			}
			b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
				lipgloss.NewStyle().Width(20).Render(agentBadge(agent.Agent)),
				styleAccent.Render(fmt.Sprintf("%d prompts", agent.Prompts)),
				styleMuted.Render(compactInt(agent.TokensPerPrompt)+" avg/prompt"),
				styleMuted.Render(ratio+" out/in"),
			))
		}
	}
	return quietPanel(width, b.String())
}

func renderCacheUtilizationPanel(width int, entries []cacheUsageEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(sectionHeader("Cache utilization", "prompt-cache reads vs writes per agent", width-4) + "\n")
	for _, entry := range entries {
		total := entry.Reads + entry.Writes
		ratio := "—"
		if total > 0 {
			ratio = formatPercent(float64(entry.Reads) * 100.0 / float64(total))
		}
		b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
			lipgloss.NewStyle().Width(20).Render(agentBadge(entry.Agent)),
			styleAccent.Render(compactInt(entry.Reads)+" reads"),
			styleMuted.Render(compactInt(entry.Writes)+" writes"),
			styleSuccess.Render(ratio+" hit"),
		))
	}
	return quietPanel(width, b.String())
}

func renderProjectsByTokens(width int, entries []projectInsight, limit int) string {
	if len(entries) == 0 {
		return ""
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	var b strings.Builder
	b.WriteString(sectionHeader("Top projects by token volume", "where tokens are concentrated", width-4) + "\n")
	for i, entry := range entries {
		name := entry.Path
		if len(name) > 40 {
			name = "..." + name[len(name)-37:]
		}
		rank := styleAccent.Render(fmt.Sprintf("%2d", i+1))
		b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
			rank,
			lipgloss.NewStyle().Width(42).Render(styleText(name)),
			styleMuted.Render(compactInt(entry.Tokens)+" tokens"),
			styleMuted.Render(fmt.Sprintf("%d sessions", entry.Sessions)),
		))
	}
	return quietPanel(width, b.String())
}

// renderSessionStateChips returns a row of chips summarizing active / idle / failed counts.
func renderSessionStateChips(b sessionStateBreakdown) string {
	if b.Total == 0 {
		return ""
	}
	chips := []string{quietPill(fmt.Sprintf("%d total", b.Total))}
	if b.Active > 0 {
		chips = append(chips, statusCountChip(statusActive, b.Active))
	}
	if b.Idle > 0 {
		chips = append(chips, statusCountChip(statusIdle, b.Idle))
	}
	if b.Failed > 0 {
		chips = append(chips, statusCountChip(statusFailed, b.Failed))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, joinWithSpaces(chips)...)
}
