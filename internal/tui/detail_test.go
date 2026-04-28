package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

func TestDetailToggleSelectedThreadCollapsesRelatedAssistants(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "first prompt"},
			{Role: "assistant", Content: "first response"},
			{Role: "assistant", Content: "second response"},
			{Role: "user", Content: "second prompt"},
			{Role: "assistant", Content: "third response"},
		},
	})

	detail.ToggleSelectedThread()
	view := detail.View()

	if strings.Contains(view, "first response") || strings.Contains(view, "second response") {
		t.Fatalf("expected selected user's assistant responses to be collapsed, got:\n%s", view)
	}
	if !strings.Contains(view, "2 activity entries folded") {
		t.Fatalf("expected collapsed activity summary, got:\n%s", view)
	}
	if !strings.Contains(view, "third response") {
		t.Fatalf("expected following user thread to remain expanded, got:\n%s", view)
	}
}

func TestDetailSelectNextUserAndCollapse(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "first prompt"},
			{Role: "assistant", Content: "first response"},
			{Role: "user", Content: "second prompt"},
			{Role: "assistant", Content: "second response"},
		},
	})

	detail.SelectNextUser()
	detail.ToggleSelectedThread()
	view := detail.View()

	if !strings.Contains(view, "first response") {
		t.Fatalf("expected first thread to remain expanded, got:\n%s", view)
	}
	if strings.Contains(view, "second response") {
		t.Fatalf("expected second thread assistant response to be collapsed, got:\n%s", view)
	}
}

func TestDetailCollapseAllThreads(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "first prompt"},
			{Role: "assistant", Content: "first response"},
			{Role: "user", Content: "second prompt"},
			{Role: "assistant", Content: "second response"},
			{Role: "assistant", Content: "third response"},
		},
	})

	detail.CollapseAllThreads()
	view := detail.View()

	if strings.Contains(view, "first response") || strings.Contains(view, "second response") || strings.Contains(view, "third response") {
		t.Fatalf("expected all assistant responses to be collapsed, got:\n%s", view)
	}
	if strings.Count(view, "activity entries folded") != 2 {
		t.Fatalf("expected both user threads to show collapsed summaries, got:\n%s", view)
	}
}

func TestDetailToggleAllThreadsCollapsed(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "first prompt"},
			{Role: "assistant", Content: "first response"},
			{Role: "user", Content: "second prompt"},
			{Role: "assistant", Content: "second response"},
		},
	})

	detail.ToggleAllThreadsCollapsed()
	collapsed := detail.View()
	if strings.Contains(collapsed, "first response") || strings.Contains(collapsed, "second response") {
		t.Fatalf("expected toggle to collapse all threads, got:\n%s", collapsed)
	}
	if strings.Count(collapsed, "activity entries folded") != 2 {
		t.Fatalf("expected all collapsible threads to be folded, got:\n%s", collapsed)
	}

	detail.ToggleAllThreadsCollapsed()
	expanded := detail.View()
	if !strings.Contains(expanded, "first response") || !strings.Contains(expanded, "second response") {
		t.Fatalf("expected second toggle to expand all threads, got:\n%s", expanded)
	}
	if strings.Contains(expanded, "activity entries folded") {
		t.Fatalf("expected expanded threads to hide folded summaries, got:\n%s", expanded)
	}
}

