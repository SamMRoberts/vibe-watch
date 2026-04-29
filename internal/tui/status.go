package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

const (
	statusActive     = "active"
	statusIdle       = "idle"
	statusFollow     = "follow"
	statusPaused     = "paused"
	statusRequested  = "requested"
	statusRunning    = "running"
	statusDone       = "done"
	statusFailed     = "failed"
	statusRefreshing = "refreshing"
)

type statusSpec struct {
	Label   string
	Compact string
	Icon    string
	BG      lipgloss.Color
	FG      lipgloss.Color
	Style   lipgloss.Style
}

type activityStatusSummary struct {
	Requested int
	Running   int
	Done      int
	Failed    int
}

func indicatorSpec(state string) statusSpec {
	switch state {
	case statusActive:
		return statusSpec{"ACTIVE", "act", "●", colorSuccess, colorBackground, styleSuccess}
	case statusIdle:
		return statusSpec{"IDLE", "idle", "○", colorSurfaceAlt, colorMuted, styleMuted}
	case statusFollow:
		return statusSpec{"FOLLOW", "fol", "◆", colorPrimary, colorBackground, styleAccent}
	case statusPaused:
		return statusSpec{"PAUSED", "pause", "Ⅱ", colorWarning, colorBackground, styleWarning}
	case statusRequested:
		return statusSpec{"REQUESTED", "req", "◌", colorInfo, colorBackground, styleInfo}
	case statusRunning:
		return statusSpec{"RUNNING", "run", "⏳", colorWarning, colorBackground, styleWarning}
	case statusDone:
		return statusSpec{"DONE", "done", "✓", colorSuccess, colorBackground, styleSuccess}
	case statusFailed:
		return statusSpec{"FAILED", "fail", "⚠", colorError, colorBackground, styleError}
	case statusRefreshing:
		return statusSpec{"REFRESH", "ref", "↻", colorInfo, colorBackground, styleInfo}
	default:
		return statusSpec{state, state, "●", colorSurfaceAlt, colorMuted, styleMuted}
	}
}

func statusChip(state string) string {
	spec := indicatorSpec(state)
	return pill(spec.Icon+" "+spec.Label, spec.BG, spec.FG, true)
}

func statusCountChip(state string, count int) string {
	spec := indicatorSpec(state)
	return pill(fmt.Sprintf("%s %s %d", spec.Icon, spec.Label, count), spec.BG, spec.FG, true)
}

func plainStatusText(state string, width int) string {
	spec := indicatorSpec(state)
	if width > 0 && width < 5 {
		return spec.Compact
	}
	text := spec.Icon + " " + lowerStatusLabel(spec.Label)
	if width > 0 && lipgloss.Width(text) > width {
		return spec.Compact
	}
	return text
}

func lowerStatusLabel(label string) string {
	switch label {
	case "ACTIVE":
		return "active"
	case "IDLE":
		return "idle"
	case "FOLLOW":
		return "follow"
	case "PAUSED":
		return "paused"
	case "REQUESTED":
		return "requested"
	case "RUNNING":
		return "running"
	case "DONE":
		return "done"
	case "FAILED":
		return "failed"
	default:
		return label
	}
}

func sessionStatus(session *models.Session) string {
	if session != nil && session.IsActive {
		return statusActive
	}
	if sessionTerminalFailed(session) {
		return statusFailed
	}
	return statusIdle
}

func sessionTerminalFailed(session *models.Session) bool {
	if session == nil {
		return false
	}
	for _, msg := range session.Messages {
		if msg.Meta.Kind == models.ActivityKindSession && msg.Meta.Lifecycle == models.ActivityLifecycleFailed {
			return true
		}
	}
	return false
}

func lifecycleIndicatorState(lifecycle string) string {
	switch lifecycle {
	case models.ActivityLifecycleRequested:
		return statusRequested
	case models.ActivityLifecycleCompleted:
		return statusDone
	case models.ActivityLifecycleFailed:
		return statusFailed
	default:
		return statusRunning
	}
}

func activityStatusCounts(messages []models.Message) activityStatusSummary {
	states := make(map[actionGroupKey]string)
	for _, msg := range messages {
		if msg.Meta.Kind == "" || msg.Meta.ID == "" {
			continue
		}
		key := actionGroupKey{kind: msg.Meta.Kind, id: msg.Meta.ID}
		switch {
		case isActionEnd(msg):
			states[key] = lifecycleIndicatorState(msg.Meta.Lifecycle)
		case isActionStart(msg):
			if _, exists := states[key]; !exists || msg.Meta.Lifecycle == models.ActivityLifecycleStarted {
				states[key] = lifecycleIndicatorState(groupedActionState(msg, models.Message{}, false))
			}
		}
	}

	var summary activityStatusSummary
	for _, state := range states {
		switch state {
		case statusRequested:
			summary.Requested++
		case statusRunning:
			summary.Running++
		case statusDone:
			summary.Done++
		case statusFailed:
			summary.Failed++
		}
	}
	return summary
}

func activityStatusSummaryChips(summary activityStatusSummary) string {
	chips := make([]string, 0, 4)
	if summary.Requested > 0 {
		chips = append(chips, statusCountChip(statusRequested, summary.Requested))
	}
	if summary.Running > 0 {
		chips = append(chips, statusCountChip(statusRunning, summary.Running))
	}
	if summary.Done > 0 {
		chips = append(chips, statusCountChip(statusDone, summary.Done))
	}
	if summary.Failed > 0 {
		chips = append(chips, statusCountChip(statusFailed, summary.Failed))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, joinWithSpaces(chips)...)
}

func joinWithSpaces(parts []string) []string {
	if len(parts) <= 1 {
		return parts
	}
	spaced := make([]string, 0, len(parts)*2-1)
	for i, part := range parts {
		if i > 0 {
			spaced = append(spaced, " ")
		}
		spaced = append(spaced, part)
	}
	return spaced
}
