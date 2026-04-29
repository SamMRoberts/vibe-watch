package analysis

import "fmt"

func Suggest(summary ScanSummary, metrics Metrics) []Suggestion {
	var suggestions []Suggestion
	if summary.SessionsScanned == 0 {
		return []Suggestion{{
			Title:       "No sessions found for the selected range",
			Severity:    "info",
			Confidence:  1.0,
			Evidence:    []string{"0 session files scanned"},
			Action:      "Check --session-root, --since, --until, or --limit before drawing workflow conclusions.",
			AffectedSet: "selected range",
		}}
	}

	if metrics.VerificationSignals < summary.SessionsScanned {
		suggestions = append(suggestions, Suggestion{
			Title:       "Increase explicit verification evidence",
			Severity:    "medium",
			Confidence:  0.7,
			Evidence:    []string{fmt.Sprintf("%d verification signals across %d sessions", metrics.VerificationSignals, summary.SessionsScanned)},
			Action:      "End sessions with concrete validation commands and results so future reviews can distinguish tested work from unverified work.",
			AffectedSet: "selected sessions",
		})
	}

	if metrics.ShellCommands >= 3 && metrics.FailedShellCommands*100/metrics.ShellCommands >= 20 {
		suggestions = append(suggestions, Suggestion{
			Title:       "Review repeated shell command failures",
			Severity:    "medium",
			Confidence:  0.75,
			Evidence:    []string{fmt.Sprintf("%d failed shell commands out of %d detected", metrics.FailedShellCommands, metrics.ShellCommands)},
			Action:      "When a command fails twice, pause to inspect assumptions, paths, environment, or sandbox requirements before retrying variants.",
			AffectedSet: "shell tool usage",
		})
	}

	if metrics.ApprovalEvents > 0 {
		suggestions = append(suggestions, Suggestion{
			Title:       "Track escalation reasons",
			Severity:    "low",
			Confidence:  0.65,
			Evidence:    []string{fmt.Sprintf("%d approval or escalation signals detected", metrics.ApprovalEvents)},
			Action:      "Group escalation causes into dependency, network, destructive, and out-of-workspace categories to spot avoidable friction.",
			AffectedSet: "approval events",
		})
	}

	if summary.MalformedLines > 0 || metrics.UnknownEventTypes > 0 {
		suggestions = append(suggestions, Suggestion{
			Title:       "Harden Codex event decoding",
			Severity:    "low",
			Confidence:  0.8,
			Evidence:    []string{fmt.Sprintf("%d malformed lines, %d unknown event types", summary.MalformedLines, metrics.UnknownEventTypes)},
			Action:      "Add synthetic fixtures for the unrecognized shapes before relying on derived metrics from this range.",
			AffectedSet: "Codex adapter",
		})
	}

	if metrics.FinalResponses < summary.SessionsScanned {
		suggestions = append(suggestions, Suggestion{
			Title:       "Improve final handoff detection",
			Severity:    "low",
			Confidence:  0.55,
			Evidence:    []string{fmt.Sprintf("%d final-response signals across %d sessions", metrics.FinalResponses, summary.SessionsScanned)},
			Action:      "Either end sessions with a clear final response or teach the parser the current final-message event shape.",
			AffectedSet: "handoffs",
		})
	}

	return suggestions
}