func TestDetailTimelineDetailLevelCyclesThroughThreeModes(t *testing.T) {
	startedAt := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	detail := NewDetailView(140, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt", Timestamp: startedAt, Tokens: models.TokenUsage{InputTokens: 100}},
			{Role: "assistant", Content: "assistant response", Timestamp: startedAt.Add(time.Second), Tokens: models.TokenUsage{OutputTokens: 200, CacheReads: 300}},
			{
				Role:      "tool",
				Content:   "Started tool: bash",
				Timestamp: startedAt.Add(2 * time.Second),
				Meta: models.ActivityMeta{
					Kind:          models.ActivityKindTool,
					Lifecycle:     models.ActivityLifecycleStarted,
					ID:            "tool-1",
					ParentID:      "prompt-1",
					InteractionID: "turn-1",
					Label:         "bash",
				},
			},
			{
				Role:      "tool",
				Content:   "Tool completed: bash\ntelemetry: resultLength:12",
				Timestamp: startedAt.Add(5 * time.Second),
				Meta: models.ActivityMeta{
					Kind:          models.ActivityKindTool,
					Lifecycle:     models.ActivityLifecycleCompleted,
					ID:            "tool-1",
					ParentID:      "prompt-1",
					InteractionID: "turn-1",
					Label:         "bash",
				},
			},
		},
	})

	standard := detail.View()
	if !strings.Contains(standard, "detail standard") || !strings.Contains(standard, "telemetry: resultLength:12") {
		t.Fatalf("expected standard detail to show action detail, got:\n%s", standard)
	}
	if strings.Contains(standard, "12:00:") || !strings.Contains(standard, "time off") {
		t.Fatalf("expected timestamps to be hidden by default, got:\n%s", standard)
	}

	detail.ToggleTimelineDetailLevel()
	expanded := detail.View()
	for _, want := range []string{
		"detail expanded",
		"role assistant",
		"content assistant response",
		"tokens total:500 input:0 output:200 cache-read:300 cache-write:0",
		"state done · duration 3s",
		"kind tool · lifecycle started · label bash · id tool-1 · parent prompt-1 · interaction turn-1",
		"end Tool completed: bash · telemetry: resultLength:12",
	} {
		if !strings.Contains(expanded, want) {
			t.Fatalf("expected expanded detail to show %q, got:\n%s", want, expanded)
		}
	}
	if strings.Contains(expanded, "2026-04-28 12:00:") || strings.Contains(expanded, "12:00:01") {
		t.Fatalf("expected expanded detail to hide timestamps while timestamp toggle is off, got:\n%s", expanded)
	}

	detail.ToggleTimestamps()
	expandedWithTime := detail.View()
	for _, want := range []string{
		"time on",
		"role assistant · time 2026-04-28 12:00:01",
		"state done · started 2026-04-28 12:00:02 · ended 2026-04-28 12:00:05 · duration 3s",
	} {
		if !strings.Contains(expandedWithTime, want) {
			t.Fatalf("expected timestamp-enabled expanded detail to show %q, got:\n%s", want, expandedWithTime)
		}
	}
	detail.ToggleTimestamps()

	detail.ToggleTimelineDetailLevel()
	compact := detail.View()
	if !strings.Contains(compact, "detail compact") {
		t.Fatalf("expected compact detail label, got:\n%s", compact)
	}
	if strings.Contains(compact, "telemetry: resultLength:12") || strings.Contains(compact, "tokens total:500") {
		t.Fatalf("expected compact detail to suppress secondary detail lines, got:\n%s", compact)
	}

	detail.ToggleTimelineDetailLevel()
	if view := detail.View(); !strings.Contains(view, "detail standard") {
		t.Fatalf("expected detail level cycle to return to standard, got:\n%s", view)
	}
}

func TestDetailSelectNextRowNavigatesActivityRows(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "assistant", Content: "assistant response"},
			{Role: "tool", Content: "tool output"},
		},
	})

	detail.SelectNextRow()

	row, ok := detail.selectedActivityRow()
	if !ok {
		t.Fatalf("expected selected row")
	}
	if row.messageIndex != 1 {
		t.Fatalf("expected down to select assistant activity row, got message index %d", row.messageIndex)
	}
	if detail.selectedUser != 0 {
		t.Fatalf("expected selected activity to remain associated with first user prompt, got %d", detail.selectedUser)
	}
}

func TestDetailPromptJumpSelectsUserRows(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "first prompt"},
			{Role: "assistant", Content: "first response"},
			{Role: "tool", Content: "tool output"},
			{Role: "user", Content: "second prompt"},
			{Role: "assistant", Content: "second response"},
		},
	})
	detail.SelectNextRow()

	detail.SelectNextUser()

	row, ok := detail.selectedActivityRow()
	if !ok {
		t.Fatalf("expected selected row")
	}
	if row.messageIndex != 3 {
		t.Fatalf("expected prompt jump to select second user prompt, got message index %d", row.messageIndex)
	}
}

func TestDetailUserRowsShowThreadTokenLoadIndicators(t *testing.T) {
	detail := NewDetailView(160, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "small prompt", Tokens: models.TokenUsage{InputTokens: 500}},
			{Role: "assistant", Content: "small response", Tokens: models.TokenUsage{OutputTokens: 2_500}},
			{Role: "user", Content: "medium prompt", Tokens: models.TokenUsage{InputTokens: 8_000}},
			{Role: "assistant", Content: "medium response", Tokens: models.TokenUsage{OutputTokens: 7_000}},
			{Role: "user", Content: "large prompt", Tokens: models.TokenUsage{InputTokens: 20_000}},
			{Role: "assistant", Content: "large response", Tokens: models.TokenUsage{OutputTokens: 20_000}},
			{Role: "user", Content: "extreme prompt", Tokens: models.TokenUsage{InputTokens: 50_000}},
			{Role: "assistant", Content: "extreme response", Tokens: models.TokenUsage{OutputTokens: 20_000}},
		},
	})

	view := detail.View()
	for _, want := range []string{
		"◌ 3.0k",
		"◐ 15.0k",
		"◆ 40.0k",
		"⚠ 70.0k",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected thread token indicator %q, got:\n%s", want, view)
		}
	}
	if strings.Contains(view, "tok ") ||
		strings.Contains(view, "efficient") ||
		strings.Contains(view, "kinda high") ||
		strings.Contains(view, "way high") {
		t.Fatalf("expected minimal icon + number token indicators without old labels, got:\n%s", view)
	}
}

