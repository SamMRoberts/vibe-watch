package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

func TestAppShellFitsAdaptiveWidths(t *testing.T) {
	for _, width := range []int{70, 80, 100, 140} {
		app := &App{
			view:     viewDashboard,
			width:    width,
			height:   30,
			sessions: []*models.Session{{AgentType: models.AgentCopilot, ProjectPath: "/repo", IsActive: true}},
			dashboard: func() *DashboardView {
				d := NewDashboardView(width, 30)
				d.SetSessions([]*models.Session{{AgentType: models.AgentCopilot, ProjectPath: "/repo", IsActive: true}}, "")
				return d
			}(),
		}

		view := app.View()
		if got := maxRenderedLineWidth(view); got > width {
			t.Fatalf("expected app shell to fit width %d, got max line width %d:\n%s", width, got, view)
		}
		if !strings.Contains(view, "vibe-watch") {
			t.Fatalf("expected shell branding, got:\n%s", view)
		}
		if width >= 82 && !strings.Contains(view, "observatory") {
			t.Fatalf("expected observatory subtitle at width %d, got:\n%s", width, view)
		}
	}
}

func TestAppShellKeepsHeaderVisibleAfterDashboardLoads(t *testing.T) {
	width, height := 90, 18
	sessions := make([]*models.Session, 20)
	for i := range sessions {
		sessions[i] = &models.Session{
			ID:          "session",
			AgentType:   models.AgentCopilot,
			ProjectPath: "/repo/project/with/a/long/path",
			IsActive:    i == 0,
			Messages:    []models.Message{{Role: "user", Content: "prompt"}},
			LastUpdated: time.Now(),
		}
	}
	dashboard := NewDashboardView(width, height)
	dashboard.SetSessions(sessions, "")
	app := &App{
		view:      viewDashboard,
		width:     width,
		height:    height,
		loading:   false,
		sessions:  sessions,
		dashboard: dashboard,
	}

	view := app.View()

	if got := lipgloss.Height(view); got > height {
		t.Fatalf("expected loaded shell height to fit terminal height %d, got %d:\n%s", height, got, view)
	}
	lines := strings.Split(view, "\n")
	if len(lines) == 0 || !strings.Contains(lines[0], "vibe-watch") {
		t.Fatalf("expected top shell header to remain visible after load, got:\n%s", view)
	}
	if !strings.Contains(view, "q quit") {
		t.Fatalf("expected footer help to remain visible after load, got:\n%s", view)
	}
}

func TestAppFooterUsesGeneratedHelp(t *testing.T) {
	app := &App{view: viewDetail, width: 150, height: 30, detail: NewDetailView(150, 30)}
	app.detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "assistant", Content: "activity"},
		},
	})

	view := app.View()
	for _, want := range []string{"detail standard", "enter opens prompt thread", "tab", "views", "f", "follow"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected generated detail help to include %q, got:\n%s", want, view)
		}
	}
}

func TestAppHeaderFailedCountUsesTerminalFailuresOnly(t *testing.T) {
	app := &App{
		width: 120,
		sessions: []*models.Session{
			{
				ID:       "active-with-failed-step",
				IsActive: true,
				Messages: []models.Message{
					toolLifecycleMessage("Tool failed: bash", models.ActivityLifecycleFailed, "tool-1"),
				},
			},
			{
				ID: "terminal-failed",
				Messages: []models.Message{
					sessionLifecycleMessage("Task incomplete", models.ActivityLifecycleFailed),
				},
			},
		},
	}

	status := app.renderHeaderStatus()
	for _, want := range []string{"● ACTIVE 1", "⚠ FAILED 1"} {
		if !strings.Contains(status, want) {
			t.Fatalf("expected shell status %q, got %q", want, status)
		}
	}
	if strings.Contains(status, "⚠ FAILED 2") {
		t.Fatalf("expected failed activity in active session not to count as failed session, got %q", status)
	}
}

