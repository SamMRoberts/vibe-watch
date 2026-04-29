package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

const maxAnalyticsSectionWidth = 120

var promptCategoryOrder = []string{"Implementation", "Debugging", "Planning", "Exploration", "Refactor", "Review", "General"}

type AnalyticsView struct {
	viewport viewport.Model
	sessions []*models.Session
	width    int
	height   int
}

type promptThread struct {
	SessionID            string
	Agent                string
	Project              string
	Prompt               string
	Category             string
	Start                time.Time
	End                  time.Time
	Duration             time.Duration
	FirstResponseLatency time.Duration
	MessageCount         int
	ToolEvents           int
	FailedEvents         int
	Tokens               int
	InputTokens          int
	OutputTokens         int
}

type categorySummary struct {
	Name           string
	Count          int
	Tokens         int
	Duration       time.Duration
	Tools          int
	Failures       int
	LatencySum     time.Duration
	LatencySamples int
}

type toolSummary struct {
	Name      string
	Count     int
	Started   int
	Completed int
	Failed    int
}

type sessionInsight struct {
	Session *models.Session
	Value   int
	Label   string
}

type analyticsInsights struct {
	Threads       []promptThread
	Categories    []categorySummary
	Tools         []toolSummary
	Outliers      []sessionInsight
	Hints         []string
	FollowUps     int
	TotalFailures int
}

func NewAnalyticsView(width, height int) *AnalyticsView {
	vp := viewport.New(analyticsViewportWidth(width), analyticsViewportHeight(height))
	a := &AnalyticsView{viewport: vp, width: width, height: height}
	a.renderContent()
	return a
}

func (a *AnalyticsView) SetSize(width, height int) {
	a.width = width
	a.height = height
	a.viewport.Width = analyticsViewportWidth(width)
	a.viewport.Height = analyticsViewportHeight(height)
	a.renderContent()
}

func (a *AnalyticsView) SetSessions(sessions []*models.Session) {
	a.sessions = sessions
	a.renderContent()
}

func (a *AnalyticsView) View() string {
	return a.viewport.View()
}

func (a *AnalyticsView) ScrollDown() {
	a.viewport.ScrollDown(3)
}

func (a *AnalyticsView) ScrollUp() {
	a.viewport.ScrollUp(3)
}

func (a *AnalyticsView) PageDown() {
	a.viewport.HalfViewDown()
}

func (a *AnalyticsView) PageUp() {
	a.viewport.HalfViewUp()
}

func (a *AnalyticsView) GotoTop() {
	a.viewport.GotoTop()
}

func (a *AnalyticsView) GotoBottom() {
	a.viewport.GotoBottom()
}

func (a *AnalyticsView) FooterStatus() string {
	if a == nil {
		return ""
	}
	return fmt.Sprintf("%d%%  analytics", int(a.viewport.ScrollPercent()*100))
}

func analyticsViewportWidth(width int) int {
	return maxInt(1, width-4)
}

func analyticsViewportHeight(height int) int {
	if height < 6 {
		return maxInt(1, height-2)
	}
	return maxInt(3, height-8)
}

func (a *AnalyticsView) renderContent() {
	a.viewport.SetContent(a.renderBody())
}