func TestDetailTimelineOmitsLeftMessageNumbers(t *testing.T) {
	detail := NewDetailView(140, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "alpha prompt"},
			toolLifecycleMessage("Started tool: bash", models.ActivityLifecycleStarted, "tool-1"),
			toolLifecycleMessage("Tool completed: bash", models.ActivityLifecycleCompleted, "tool-1"),
			{Role: "assistant", Content: "omega response"},
		},
	})

	view := detail.View()
	for _, removed := range []string{"001", "002", "003", "004"} {
		if strings.Contains(view, removed) {
			t.Fatalf("expected detail timeline to omit left message number %q, got:\n%s", removed, view)
		}
	}
}

func TestThreadTokenUsageIncludesCacheTokens(t *testing.T) {
	messages := []models.Message{
		{Role: "user", Content: "prompt", Tokens: models.TokenUsage{InputTokens: 100}},
		{Role: "assistant", Content: "response", Tokens: models.TokenUsage{OutputTokens: 200, CacheReads: 300, CacheWrites: 400}},
	}

	tokens := threadTokenUsage(messages, 0)
	if got := tokenUsageTotal(tokens); got != 1_000 {
		t.Fatalf("expected thread token total to include cache reads/writes, got %d", got)
	}
	if spec := tokenLoadIndicator(9_999); spec.Level != "optimal" {
		t.Fatalf("expected optimal indicator below 10k, got %#v", spec)
	}
	if spec := tokenLoadIndicator(10_000); spec.Level != "moderate" {
		t.Fatalf("expected moderate indicator at 10k, got %#v", spec)
	}
	if spec := tokenLoadIndicator(30_000); spec.Level != "elevated" {
		t.Fatalf("expected elevated indicator at 30k, got %#v", spec)
	}
	if spec := tokenLoadIndicator(60_000); spec.Level != "critical" {
		t.Fatalf("expected critical indicator at 60k, got %#v", spec)
	}
}

func TestDetailOpenSelectedDetailShowsSingleEvent(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		ProjectPath: "/repo/project",
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "tool", Content: "ran tests\nall passed"},
			{Role: "assistant", Content: "done"},
		},
	})
	detail.SelectNextRow()

	if !detail.OpenSelectedDetail() {
		t.Fatalf("expected selected tool activity to open")
	}
	view := detail.ThreadView()

	if !strings.Contains(view, "Focused activity 2") || !strings.Contains(view, "ran tests") {
		t.Fatalf("expected focused event detail, got:\n%s", view)
	}
	if strings.Contains(view, "prompt") || strings.Contains(view, "done") {
		t.Fatalf("expected focused event detail to exclude surrounding thread activity, got:\n%s", view)
	}
}

func TestDetailGroupsToolStartAndCompletion(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			toolLifecycleMessage("Started tool: bash", models.ActivityLifecycleStarted, "tool-1"),
			{Role: "assistant", Content: "assistant between action events"},
			toolLifecycleMessage("Tool completed: bash", models.ActivityLifecycleCompleted, "tool-1"),
		},
	})

	if len(detail.rows) != 3 {
		t.Fatalf("expected user, grouped action, and assistant rows, got %d: %#v", len(detail.rows), detail.rows)
	}
	group := detail.rows[1]
	if group.kind != activityRowActionGroup || group.messageIndex != 1 || group.endMessageIndex != 3 {
		t.Fatalf("expected tool start/completion group, got %#v", group)
	}

	view := detail.View()
	groupPos := strings.Index(view, "bash · done")
	betweenPos := strings.Index(view, "assistant between action events")
	if groupPos < 0 || betweenPos < 0 {
		t.Fatalf("expected grouped and surrounding activity, got:\n%s", view)
	}
	if strings.Contains(view, "Started tool: bash") || strings.Contains(view, "Tool completed: bash") {
		t.Fatalf("expected lifecycle row to avoid duplicate raw start/complete wording, got:\n%s", view)
	}
	if !(groupPos < betweenPos) {
		t.Fatalf("expected lifecycle summary before intervening activity, got:\n%s", view)
	}
}

func TestDetailTimelineAddsBreathingRoomBetweenRows(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "assistant", Content: "assistant response"},
			{Role: "tool", Content: "tool output"},
		},
	})

	if got := detail.rowLineOffsets[1]; got != 1 {
		t.Fatalf("expected second activity to stay inside prompt container, got row offset %d", got)
	}
	if got := detail.rowLineOffsets[2]; got != 2 {
		t.Fatalf("expected third activity to stay inside assistant container, got row offset %d", got)
	}
}