func TestUpdateViewsRefreshesSelectedDetailSession(t *testing.T) {
	oldSession := &models.Session{
		ID:          "session-1",
		AgentType:   models.AgentCopilot,
		ProjectPath: "/repo/project",
		Messages: []models.Message{
			{Role: "assistant", Content: "old response"},
		},
		LastUpdated: time.Now().Add(-time.Minute),
	}
	updatedSession := &models.Session{
		ID:          "session-1",
		AgentType:   models.AgentCopilot,
		ProjectPath: "/repo/project",
		Messages: []models.Message{
			{Role: "assistant", Content: "new response"},
		},
		LastUpdated: time.Now(),
	}

	app := &App{
		sessions: []*models.Session{updatedSession},
		detail:   NewDetailView(100, 40),
	}
	app.detail.SetSession(oldSession)

	app.updateViews()

	if app.detail.session != updatedSession {
		t.Fatalf("expected selected detail session to be rebound to refreshed session")
	}
	if got := app.detail.session.Messages[0].Content; got != "new response" {
		t.Fatalf("expected refreshed detail content, got %q", got)
	}
}

func TestFindMatchingSessionFallsBackToLogPath(t *testing.T) {
	target := &models.Session{LogPath: "/tmp/session.jsonl"}
	updated := &models.Session{LogPath: "/tmp/session.jsonl"}

	if got := findMatchingSession(target, []*models.Session{updated}); got != updated {
		t.Fatalf("expected session to match by log path")
	}
}

func TestUpdateViewsScrollsActiveDetailSessionToBottom(t *testing.T) {
	oldSession := &models.Session{
		ID:        "session-1",
		AgentType: models.AgentCopilot,
		IsActive:  true,
		Messages:  makeDetailMessages(20),
	}
	updatedSession := &models.Session{
		ID:        "session-1",
		AgentType: models.AgentCopilot,
		IsActive:  true,
		Messages:  makeDetailMessages(40),
	}

	app := &App{
		view:     viewDetail,
		sessions: []*models.Session{updatedSession},
		detail:   NewDetailView(100, 20),
	}
	app.detail.SetSession(oldSession)
	app.detail.viewport.GotoBottom()

	app.updateViews()

	if !app.detail.viewport.AtBottom() {
		t.Fatalf("expected active detail viewport to auto-scroll to bottom")
	}
}

func TestUpdateViewsDoesNotScrollActiveDetailSessionWhenUserScrolledUp(t *testing.T) {
	oldSession := &models.Session{
		ID:        "session-1",
		AgentType: models.AgentCopilot,
		IsActive:  true,
		Messages:  makeDetailMessages(40),
	}
	updatedSession := &models.Session{
		ID:        "session-1",
		AgentType: models.AgentCopilot,
		IsActive:  true,
		Messages:  makeDetailMessages(60),
	}

	app := &App{
		view:     viewDetail,
		sessions: []*models.Session{updatedSession},
		detail:   NewDetailView(100, 20),
	}
	app.detail.SetSession(oldSession)
	app.detail.viewport.GotoBottom()
	app.detail.PageUp()
	previousOffset := app.detail.viewport.YOffset

	app.updateViews()

	if app.detail.viewport.AtBottom() {
		t.Fatalf("expected active detail viewport to preserve scrolled-up position")
	}
	if app.detail.viewport.YOffset != previousOffset {
		t.Fatalf("expected scrolled-up offset %d to be preserved, got %d", previousOffset, app.detail.viewport.YOffset)
	}
}

func TestEnterOpensPromptDetailFromSessionDetail(t *testing.T) {
	app := &App{
		view:   viewDetail,
		detail: NewDetailView(100, 40),
	}
	app.detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "assistant", Content: "activity"},
			{Role: "user", Content: "next prompt"},
		},
	})

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := model.(*App)

	if updated.view != viewPromptDetail {
		t.Fatalf("expected enter to open prompt detail view, got %v", updated.view)
	}
	if view := updated.detail.ThreadView(); !strings.Contains(view, "activity") || strings.Contains(view, "next prompt") {
		t.Fatalf("expected prompt detail content for selected prompt, got:\n%s", view)
	}
}