func (a *AnalyticsView) renderBody() string {
	var sb strings.Builder

	if len(a.sessions) == 0 {
		return emptyState(a.width-4, "No analytics yet", "Start using agentic coding tools to populate session and token charts.")
	}

	agentCounts := make(map[string]int)
	agentTokens := make(map[string]int)
	projectCounts := make(map[string]int)
	totalTokens := 0

	for _, s := range a.sessions {
		if s == nil {
			continue
		}
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
	if sectionWidth < 40 {
		sectionWidth = 40
	}
	barWidth := clampInt(sectionWidth-46, 18, 54)
	insights := analyzeSessions(a.sessions)
	timeline := buildActivityTimeline(a.sessions)
	cacheUsage := summarizeCacheUsage(a.sessions)
	projectsByTokens := summarizeProjectsByTokens(a.sessions)
	stateBreakdown := sessionStateCounts(a.sessions)
	agentEfficiency := summarizeAgentEfficiency(insights.Threads)

	avgLatency := averageThreadLatency(insights.Threads)
	latencyValue := "—"
	if avgLatency > 0 {
		latencyValue = models.FormatDuration(avgLatency)
	}
	successValue := "—"
	successStyle := styleAccent
	if rate := toolSuccessRate(insights.Tools); rate >= 0 {
		successValue = fmt.Sprintf("%.0f%%", rate)
		switch {
		case rate < 50:
			successStyle = styleError
		case rate < 80:
			successStyle = styleWarning
		default:
			successStyle = styleSuccess
		}
	}

	cardWidth := clampInt((a.width-20)/4, 14, 22)
	statsRow := lipgloss.JoinHorizontal(lipgloss.Top,
		metricCardWidth("Sessions", fmt.Sprintf("%d", len(a.sessions)), "☷", styleAccent, cardWidth),
		"  ",
		metricCardWidth("Prompts", fmt.Sprintf("%d", len(insights.Threads)), "✎", styleInfo, cardWidth),
		"  ",
		metricCardWidth("Avg latency", latencyValue, "⏱", styleAccent, cardWidth),
		"  ",
		metricCardWidth("Tool success", successValue, "✓", successStyle, cardWidth),
	)
	tokenRow := lipgloss.JoinHorizontal(lipgloss.Top,
		metricCardWidth("Tokens", compactInt(totalTokens), "◇", styleAccent, cardWidth),
		"  ",
		metricCardWidth("Failures", fmt.Sprintf("%d", insights.TotalFailures), "⚠", styleError, cardWidth),
		"  ",
		metricCardWidth("Tools", fmt.Sprintf("%d", len(insights.Tools)), "⚙", styleInfo, cardWidth),
		"  ",
		metricCardWidth("Outliers", fmt.Sprintf("%d", len(insights.Outliers)), "◆", styleWarning, cardWidth),
	)
	analyticsSubtitle := "prompt insights"
	if sectionWidth >= 84 {
		analyticsSubtitle = "session distribution, prompt efficiency, and tool load"
	}
	sb.WriteString(sectionHeader("Observatory analytics", analyticsSubtitle, sectionWidth) + "\n\n")
	sb.WriteString(statsRow + "\n\n")
	sb.WriteString(tokenRow + "\n\n")
	if chipRow := renderSessionStateChips(stateBreakdown); chipRow != "" {
		sb.WriteString("  " + chipRow + "\n\n")
	}

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
	sb.WriteString(renderActivityTrend(sectionWidth, timeline) + "\n\n")
	sb.WriteString(renderPromptThreadSummary(sectionWidth, insights.Threads) + "\n\n")
	sb.WriteString(renderEfficiencyPanel(sectionWidth, insights.Threads, agentEfficiency) + "\n\n")
	sb.WriteString(renderPromptCategoryBreakdown(sectionWidth, insights.Categories) + "\n\n")
	if cachePanel := renderCacheUtilizationPanel(sectionWidth, cacheUsage); cachePanel != "" {
		sb.WriteString(cachePanel + "\n\n")
	}
	sb.WriteString(renderOutlierPanel(sectionWidth, insights.Outliers) + "\n\n")
	sb.WriteString(renderToolActivitySummary(sectionWidth, insights.Tools, insights.TotalFailures) + "\n\n")
	sb.WriteString(renderActionableHints(sectionWidth, insights.Hints) + "\n\n")

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
		if projs[i].count == projs[j].count {
			return projs[i].name < projs[j].name
		}
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
	if tokensPanel := renderProjectsByTokens(sectionWidth, projectsByTokens, 10); tokensPanel != "" {
		sb.WriteString(tokensPanel + "\n\n")
	}

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

func analyzeSessions(sessions []*models.Session) analyticsInsights {
	threads := buildPromptThreads(sessions)
	categories := summarizePromptCategories(threads)
	tools, failures := summarizeToolActivity(sessions)
	outliers := summarizeSessionOutliers(sessions)
	followUps := countPromptFollowUps(sessions)
	return analyticsInsights{
		Threads:       threads,
		Categories:    categories,
		Tools:         tools,
		Outliers:      outliers,
		Hints:         buildActionableHints(threads, categories, tools, outliers, followUps, failures),
		FollowUps:     followUps,
		TotalFailures: failures,
	}
}

func buildPromptThreads(sessions []*models.Session) []promptThread {
	var threads []promptThread
	for _, session := range sessions {
		if session == nil {
			continue
		}
		var current *promptThread
		for _, message := range session.Messages {
			if message.Role == "user" {
				if current != nil {
					threads = append(threads, finalizePromptThread(*current, session))
				}
				current = &promptThread{
					SessionID:    session.ID,
					Agent:        string(session.AgentType),
					Project:      session.ProjectPath,
					Prompt:       strings.TrimSpace(message.Content),
					Category:     classifyPrompt(message.Content),
					Start:        message.Timestamp,
					End:          message.Timestamp,
					MessageCount: 1,
					Tokens:       messageTokens(message),
					InputTokens:  message.Tokens.InputTokens,
					OutputTokens: message.Tokens.OutputTokens,
				}
				continue
			}
			if current == nil {
				continue
			}
			current.MessageCount++
			current.Tokens += messageTokens(message)
			current.InputTokens += message.Tokens.InputTokens
			current.OutputTokens += message.Tokens.OutputTokens
			if !message.Timestamp.IsZero() {
				current.End = message.Timestamp
			}
			if isToolActivity(message) {
				current.ToolEvents++
			}
			if isFailedActivity(message) {
				current.FailedEvents++
			}
			if current.FirstResponseLatency == 0 && message.Role == "assistant" &&
				!message.Timestamp.IsZero() && !current.Start.IsZero() &&
				message.Timestamp.After(current.Start) {
				current.FirstResponseLatency = message.Timestamp.Sub(current.Start)
			}
		}
		if current != nil {
			threads = append(threads, finalizePromptThread(*current, session))
		}
	}
	return threads
}

func finalizePromptThread(thread promptThread, session *models.Session) promptThread {
	if thread.Start.IsZero() {
		thread.Start = session.StartTime
	}
	if thread.End.IsZero() {
		thread.End = session.LastUpdated
	}
	if !thread.Start.IsZero() && !thread.End.IsZero() && thread.End.After(thread.Start) {
		thread.Duration = thread.End.Sub(thread.Start)
	}
	return thread
}

func summarizePromptCategories(threads []promptThread) []categorySummary {
	byCategory := make(map[string]*categorySummary)
	for _, thread := range threads {
		category := thread.Category
		if category == "" {
			category = "General"
		}
		summary := byCategory[category]
		if summary == nil {
			summary = &categorySummary{Name: category}
			byCategory[category] = summary
		}
		summary.Count++
		summary.Tokens += thread.Tokens
		summary.Duration += thread.Duration
		summary.Tools += thread.ToolEvents
		summary.Failures += thread.FailedEvents
		if thread.FirstResponseLatency > 0 {
			summary.LatencySum += thread.FirstResponseLatency
			summary.LatencySamples++
		}
	}

	var categories []categorySummary
	for _, name := range promptCategoryOrder {
		if summary := byCategory[name]; summary != nil {
			categories = append(categories, *summary)
			delete(byCategory, name)
		}
	}
	for _, summary := range byCategory {
		categories = append(categories, *summary)
	}
	sort.SliceStable(categories, func(i, j int) bool {
		if categories[i].Count == categories[j].Count {
			return categories[i].Name < categories[j].Name
		}
		return categories[i].Count > categories[j].Count
	})
	return categories
}

func summarizeToolActivity(sessions []*models.Session) ([]toolSummary, int) {
	byTool := make(map[string]*toolSummary)
	totalFailures := 0
	for _, session := range sessions {
		if session == nil {
			continue
		}
		for _, message := range session.Messages {
			if !isToolActivity(message) {
				continue
			}
			name := toolActivityName(message)
			summary := byTool[name]
			if summary == nil {
				summary = &toolSummary{Name: name}
				byTool[name] = summary
			}
			summary.Count++
			switch message.Meta.Lifecycle {
			case models.ActivityLifecycleStarted, models.ActivityLifecycleRequested:
				summary.Started++
			case models.ActivityLifecycleCompleted:
				summary.Completed++
			case models.ActivityLifecycleFailed:
				summary.Failed++
				totalFailures++
			}
		}
	}

	var tools []toolSummary
	for _, summary := range byTool {
		tools = append(tools, *summary)
	}
	sort.Slice(tools, func(i, j int) bool {
		if tools[i].Failed != tools[j].Failed {
			return tools[i].Failed > tools[j].Failed
		}
		if tools[i].Count == tools[j].Count {
			return tools[i].Name < tools[j].Name
		}
		return tools[i].Count > tools[j].Count
	})
	if len(tools) > 8 {
		tools = tools[:8]
	}
	return tools, totalFailures
}

func summarizeSessionOutliers(sessions []*models.Session) []sessionInsight {
	seen := make(map[string]bool)
	var outliers []sessionInsight
	add := func(session *models.Session, value int, label string) {
		if session == nil || value <= 0 {
			return
		}
		key := sessionOutlierKey(session) + ":" + label
		if seen[key] {
			return
		}
		seen[key] = true
		outliers = append(outliers, sessionInsight{Session: session, Value: value, Label: label})
	}

	if session, value := maxSessionBy(sessions, sessionTokenTotal); session != nil {
		add(session, value, "highest token load")
	}
	if session, value := maxSessionBy(sessions, func(s *models.Session) int { return int(s.Duration().Minutes()) }); session != nil {
		add(session, value, "longest duration")
	}
	if session, value := maxSessionBy(sessions, sessionToolEvents); session != nil {
		add(session, value, "most tool activity")
	}
	if session, value := maxSessionBy(sessions, sessionFailureEvents); session != nil {
		add(session, value, "most failed activity")
	}
	return outliers
}

func maxSessionBy(sessions []*models.Session, score func(*models.Session) int) (*models.Session, int) {
	var best *models.Session
	bestScore := 0
	for _, session := range sessions {
		if session == nil {
			continue
		}
		value := score(session)
		if value > bestScore {
			best = session
			bestScore = value
		}
	}
	return best, bestScore
}

func buildActionableHints(threads []promptThread, categories []categorySummary, tools []toolSummary, outliers []sessionInsight, followUps, failures int) []string {
	var hints []string
	if len(threads) == 0 {
		return []string{"Start prompts with clear goals and target files so future sessions can be compared."}
	}

	totalTools := 0
	totalTokens := 0
	for _, thread := range threads {
		totalTools += thread.ToolEvents
		totalTokens += thread.Tokens
	}
	avgTools := float64(totalTools) / float64(len(threads))
	if avgTools >= 3 {
		hints = append(hints, "Name target files or components up front to reduce exploratory tool fan-out.")
	}
	if followUps >= maxInt(2, len(threads)/3) {
		hints = append(hints, "Add acceptance criteria and constraints to reduce corrective follow-up prompts.")
	}
	if failures > 0 {
		hints = append(hints, "Include expected validation commands so agents can converge after failed tool activity.")
	}
	if len(outliers) > 0 && totalTokens > 0 && sessionTokenTotal(outliers[0].Session)*2 > totalTokens {
		hints = append(hints, "Split unusually expensive work into planning, implementation, and validation prompts.")
	}
	for _, category := range categories {
		if category.Name == "Debugging" && category.Count > 0 && category.Tokens*2 >= maxInt(1, totalTokens) {
			hints = append(hints, "For debugging prompts, provide the failing command, exact error, and recent change context together.")
			break
		}
	}
	if len(tools) > 0 && tools[0].Failed > 0 {
		hints = append(hints, fmt.Sprintf("Review repeated %s failures before retrying broad prompts.", tools[0].Name))
	}
	if len(hints) == 0 {
		hints = append(hints, "Prompt patterns look efficient; keep using scoped goals with explicit outcomes.")
	}
	if len(hints) > 5 {
		hints = hints[:5]
	}
	return hints
}

func renderPromptThreadSummary(width int, threads []promptThread) string {
	var b strings.Builder
	b.WriteString(sectionHeader("Prompt thread analytics", "per-user-prompt cost and flow", width-4) + "\n")
	if len(threads) == 0 {
		b.WriteString("  " + styleMuted.Render("No user prompt threads detected yet.") + "\n")
		return quietPanel(width, b.String())
	}

	totalTokens, totalTools, totalFailures := 0, 0, 0
	var totalDuration time.Duration
	for _, thread := range threads {
		totalTokens += thread.Tokens
		totalTools += thread.ToolEvents
		totalFailures += thread.FailedEvents
		totalDuration += thread.Duration
	}
	avgTokens := totalTokens / maxInt(1, len(threads))
	avgTools := float64(totalTools) / float64(len(threads))
	b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
		metricChip("Threads", fmt.Sprintf("%d", len(threads)), "◇", styleAccent),
		metricChip("Avg tokens", compactInt(avgTokens), "◈", styleInfo),
		metricChip("Avg tools", fmt.Sprintf("%.1f", avgTools), "⚙", styleWarning),
		metricChip("Failures", fmt.Sprintf("%d", totalFailures), "⚠", styleError),
	))
	if totalDuration > 0 {
		b.WriteString("  " + styleMuted.Render("Average thread duration: ") + styleText(models.FormatDuration(totalDuration/time.Duration(len(threads)))) + "\n")
	}
	if top := topPromptThreads(threads, 3); len(top) > 0 {
		b.WriteString("\n")
		for i, thread := range top {
			b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
				styleAccent.Render(fmt.Sprintf("%d.", i+1)),
				styleText(truncateEnd(thread.Category+" · "+threadPromptLabel(thread), 48)),
				styleMuted.Render(compactInt(thread.Tokens)+" tokens"),
				styleMuted.Render(fmt.Sprintf("%d tools", thread.ToolEvents)),
			))
		}
	}
	return quietPanel(width, b.String())
}