func TestDetailTimelineRendersNestedContainers(t *testing.T) {
	detail := NewDetailView(140, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "assistant", Content: "assistant activity"},
			toolLifecycleMessage("Started tool: bash", models.ActivityLifecycleStarted, "tool-1"),
			toolLifecycleMessage("Tool completed: bash", models.ActivityLifecycleCompleted, "tool-1"),
		},
	})

	view := detail.View()
	for _, want := range []string{
		"⠋ ASSIST",
		"⚙ TOOL",
		"╰─",
		"bash · done",
		"assistant activity",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected nested container marker %q, got:\n%s", want, view)
		}
	}
	if strings.Contains(view, "--:--:--") {
		t.Fatalf("expected timestamps hidden by default in nested timeline, got:\n%s", view)
	}
}

func TestDetailTimelineUsesTighterTreeIndentation(t *testing.T) {
	detail := NewDetailView(140, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "assistant", Content: "assistant activity"},
			toolLifecycleMessage("Started tool: bash", models.ActivityLifecycleStarted, "tool-1"),
			toolLifecycleMessage("Tool completed: bash", models.ActivityLifecycleCompleted, "tool-1"),
		},
	})

	treeView := renderTimelineTree(detail.timelineTree(0, len(detail.rows)))
	assistantLine := lineContaining(treeView, "ASSIST")
	toolLine := lineContaining(treeView, "TOOL")
	if assistantLine == "" || toolLine == "" {
		t.Fatalf("expected assistant and tool tree lines, got:\n%s", treeView)
	}
	if strings.HasPrefix(assistantLine, "    ") {
		t.Fatalf("expected assistant line to be only lightly indented, got %q in:\n%s", assistantLine, treeView)
	}
	if !strings.HasPrefix(toolLine, "  ") || strings.HasPrefix(toolLine, "      ") {
		t.Fatalf("expected tool line to be deeper than assistant but not over-indented, got %q in:\n%s", toolLine, treeView)
	}
}

func TestDetailGroupedActionStaysVisuallyTight(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			toolLifecycleMessage("Started tool: bash", models.ActivityLifecycleStarted, "tool-1"),
			toolLifecycleMessage("Tool completed: bash", models.ActivityLifecycleCompleted, "tool-1"),
			{Role: "assistant", Content: "done"},
		},
	})

	group := detail.rows[1]
	rendered := detail.renderActionGroupRow(group)
	if strings.Contains(rendered, "\n") {
		t.Fatalf("expected grouped start/completion without extra detail to fit on one line, got:\n%s", rendered)
	}
	if got := detail.rowLineOffsets[2] - detail.rowLineOffsets[1]; got != 1 {
		t.Fatalf("expected lifecycle row to stay inside prompt container before next group, got offset delta %d", got)
	}
}

func TestDetailLifecycleRowShowsUsefulCompletionDetail(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			toolLifecycleMessage("Started tool: bash", models.ActivityLifecycleStarted, "tool-1"),
			toolLifecycleMessage("Tool completed: bash\nmodel: gpt-5.5\ntelemetry: resultLength:12", models.ActivityLifecycleCompleted, "tool-1"),
			{Role: "assistant", Content: "done"},
		},
	})

	view := detail.View()
	if !strings.Contains(view, "bash · done") || !strings.Contains(view, "telemetry: resultLength:12") {
		t.Fatalf("expected compact lifecycle summary with useful telemetry detail, got:\n%s", view)
	}
	if strings.Contains(view, "model: gpt-5.5") || strings.Contains(view, "Tool completed: bash") {
		t.Fatalf("expected lifecycle row to suppress low-signal duplicate detail, got:\n%s", view)
	}
	if got := detail.rowLineOffsets[2] - detail.rowLineOffsets[1]; got != 2 {
		t.Fatalf("expected lifecycle detail line inside prompt container before next group, got offset delta %d", got)
	}
}

func TestDetailLifecycleRowShowsStartTimeWithDurationAtEnd(t *testing.T) {
	startedAt := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{
				Role:      "tool",
				Content:   "Started tool: bash",
				Timestamp: startedAt,
				Meta: models.ActivityMeta{
					Kind:      models.ActivityKindTool,
					Lifecycle: models.ActivityLifecycleStarted,
					ID:        "tool-1",
					Label:     "bash",
				},
			},
			{
				Role:      "tool",
				Content:   "Tool completed: bash",
				Timestamp: startedAt.Add(5 * time.Second),
				Meta: models.ActivityMeta{
					Kind:      models.ActivityKindTool,
					Lifecycle: models.ActivityLifecycleCompleted,
					ID:        "tool-1",
					Label:     "bash",
				},
			},
		},
	})

	detail.ToggleTimestamps()
	row := detail.renderActionGroupRow(detail.rows[1])
	if !strings.Contains(row, "12:00:00") {
		t.Fatalf("expected lifecycle row to show start time, got:\n%s", row)
	}
	if strings.Contains(row, "12:00:05") || strings.Contains(row, "→") {
		t.Fatalf("expected lifecycle row to omit end timestamp/range arrow, got:\n%s", row)
	}
	if durationPos, summaryPos := strings.LastIndex(row, "5s"), strings.Index(row, "bash · done"); durationPos < 0 || durationPos < summaryPos {
		t.Fatalf("expected duration at the end of the lifecycle row, got:\n%s", row)
	}
}

