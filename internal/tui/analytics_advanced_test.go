package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

func TestBuildActivityTimelineAggregatesDailyAndHourly(t *testing.T) {
	end := time.Date(2026, 4, 29, 14, 0, 0, 0, time.UTC)
	earlier := end.AddDate(0, 0, -3)
	sessions := []*models.Session{
		{
			AgentType:   models.AgentClaude,
			ProjectPath: "/repo/app",
			StartTime:   earlier,
			LastUpdated: earlier.Add(time.Hour),
			TotalTokens: models.TokenUsage{InputTokens: 100, OutputTokens: 50},
			Messages: []models.Message{
				{Role: "user", Content: "hello", Timestamp: earlier},
				{Role: "assistant", Content: "hi", Timestamp: earlier.Add(time.Minute)},
			},
		},
		{
			AgentType:   models.AgentCopilot,
			ProjectPath: "/repo/app",
			StartTime:   end,
			LastUpdated: end.Add(time.Hour),
			TotalTokens: models.TokenUsage{InputTokens: 200, OutputTokens: 100},
			Messages: []models.Message{
				{Role: "user", Content: "fix it", Timestamp: end},
				{Role: "assistant", Content: "done", Timestamp: end.Add(time.Minute)},
			},
		},
	}

	timeline := buildActivityTimeline(sessions)
	if !timeline.HasData {
		t.Fatalf("expected timeline to report data")
	}
	if len(timeline.Days) != activityTrendDays {
		t.Fatalf("expected %d daily buckets, got %d", activityTrendDays, len(timeline.Days))
	}
	last := timeline.Days[len(timeline.Days)-1]
	if last.Sessions != 1 || last.Prompts != 1 || last.Tokens != 300 {
		t.Fatalf("unexpected last day bucket: %+v", last)
	}
	earlierBucket := timeline.Days[len(timeline.Days)-4]
	if earlierBucket.Sessions != 1 || earlierBucket.Prompts != 1 || earlierBucket.Tokens != 150 {
		t.Fatalf("unexpected earlier bucket: %+v", earlierBucket)
	}
	if timeline.Hours[14] == 0 {
		t.Fatalf("expected non-zero count for hour 14, got %v", timeline.Hours)
	}
}

func TestSummarizeCacheUsageOnlyIncludesAgentsWithCache(t *testing.T) {
	sessions := []*models.Session{
		{
			AgentType:   models.AgentClaude,
			TotalTokens: models.TokenUsage{InputTokens: 100, OutputTokens: 50},
		},
		{
			AgentType:   models.AgentCopilot,
			TotalTokens: models.TokenUsage{InputTokens: 200, OutputTokens: 80, CacheReads: 1000, CacheWrites: 200},
		},
		{
			AgentType:   models.AgentCopilot,
			TotalTokens: models.TokenUsage{InputTokens: 150, OutputTokens: 60, CacheReads: 500, CacheWrites: 100},
		},
	}
	entries := summarizeCacheUsage(sessions)
	if len(entries) != 1 {
		t.Fatalf("expected 1 cache entry, got %d", len(entries))
	}
	if entries[0].Agent != string(models.AgentCopilot) {
		t.Fatalf("expected Copilot CLI entry, got %q", entries[0].Agent)
	}
	if entries[0].Reads != 1500 || entries[0].Writes != 300 {
		t.Fatalf("expected aggregated cache totals, got %+v", entries[0])
	}
}

func TestSummarizeProjectsByTokensRanksByVolume(t *testing.T) {
	sessions := []*models.Session{
		{ProjectPath: "/a", TotalTokens: models.TokenUsage{InputTokens: 100, OutputTokens: 50}},
		{ProjectPath: "/a", TotalTokens: models.TokenUsage{InputTokens: 100, OutputTokens: 50}},
		{ProjectPath: "/b", TotalTokens: models.TokenUsage{InputTokens: 1000, OutputTokens: 500}},
	}
	out := summarizeProjectsByTokens(sessions)
	if len(out) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(out))
	}
	if out[0].Path != "/b" || out[0].Tokens != 1500 {
		t.Fatalf("expected /b first with 1500 tokens, got %+v", out[0])
	}
	if out[1].Path != "/a" || out[1].Sessions != 2 {
		t.Fatalf("expected /a second with 2 sessions, got %+v", out[1])
	}
}