func TestEnterOpensFocusedEventDetailFromSessionDetail(t *testing.T) {
	app := &App{
		view:   viewDetail,
		detail: NewDetailView(100, 40),
	}
	app.detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "tool", Content: "tool activity"},
			{Role: "assistant", Content: "assistant activity"},
		},
	})

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated := model.(*App)
	model, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated = model.(*App)

	if updated.view != viewPromptDetail {
		t.Fatalf("expected enter to open focused detail view, got %v", updated.view)
	}
	view := updated.detail.ThreadView()
	if !strings.Contains(view, "Focused activity 2") || !strings.Contains(view, "tool activity") {
		t.Fatalf("expected focused event detail, got:\n%s", view)
	}
	if strings.Contains(view, "assistant activity") {
		t.Fatalf("expected focused event detail to exclude following activity, got:\n%s", view)
	}
}

func TestBracketKeysJumpBetweenUserPrompts(t *testing.T) {
	app := &App{
		view:   viewDetail,
		detail: NewDetailView(100, 40),
	}
	app.detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "first prompt"},
			{Role: "assistant", Content: "first activity"},
			{Role: "user", Content: "second prompt"},
		},
	})

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	updated := model.(*App)

	row, ok := updated.detail.selectedActivityRow()
	if !ok {
		t.Fatalf("expected selected row")
	}
	if row.messageIndex != 2 {
		t.Fatalf("expected ] to jump to next user prompt, got message index %d", row.messageIndex)
	}

	model, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	updated = model.(*App)
	row, ok = updated.detail.selectedActivityRow()
	if !ok {
		t.Fatalf("expected selected row")
	}
	if row.messageIndex != 0 {
		t.Fatalf("expected [ to jump to previous user prompt, got message index %d", row.messageIndex)
	}
}

func TestCollapseAllKeyTogglesAllPromptThreads(t *testing.T) {
	app := &App{
		view:   viewDetail,
		detail: NewDetailView(100, 40),
	}
	app.detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "first prompt"},
			{Role: "assistant", Content: "first activity"},
			{Role: "user", Content: "second prompt"},
			{Role: "assistant", Content: "second activity"},
		},
	})

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	updated := model.(*App)
	collapsed := updated.detail.View()
	if strings.Contains(collapsed, "first activity") || strings.Contains(collapsed, "second activity") {
		t.Fatalf("expected c to collapse all prompt threads, got:\n%s", collapsed)
	}

	model, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	updated = model.(*App)
	expanded := updated.detail.View()
	if !strings.Contains(expanded, "first activity") || !strings.Contains(expanded, "second activity") {
		t.Fatalf("expected c to expand all prompt threads on second press, got:\n%s", expanded)
	}
}

func TestDetailLevelKeyCyclesTimelineDetail(t *testing.T) {
	app := &App{
		view:   viewDetail,
		detail: NewDetailView(100, 40),
	}
	app.detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "assistant", Content: "activity", Tokens: models.TokenUsage{OutputTokens: 25}},
		},
	})

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	updated := model.(*App)
	if status := updated.detail.FooterStatus(); !strings.Contains(status, "detail expanded") {
		t.Fatalf("expected d to switch to expanded detail, got status %q", status)
	}

	model, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	updated = model.(*App)
	if status := updated.detail.FooterStatus(); !strings.Contains(status, "detail compact") {
		t.Fatalf("expected d to switch to compact detail, got status %q", status)
	}
}