func TestActionLifecycleDurationShowsElapsedForRunningActions(t *testing.T) {
	startedAt := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	start := models.Message{Timestamp: startedAt}

	if got := actionLifecycleDuration(start, models.Message{}, false, startedAt.Add(90*time.Second)); got != "1m30s" {
		t.Fatalf("expected running lifecycle duration from start to now, got %q", got)
	}
}

func TestDetailShowsRunningToolLifecycle(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			toolLifecycleMessage("Started tool: bash", models.ActivityLifecycleStarted, "tool-1"),
		},
	})

	view := detail.View()
	row := detail.renderActionGroupRow(detail.rows[1])
	if !strings.Contains(row, "⠋") || !strings.Contains(row, "⚙ TOOL") || !strings.Contains(row, "bash · running") {
		t.Fatalf("expected running lifecycle row, got:\n%s", view)
	}
	if strings.Contains(view, "→") {
		t.Fatalf("expected running lifecycle row to show only the start time, got:\n%s", view)
	}
	if strings.Contains(view, "Started tool: bash") {
		t.Fatalf("expected running lifecycle row to avoid raw start wording, got:\n%s", view)
	}
	detail.AdvanceAnimation()
	animatedRow := detail.renderActionGroupRow(detail.rows[1])
	if !strings.Contains(animatedRow, "⠙") {
		t.Fatalf("expected running lifecycle icon to animate, got:\n%s", animatedRow)
	}
}

func TestDetailShowsRequestedToolLifecycle(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			toolLifecycleMessage("User requested tool: bash", models.ActivityLifecycleRequested, "tool-1"),
		},
	})

	view := detail.View()
	row := detail.renderActionGroupRow(detail.rows[1])
	if !strings.Contains(row, "◇") || !strings.Contains(row, "⚙ TOOL") || !strings.Contains(view, "bash · requested") {
		t.Fatalf("expected requested lifecycle indicator, got:\n%s", view)
	}
}

func TestDetailShowsFailedToolLifecycleDetail(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			toolLifecycleMessage("Started tool: bash", models.ActivityLifecycleStarted, "tool-1"),
			toolLifecycleMessage("Tool failed: bash\nerror: permission denied", models.ActivityLifecycleFailed, "tool-1"),
		},
	})

	view := detail.View()
	if !strings.Contains(view, "⚠") || !strings.Contains(view, "bash · failed") || !strings.Contains(view, "permission denied") {
		t.Fatalf("expected failed lifecycle row with error detail, got:\n%s", view)
	}
	if strings.Contains(view, "Tool failed: bash") || strings.Contains(view, "error: permission denied") {
		t.Fatalf("expected failed lifecycle row to suppress duplicate failure prefix, got:\n%s", view)
	}
}

func TestDetailHeaderShowsStatusIndicatorsAndLifecycleCounts(t *testing.T) {
	detail := NewDetailView(140, 80)
	detail.SetSession(&models.Session{
		AgentType: models.AgentCopilot,
		IsActive:  true,
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			toolLifecycleMessage("Started tool: bash", models.ActivityLifecycleStarted, "running-tool"),
			toolLifecycleMessage("Started tool: view", models.ActivityLifecycleStarted, "done-tool"),
			toolLifecycleMessage("Tool completed: view", models.ActivityLifecycleCompleted, "done-tool"),
			toolLifecycleMessage("Started tool: gh", models.ActivityLifecycleStarted, "failed-tool"),
			toolLifecycleMessage("Tool failed: gh\nerror: denied", models.ActivityLifecycleFailed, "failed-tool"),
		},
	})

	view := detail.View()
	for _, want := range []string{"ACTIVE", "FOLLOW", "RUNNING 1", "DONE 1", "FAILED 1"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected detail header status indicator %q, got:\n%s", want, view)
		}
	}
}