func TestSessionStateCountsBucketsByStatus(t *testing.T) {
	sessions := []*models.Session{
		{IsActive: true},
		{IsActive: false},
		{IsActive: false, Messages: []models.Message{
			{Meta: models.ActivityMeta{Kind: models.ActivityKindSession, Lifecycle: models.ActivityLifecycleFailed}},
		}},
	}
	b := sessionStateCounts(sessions)
	if b.Total != 3 || b.Active != 1 || b.Idle != 1 || b.Failed != 1 {
		t.Fatalf("unexpected breakdown: %+v", b)
	}
}

func TestAverageThreadLatencyIgnoresZeros(t *testing.T) {
	threads := []promptThread{
		{FirstResponseLatency: 4 * time.Second},
		{FirstResponseLatency: 2 * time.Second},
		{FirstResponseLatency: 0},
	}
	got := averageThreadLatency(threads)
	if got != 3*time.Second {
		t.Fatalf("expected 3s, got %s", got)
	}
}

func TestToolSuccessRate(t *testing.T) {
	if toolSuccessRate(nil) != -1 {
		t.Fatalf("expected -1 for empty input")
	}
	tools := []toolSummary{
		{Completed: 8, Failed: 2},
		{Completed: 0, Failed: 0},
	}
	if got := toolSuccessRate(tools); got != 80 {
		t.Fatalf("expected 80%%, got %v", got)
	}
}

func TestSparklineRendersExpectedLength(t *testing.T) {
	out := sparkline([]int{0, 1, 2, 3, 4, 5})
	if len([]rune(out)) != 6 {
		t.Fatalf("expected 6 runes, got %d (%q)", len([]rune(out)), out)
	}
	allZero := sparkline([]int{0, 0, 0, 0})
	if len([]rune(allZero)) != 4 {
		t.Fatalf("expected 4 runes for all-zero input, got %d", len([]rune(allZero)))
	}
}

func TestSummarizeAgentEfficiencyComputesRatio(t *testing.T) {
	threads := []promptThread{
		{Agent: "Claude Code", InputTokens: 100, OutputTokens: 50},
		{Agent: "Claude Code", InputTokens: 100, OutputTokens: 50},
		{Agent: "Codex CLI", InputTokens: 0, OutputTokens: 30},
	}
	out := summarizeAgentEfficiency(threads)
	if len(out) != 2 {
		t.Fatalf("expected 2 agent rows, got %d", len(out))
	}
	var claude, codex *efficiencySummary
	for i := range out {
		switch out[i].Agent {
		case "Claude Code":
			claude = &out[i]
		case "Codex CLI":
			codex = &out[i]
		}
	}
	if claude == nil || claude.Prompts != 2 || claude.IORatio != 0.5 || claude.TokensPerPrompt != 150 {
		t.Fatalf("unexpected Claude entry: %+v", claude)
	}
	if codex == nil || codex.IORatio != 0 {
		t.Fatalf("expected Codex IORatio 0 when no input, got %+v", codex)
	}
}

func TestBuildPromptThreadsRecordsFirstResponseLatency(t *testing.T) {
	start := time.Date(2026, 4, 29, 1, 0, 0, 0, time.UTC)
	session := &models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "Implement feature", Timestamp: start},
			{Role: "tool", Content: "edit", Timestamp: start.Add(2 * time.Second), Meta: models.ActivityMeta{Kind: models.ActivityKindTool}},
			{Role: "assistant", Content: "done", Timestamp: start.Add(5 * time.Second)},
		},
	}
	threads := buildPromptThreads([]*models.Session{session})
	if len(threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(threads))
	}
	if threads[0].FirstResponseLatency != 5*time.Second {
		t.Fatalf("expected 5s latency, got %s", threads[0].FirstResponseLatency)
	}
}

func TestAnalyticsRendersAdvancedSections(t *testing.T) {
	view := NewAnalyticsView(120, 200)
	view.SetSessions([]*models.Session{analyticsTestSession()})
	rendered := view.View()
	for _, want := range []string{
		"Activity trend",
		"Latency & token efficiency",
		"Top projects by token volume",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected analytics view to include %q, got:\n%s", want, rendered)
		}
	}
}