func renderPromptCategoryBreakdown(width int, categories []categorySummary) string {
	var b strings.Builder
	b.WriteString(sectionHeader("Prompt category breakdown", "local heuristic classification", width-4) + "\n")
	if len(categories) == 0 {
		b.WriteString("  " + styleMuted.Render("No categorized prompts yet.") + "\n")
		return quietPanel(width, b.String())
	}
	for _, category := range categories {
		avgTokens := category.Tokens / maxInt(1, category.Count)
		avgDuration := time.Duration(0)
		if category.Count > 0 {
			avgDuration = category.Duration / time.Duration(category.Count)
		}
		latencyLabel := "—"
		if category.LatencySamples > 0 {
			latencyLabel = models.FormatDuration(category.LatencySum / time.Duration(category.LatencySamples))
		}
		b.WriteString(fmt.Sprintf("  %s  %s  %s  %s  %s  %s\n",
			lipgloss.NewStyle().Width(16).Render(styleText(category.Name)),
			styleAccent.Render(fmt.Sprintf("%d prompts", category.Count)),
			styleMuted.Render(compactInt(avgTokens)+" avg tokens"),
			styleMuted.Render(models.FormatDuration(avgDuration)+" avg"),
			styleMuted.Render(latencyLabel+" lat"),
			styleMuted.Render(fmt.Sprintf("%d tools", category.Tools)),
		))
	}
	return quietPanel(width, b.String())
}