func TestInactiveSessionWithFailedActivityRemainsIdle(t *testing.T) {
	detail := NewDetailView(140, 80)
	detail.SetSession(&models.Session{
		AgentType: models.AgentCopilot,
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			toolLifecycleMessage("Started tool: bash", models.ActivityLifecycleStarted, "tool-1"),
			toolLifecycleMessage("Tool failed: bash\nerror: denied", models.ActivityLifecycleFailed, "tool-1"),
		},
	})

	view := detail.View()
	for _, want := range []string{"IDLE", "FAILED 1"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected inactive session status %q with failed activity count, got:\n%s", want, view)
		}
	}
	if strings.Contains(view, "⚠ FAILED  ") {
		t.Fatalf("expected session status to remain idle, got:\n%s", view)
	}
}

func TestInactiveSessionWithTerminalFailureShowsFailed(t *testing.T) {
	detail := NewDetailView(140, 80)
	detail.SetSession(&models.Session{
		AgentType: models.AgentCopilot,
		Messages: []models.Message{
			sessionLifecycleMessage("Task incomplete", models.ActivityLifecycleFailed),
		},
	})

	view := detail.View()
	if !strings.Contains(view, "FAILED") {
		t.Fatalf("expected terminal failed session status, got:\n%s", view)
	}
}

func TestDetailShowsUnmatchedToolCompletion(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			toolLifecycleMessage("Tool completed: orphan", models.ActivityLifecycleCompleted, "tool-orphan"),
		},
	})

	if len(detail.rows) != 2 || detail.rows[1].kind != activityRowMessage {
		t.Fatalf("expected unmatched completion to remain a standalone row, got %#v", detail.rows)
	}
	if view := detail.View(); !strings.Contains(view, "Tool completed: orphan") {
		t.Fatalf("expected unmatched completion to remain visible, got:\n%s", view)
	}
}

func TestDetailOpenGroupedActionShowsStartAndCompletion(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		ProjectPath: "/repo/project",
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			toolLifecycleMessage("Started tool: bash", models.ActivityLifecycleStarted, "tool-1"),
			{Role: "assistant", Content: "assistant between action events"},
			toolLifecycleMessage("Tool completed: bash", models.ActivityLifecycleCompleted, "tool-1"),
		},
	})
	detail.SelectNextRow()

	if !detail.OpenSelectedDetail() {
		t.Fatalf("expected grouped action to open focused detail")
	}
	view := detail.ThreadView()
	for _, want := range []string{"Status", "DONE", "Started tool: bash", "Grouped completion", "Tool completed: bash"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected focused grouped action detail with %q, got:\n%s", want, view)
		}
	}
	if strings.Contains(view, "assistant between action events") {
		t.Fatalf("expected grouped action detail to exclude surrounding activity, got:\n%s", view)
	}
}

func TestDetailCollapseHidesGroupedActionRows(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			toolLifecycleMessage("Started tool: bash", models.ActivityLifecycleStarted, "tool-1"),
			toolLifecycleMessage("Tool completed: bash", models.ActivityLifecycleCompleted, "tool-1"),
		},
	})

	detail.ToggleSelectedThread()
	view := detail.View()
	if strings.Contains(view, "Started tool: bash") || strings.Contains(view, "Tool completed: bash") {
		t.Fatalf("expected grouped action to collapse with the prompt thread, got:\n%s", view)
	}
	if !strings.Contains(view, "2 activity entries folded") {
		t.Fatalf("expected collapsed grouped action summary, got:\n%s", view)
	}
}

func TestDetailFollowModePausesAndResumes(t *testing.T) {
	detail := NewDetailView(120, 22)
	detail.SetSession(&models.Session{
		AgentType: models.AgentCopilot,
		IsActive:  true,
		Messages:  makeTestMessages(30),
	})
	if !detail.Following() {
		t.Fatalf("expected active session detail to start in follow mode")
	}

	detail.SelectPreviousRow()
	if detail.Following() {
		t.Fatalf("expected manual activity navigation to pause follow mode")
	}

	detail.ToggleFollow()
	if !detail.Following() {
		t.Fatalf("expected follow toggle to resume follow mode")
	}
	if !detail.AtBottom() {
		t.Fatalf("expected resuming follow mode to scroll to bottom")
	}
}

func TestDetailTimelineShowsSelectedMarkerAndFollowBadge(t *testing.T) {
	detail := NewDetailView(120, 28)
	detail.SetSession(&models.Session{
		AgentType: models.AgentCopilot,
		IsActive:  true,
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "tool", Content: "ran command"},
		},
	})

	view := detail.View()
	for _, want := range []string{"FOLLOW", "▌", "ran command"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected styled timeline state %q, got:\n%s", want, view)
		}
	}
}

func TestDetailShowsUnavailableActiveCopilotInputTokens(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		AgentType:   models.AgentCopilot,
		IsActive:    true,
		TotalTokens: models.TokenUsage{OutputTokens: 42},
	})

	view := detail.View()
	if !strings.Contains(view, "Input") || !strings.Contains(view, "-") {
		t.Fatalf("expected unavailable input token marker in detail view, got:\n%s", view)
	}
}

