package tui

import (
	"strings"
	"testing"

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
	groupPos := strings.Index(view, "bash · completed")
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

	if got := detail.rowLineOffsets[1]; got != 2 {
		t.Fatalf("expected a blank spacer line before second activity, got row offset %d", got)
	}
	if got := detail.rowLineOffsets[2]; got != 4 {
		t.Fatalf("expected consistent spacer line before third activity, got row offset %d", got)
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
	if got := detail.rowLineOffsets[2] - detail.rowLineOffsets[1]; got != 2 {
		t.Fatalf("expected one lifecycle row plus one spacer before next group, got offset delta %d", got)
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
	if !strings.Contains(view, "bash · completed") || !strings.Contains(view, "telemetry: resultLength:12") {
		t.Fatalf("expected compact lifecycle summary with useful telemetry detail, got:\n%s", view)
	}
	if strings.Contains(view, "model: gpt-5.5") || strings.Contains(view, "Tool completed: bash") {
		t.Fatalf("expected lifecycle row to suppress low-signal duplicate detail, got:\n%s", view)
	}
	if got := detail.rowLineOffsets[2] - detail.rowLineOffsets[1]; got != 3 {
		t.Fatalf("expected lifecycle detail line plus spacer before next group, got offset delta %d", got)
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
	if !strings.Contains(view, "bash · running") || !strings.Contains(view, "→…") {
		t.Fatalf("expected running lifecycle row, got:\n%s", view)
	}
	if strings.Contains(view, "Started tool: bash") {
		t.Fatalf("expected running lifecycle row to avoid raw start wording, got:\n%s", view)
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
	if !strings.Contains(view, "bash · failed") || !strings.Contains(view, "permission denied") {
		t.Fatalf("expected failed lifecycle row with error detail, got:\n%s", view)
	}
	if strings.Contains(view, "Tool failed: bash") || strings.Contains(view, "error: permission denied") {
		t.Fatalf("expected failed lifecycle row to suppress duplicate failure prefix, got:\n%s", view)
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
	for _, want := range []string{"Started tool: bash", "Grouped completion", "Tool completed: bash"} {
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