func renderOutlierPanel(width int, outliers []sessionInsight) string {
	var b strings.Builder
	b.WriteString(sectionHeader("Outlier sessions", "inspect costly, long, or noisy runs", width-4) + "\n")
	if len(outliers) == 0 {
		b.WriteString("  " + styleMuted.Render("No outlier signals yet.") + "\n")
		return quietPanel(width, b.String())
	}
	for _, outlier := range outliers {
		project := "unknown project"
		if outlier.Session.ProjectPath != "" {
			project = outlier.Session.ProjectPath
		}
		value := compactInt(outlier.Value)
		if strings.Contains(outlier.Label, "duration") {
			value = fmt.Sprintf("%dm", outlier.Value)
		}
		b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
			styleWarning.Render("◆"),
			lipgloss.NewStyle().Width(22).Render(styleText(outlier.Label)),
			styleMuted.Render(value),
			styleMuted.Render(truncateStart(project, 44)),
		))
	}
	return quietPanel(width, b.String())
}

func renderToolActivitySummary(width int, tools []toolSummary, failures int) string {
	var b strings.Builder
	b.WriteString(sectionHeader("Tool activity summary", "tool lifecycle volume and failures", width-4) + "\n")
	if len(tools) == 0 {
		b.WriteString("  " + styleMuted.Render("No tool activity detected yet.") + "\n")
		return quietPanel(width, b.String())
	}
	b.WriteString("  " + metricChip("Failed events", fmt.Sprintf("%d", failures), "⚠", styleError) + "\n\n")
	for _, tool := range tools {
		finished := tool.Completed + tool.Failed
		successLabel := styleMuted.Render("— success")
		if finished > 0 {
			pct := float64(tool.Completed) * 100.0 / float64(finished)
			style := styleSuccess
			if pct < 50 {
				style = styleError
			} else if pct < 80 {
				style = styleWarning
			}
			successLabel = style.Render(fmt.Sprintf("%.0f%% success", pct))
		}
		b.WriteString(fmt.Sprintf("  %s  %s  %s  %s  %s  %s\n",
			lipgloss.NewStyle().Width(24).Render(styleText(truncateEnd(tool.Name, 24))),
			styleAccent.Render(fmt.Sprintf("%d events", tool.Count)),
			styleMuted.Render(fmt.Sprintf("%d start", tool.Started)),
			styleMuted.Render(fmt.Sprintf("%d done", tool.Completed)),
			failureLabel(tool.Failed),
			successLabel,
		))
	}
	return quietPanel(width, b.String())
}