func TestDetailHeaderShowsCacheReadAndWriteTokens(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		AgentType: models.AgentCopilot,
		TotalTokens: models.TokenUsage{
			CacheReads:  391168,
			CacheWrites: 84,
		},
		Messages: []models.Message{{Role: "user", Content: "prompt"}},
	})

	view := detail.View()
	if !strings.Contains(view, "Cache") || !strings.Contains(view, "391168/84") {
		t.Fatalf("expected cache read/write summary in detail header, got:\n%s", view)
	}
}

func TestDetailFocusedEventShowsMetadataBlock(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		ProjectPath: "/repo/project",
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{
				Role:    "tool",
				Content: "ran command",
				Tokens:  models.TokenUsage{InputTokens: 10, OutputTokens: 20, CacheReads: 30, CacheWrites: 40},
				Meta: models.ActivityMeta{
					Kind:          models.ActivityKindTool,
					Lifecycle:     models.ActivityLifecycleCompleted,
					ID:            "tool-1",
					ParentID:      "prompt-1",
					InteractionID: "turn-1",
					Label:         "view",
				},
			},
		},
	})
	detail.SelectNextRow()

	if !detail.OpenSelectedDetail() {
		t.Fatalf("expected focused detail to open")
	}
	view := detail.ThreadView()
	for _, want := range []string{"row 002", "tool completed view", "id tool-1", "parent prompt-1", "interaction turn-1", "cache-write:40"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected focused metadata %q, got:\n%s", want, view)
		}
	}
}

func TestDetailHeaderStaysVisibleAfterScrolling(t *testing.T) {
	detail := NewDetailView(120, 22)
	detail.SetSession(&models.Session{
		AgentType:   models.AgentCopilot,
		ProjectPath: "/repo/project",
		IsActive:    true,
		TotalTokens: models.TokenUsage{InputTokens: 123, OutputTokens: 456, CacheReads: 7},
		Messages:    makeTestMessages(30),
	})

	detail.PageDown()
	view := detail.View()

	for _, want := range []string{"Copilot CLI", "/repo/project", "Input", "Output", "Cache", "ACTIVE"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected fixed header to remain visible with %q, got:\n%s", want, view)
		}
	}
}

func TestDetailOpenSelectedThreadShowsRelatedActivityOnly(t *testing.T) {
	detail := NewDetailView(120, 80)
	detail.SetSession(&models.Session{
		ProjectPath: "/repo/project",
		Messages: []models.Message{
			{Role: "user", Content: "first prompt"},
			{Role: "assistant", Content: "first response"},
			{Role: "assistant", Content: "first follow up"},
			{Role: "user", Content: "second prompt"},
			{Role: "assistant", Content: "second response"},
		},
	})

	if !detail.OpenSelectedThread() {
		t.Fatalf("expected selected user prompt to open")
	}
	view := detail.ThreadView()

	if !strings.Contains(view, "first prompt") ||
		!strings.Contains(view, "first response") ||
		!strings.Contains(view, "first follow up") {
		t.Fatalf("expected selected prompt thread content, got:\n%s", view)
	}
	if strings.Contains(view, "second prompt") || strings.Contains(view, "second response") {
		t.Fatalf("expected prompt detail to exclude following user thread, got:\n%s", view)
	}
}

func TestDetailPromptDetailIsVerbose(t *testing.T) {
	longContent := strings.Repeat("x", 600)
	rendered := renderVerboseMessage(1, models.Message{Role: "assistant", Content: longContent}, 80)
	if strings.Count(rendered, "x") != len(longContent) {
		t.Fatalf("expected prompt detail to render full assistant content")
	}
}

func makeTestMessages(count int) []models.Message {
	messages := make([]models.Message, 0, count)
	for i := 0; i < count; i++ {
		role := "assistant"
		if i%3 == 0 {
			role = "user"
		}
		messages = append(messages, models.Message{
			Role:    role,
			Content: "activity",
		})
	}
	return messages
}