func TestTimestampKeyTogglesSessionDetailTimestamps(t *testing.T) {
	ts := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	app := &App{
		view:   viewDetail,
		detail: NewDetailView(100, 40),
	}
	app.detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt", Timestamp: ts},
			{Role: "assistant", Content: "activity", Timestamp: ts.Add(time.Second)},
		},
	})
	if view := app.detail.View(); strings.Contains(view, "12:00:") {
		t.Fatalf("expected timestamps hidden by default, got:\n%s", view)
	}
	if status := app.detail.FooterStatus(); !strings.Contains(status, "time off") {
		t.Fatalf("expected timestamp status to start off, got %q", status)
	}

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	updated := model.(*App)
	if view := updated.detail.View(); !strings.Contains(view, "12:00:00") {
		t.Fatalf("expected t to show timestamps, got:\n%s", view)
	}
	if status := updated.detail.FooterStatus(); !strings.Contains(status, "time on") {
		t.Fatalf("expected timestamp status to be on, got %q", status)
	}

	model, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	updated = model.(*App)
	if view := updated.detail.View(); strings.Contains(view, "12:00:") {
		t.Fatalf("expected second t press to hide timestamps, got:\n%s", view)
	}
	if status := updated.detail.FooterStatus(); !strings.Contains(status, "time off") {
		t.Fatalf("expected timestamp status to return off, got %q", status)
	}
}

func TestEscReturnsFromPromptDetailToSessionDetail(t *testing.T) {
	app := &App{
		view:   viewPromptDetail,
		detail: NewDetailView(100, 40),
	}
	app.detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "assistant", Content: "activity"},
		},
	})
	if !app.detail.OpenSelectedThread() {
		t.Fatalf("expected prompt detail to open")
	}

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := model.(*App)

	if updated.view != viewDetail {
		t.Fatalf("expected esc to return to session detail, got %v", updated.view)
	}
	if view := updated.detail.View(); !strings.Contains(view, "prompt") || !strings.Contains(view, "activity") {
		t.Fatalf("expected session detail content after returning, got:\n%s", view)
	}
}

func TestFollowKeyDoesNotScrollFocusedDetail(t *testing.T) {
	app := &App{
		view:   viewPromptDetail,
		detail: NewDetailView(100, 18),
	}
	app.detail.SetSession(&models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "assistant", Content: strings.Repeat("long activity ", 80)},
		},
	})
	if !app.detail.OpenSelectedThread() {
		t.Fatalf("expected prompt detail to open")
	}
	before := app.detail.viewport.YOffset

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	updated := model.(*App)

	if updated.detail.viewport.YOffset != before {
		t.Fatalf("expected f not to scroll focused detail, got offset %d want %d", updated.detail.viewport.YOffset, before)
	}
}

func TestUpdateViewsKeepsFocusedEventContentWhileFollowing(t *testing.T) {
	oldSession := &models.Session{
		ID:        "session-1",
		AgentType: models.AgentCopilot,
		IsActive:  true,
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "tool", Content: "tool activity"},
		},
	}
	updatedSession := &models.Session{
		ID:        "session-1",
		AgentType: models.AgentCopilot,
		IsActive:  true,
		Messages: []models.Message{
			{Role: "user", Content: "prompt"},
			{Role: "tool", Content: "tool activity"},
			{Role: "assistant", Content: "new live activity"},
		},
	}
	app := &App{
		view:     viewPromptDetail,
		sessions: []*models.Session{updatedSession},
		detail:   NewDetailView(100, 40),
	}
	app.detail.SetSession(oldSession)
	if !app.detail.OpenSelectedDetail() {
		t.Fatalf("expected selected tool activity to open")
	}

	app.updateViews()
	view := app.detail.ThreadView()

	if !strings.Contains(view, "Focused activity 2") || !strings.Contains(view, "tool activity") {
		t.Fatalf("expected focused event content to remain visible, got:\n%s", view)
	}
	if strings.Contains(view, "new live activity") {
		t.Fatalf("expected focused event content to avoid switching back to session timeline, got:\n%s", view)
	}
}

func makeDetailMessages(count int) []models.Message {
	messages := make([]models.Message, 0, count)
	for i := 0; i < count; i++ {
		messages = append(messages, models.Message{
			Role:    "assistant",
			Content: "line",
		})
	}
	return messages
}

func maxRenderedLineWidth(rendered string) int {
	maxWidth := 0
	for _, line := range strings.Split(rendered, "\n") {
		if width := lipgloss.Width(line); width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}