func renderActionableHints(width int, hints []string) string {
	var b strings.Builder
	b.WriteString(sectionHeader("Prompt refinement hints", "derived from local session patterns", width-4) + "\n")
	for _, hint := range hints {
		b.WriteString("  " + styleSuccess.Render("→") + " " + styleText(truncateEnd(hint, width-12)) + "\n")
	}
	return quietPanel(width, b.String())
}

func topPromptThreads(threads []promptThread, limit int) []promptThread {
	out := append([]promptThread(nil), threads...)
	sort.Slice(out, func(i, j int) bool {
		left := out[i].Tokens + out[i].ToolEvents*100 + int(out[i].Duration.Minutes())
		right := out[j].Tokens + out[j].ToolEvents*100 + int(out[j].Duration.Minutes())
		if left == right {
			return out[i].Prompt < out[j].Prompt
		}
		return left > right
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func classifyPrompt(prompt string) string {
	lower := strings.ToLower(prompt)
	switch {
	case containsAny(lower, "fix", "failing", "failed", "error", "bug", "debug", "test", "ci", "build failure"):
		return "Debugging"
	case containsAny(lower, "plan", "approach", "design", "how would", "outline", "strategy"):
		return "Planning"
	case containsAny(lower, "implement", "add", "build", "create", "wire", "support"):
		return "Implementation"
	case containsAny(lower, "find", "where", "explain", "understand", "inspect", "investigate"):
		return "Exploration"
	case containsAny(lower, "refactor", "clean up", "simplify", "rename", "restructure"):
		return "Refactor"
	case containsAny(lower, "review", "audit", "security", "scan", "check"):
		return "Review"
	default:
		return "General"
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func messageTokens(message models.Message) int {
	return message.Tokens.InputTokens + message.Tokens.OutputTokens
}

func isToolActivity(message models.Message) bool {
	return message.Role == "tool" || message.Meta.Kind == models.ActivityKindTool
}

func isFailedActivity(message models.Message) bool {
	return message.Meta.Lifecycle == models.ActivityLifecycleFailed
}

func toolActivityName(message models.Message) string {
	if strings.TrimSpace(message.Meta.Label) != "" {
		return strings.TrimSpace(message.Meta.Label)
	}
	content := strings.TrimSpace(message.Content)
	if content == "" {
		return "tool"
	}
	line := strings.Split(content, "\n")[0]
	for _, sep := range []string{":", "—", "-"} {
		if idx := strings.Index(line, sep); idx > 0 {
			line = strings.TrimSpace(line[:idx])
			break
		}
	}
	if line == "" {
		return "tool"
	}
	return truncateEnd(line, 32)
}

func sessionToolEvents(session *models.Session) int {
	count := 0
	for _, message := range session.Messages {
		if isToolActivity(message) {
			count++
		}
	}
	return count
}

func sessionFailureEvents(session *models.Session) int {
	count := 0
	for _, message := range session.Messages {
		if isFailedActivity(message) {
			count++
		}
	}
	return count
}

func sessionTokenTotal(session *models.Session) int {
	if session == nil {
		return 0
	}
	return session.TotalInputTokens() + session.TotalOutputTokens()
}

func sessionOutlierKey(session *models.Session) string {
	if session.ID != "" {
		return session.ID
	}
	if session.LogPath != "" {
		return session.LogPath
	}
	return fmt.Sprintf("%s:%s:%s", session.AgentType, session.ProjectPath, session.StartTime)
}

func countPromptFollowUps(sessions []*models.Session) int {
	followUps := 0
	for _, session := range sessions {
		if session == nil {
			continue
		}
		seenPrompt := false
		for _, message := range session.Messages {
			if message.Role != "user" {
				continue
			}
			if seenPrompt {
				followUps++
			}
			seenPrompt = true
		}
	}
	return followUps
}

func threadPromptLabel(thread promptThread) string {
	if thread.Prompt != "" {
		return thread.Prompt
	}
	if thread.Project != "" {
		return thread.Project
	}
	return thread.Agent
}

func failureLabel(failed int) string {
	if failed > 0 {
		return styleError.Render(fmt.Sprintf("%d failed", failed))
	}
	return styleMuted.Render("0 failed")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