func lineContaining(text, needle string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func toolLifecycleMessage(content, lifecycle, id string) models.Message {
	return models.Message{
		Role:    "tool",
		Content: content,
		Meta: models.ActivityMeta{
			Kind:      models.ActivityKindTool,
			Lifecycle: lifecycle,
			ID:        id,
			Label:     "bash",
		},
	}
}

func sessionLifecycleMessage(content, lifecycle string) models.Message {
	return models.Message{
		Role:    "session",
		Content: content,
		Meta: models.ActivityMeta{
			Kind:      models.ActivityKindSession,
			Lifecycle: lifecycle,
			Label:     "task",
		},
	}
}

func TestDetailFollowPausePersistedAcrossRefresh(t *testing.T) {
	// Create initial active session with some messages
	initialSession := &models.Session{
		ID:       "test-session",
		LogPath:  "/tmp/test",
		IsActive: true,
		Messages: []models.Message{
			{Role: "user", Content: "msg1"},
			{Role: "assistant", Content: "resp1"},
			{Role: "assistant", Content: "resp2"},
		},
	}

	detail := NewDetailView(120, 30)
	detail.SetSession(initialSession)

	// User should start in follow mode for active session
	if !detail.follow {
		t.Error("Expected follow mode to be enabled for active session")
	}

	// User scrolls up (SelectPreviousRow), pausing follow
	detail.SelectPreviousRow()
	if detail.follow {
		t.Error("Expected follow mode to be disabled after SelectPreviousRow")
	}

	// Session refreshes with new message (simulating live updates)
	updatedSession := &models.Session{
		ID:       "test-session",
		LogPath:  "/tmp/test",
		IsActive: true,
		Messages: []models.Message{
			{Role: "user", Content: "msg1"},
			{Role: "assistant", Content: "resp1"},
			{Role: "assistant", Content: "resp2"},
			{Role: "assistant", Content: "resp3"}, // new message
		},
	}

	// Simulate what App.refreshDetailSession does
	shouldFollow := detail.Following()
	detail.SetSession(updatedSession)

	// After refresh, follow should still be paused
	if detail.follow {
		t.Errorf("Expected follow mode to remain disabled after refresh (shouldFollow was %v)", shouldFollow)
	}
}

func TestDetailFollowNotReEnabledWhenUserPausedFollow(t *testing.T) {
	// Test that even if wasAtBottom is true, we don't re-enable follow if user explicitly paused it
	initialSession := &models.Session{
		ID:       "test-session",
		LogPath:  "/tmp/test",
		IsActive: true,
		Messages: []models.Message{
			{Role: "user", Content: "msg1"},
			{Role: "assistant", Content: "resp1"},
			{Role: "assistant", Content: "resp2"},
		},
	}

	detail := NewDetailView(120, 30)
	detail.SetSession(initialSession)

	// User should start in follow mode for active session
	if !detail.follow {
		t.Error("Expected follow mode to be enabled for active session")
	}
	if detail.userPausedFollow {
		t.Error("Expected userPausedFollow to be false on new active session")
	}

	// User explicitly pauses follow (e.g., by scrolling up)
	detail.SelectPreviousRow()
	if detail.follow {
		t.Error("Expected follow mode to be disabled after SelectPreviousRow")
	}
	if !detail.userPausedFollow {
		t.Error("Expected userPausedFollow to be true after SelectPreviousRow")
	}

	// Session refreshes with new message (same session ID and log path)
	updatedSession := &models.Session{
		ID:       "test-session",
		LogPath:  "/tmp/test",
		IsActive: true,
		Messages: []models.Message{
			{Role: "user", Content: "msg1"},
			{Role: "assistant", Content: "resp1"},
			{Role: "assistant", Content: "resp2"},
			{Role: "assistant", Content: "resp3"}, // new message
		},
	}

	// SetSession with updated messages for same session
	detail.SetSession(updatedSession)

	// After refresh of same session, userPausedFollow flag should be preserved
	if !detail.userPausedFollow {
		t.Error("Expected userPausedFollow to remain true after SetSession for same session")
	}
	// And follow should still be false
	if detail.follow {
		t.Error("Expected follow to remain false after SetSession for same session")
	}
}

func TestRefreshDetailSessionRespectsPausedFollow(t *testing.T) {
	// Test simulating app.refreshDetailSession() logic to ensure wasAtBottom doesn't override userPausedFollow
	initialSession := &models.Session{
		ID:       "session-1",
		LogPath:  "/path/to/session",
		IsActive: true,
		Messages: []models.Message{
			{Role: "user", Content: "msg1"},
			{Role: "assistant", Content: "resp1"},
			{Role: "assistant", Content: "resp2"},
		},
	}

	detail := NewDetailView(120, 30)
	detail.SetSession(initialSession)

	// User pauses follow by scrolling
	detail.SelectPreviousRow()

	// Simulate app.refreshDetailSession() logic
	shouldFollow := detail.Following()            // false
	userPausedFollow := detail.UserPausedFollow() // true
	detail.SetSession(initialSession)

	// The key logic from refreshDetailSession:
	// if (shouldFollow || (wasAtBottom && !userPausedFollow)) { FollowLatest() }
	// This should NOT trigger because userPausedFollow is true
	shouldAutoFollow := shouldFollow || (true && !userPausedFollow) // true && !true = true && false = false

	if shouldAutoFollow {
		t.Error("Expected auto-follow to be disabled when user explicitly paused follow")
	}

	// Verify detail state
	if detail.follow {
		t.Error("Expected follow to remain disabled")
	}
}
