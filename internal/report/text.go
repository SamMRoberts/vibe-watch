package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SamMRoberts/vibe-watch/internal/analysis"
)

func FormatFull(result analysis.Result) string {
	var b strings.Builder
	b.WriteString(FormatScanSummary(result.Summary))
	b.WriteString("\n")
	b.WriteString(FormatMetrics(result.Metrics))
	b.WriteString("\n")
	b.WriteString(FormatSuggestions(result.Suggestions))
	return b.String()
}

func FormatScanSummary(summary analysis.ScanSummary) string {
	var b strings.Builder
	b.WriteString("Scan summary\n")
	b.WriteString(fmt.Sprintf("  Session root: %s\n", summary.SessionRoot))
	b.WriteString(fmt.Sprintf("  Sessions:     %d\n", summary.SessionsScanned))
	b.WriteString(fmt.Sprintf("  Files:        %d\n", summary.FilesScanned))
	b.WriteString(fmt.Sprintf("  Lines:        %d\n", summary.LinesScanned))
	b.WriteString(fmt.Sprintf("  Events:       %d\n", summary.EventsScanned))
	b.WriteString(fmt.Sprintf("  Malformed:    %d\n", summary.MalformedLines))
	b.WriteString(fmt.Sprintf("  Skipped:      %d\n", summary.SkippedFiles))
	if !summary.FirstDate.IsZero() {
		b.WriteString(fmt.Sprintf("  Date range:   %s to %s\n", summary.FirstDate.Format("2006-01-02"), summary.LastDate.Format("2006-01-02")))
	}
	return b.String()
}

func FormatMetrics(metrics analysis.Metrics) string {
	var b strings.Builder
	b.WriteString("Metrics\n")
	writeCountMap(&b, "  Sessions by day", metrics.SessionsByDay, 10)
	writeCountMap(&b, "  Models", metrics.Models, 10)
	writeCountMap(&b, "  Repositories", metrics.Repositories, 10)
	writeCountMap(&b, "  Tools", metrics.Tools, 10)
	writeCountMap(&b, "  Event types", metrics.EventTypes, 10)
	b.WriteString(fmt.Sprintf("  Shell commands:        %d\n", metrics.ShellCommands))
	b.WriteString(fmt.Sprintf("  Failed shell commands: %d\n", metrics.FailedShellCommands))
	b.WriteString(fmt.Sprintf("  Approval signals:      %d\n", metrics.ApprovalEvents))
	b.WriteString(fmt.Sprintf("  Error signals:         %d\n", metrics.ErrorEvents))
	b.WriteString(fmt.Sprintf("  Verification signals:  %d\n", metrics.VerificationSignals))
	b.WriteString(fmt.Sprintf("  Final response signals:%d\n", metrics.FinalResponses))
	b.WriteString(fmt.Sprintf("  Unknown event types:   %d\n", metrics.UnknownEventTypes))
	return b.String()
}

func FormatSuggestions(suggestions []analysis.Suggestion) string {
	var b strings.Builder
	b.WriteString("Suggestions\n")
	if len(suggestions) == 0 {
		b.WriteString("  No suggestions for the selected range.\n")
		return b.String()
	}
	for _, suggestion := range suggestions {
		b.WriteString(fmt.Sprintf("  - %s [%s, %.0f%%]\n", suggestion.Title, suggestion.Severity, suggestion.Confidence*100))
		if suggestion.AffectedSet != "" {
			b.WriteString(fmt.Sprintf("    Affected: %s\n", suggestion.AffectedSet))
		}
		for _, evidence := range suggestion.Evidence {
			b.WriteString(fmt.Sprintf("    Evidence: %s\n", evidence))
		}
		b.WriteString(fmt.Sprintf("    Action: %s\n", suggestion.Action))
	}
	return b.String()
}

func writeCountMap(b *strings.Builder, title string, counts map[string]int, limit int) {
	b.WriteString(title)
	b.WriteString(":\n")
	if len(counts) == 0 {
		b.WriteString("    none\n")
		return
	}
	for index, entry := range sortedCounts(counts) {
		if index >= limit {
			b.WriteString(fmt.Sprintf("    ... %d more\n", len(counts)-limit))
			break
		}
		b.WriteString(fmt.Sprintf("    %s: %d\n", entry.Key, entry.Count))
	}
}

type countEntry struct {
	Key   string
	Count int
}

func sortedCounts(counts map[string]int) []countEntry {
	entries := make([]countEntry, 0, len(counts))
	for key, count := range counts {
		entries = append(entries, countEntry{Key: key, Count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count == entries[j].Count {
			return entries[i].Key < entries[j].Key
		}
		return entries[i].Count > entries[j].Count
	})
	return entries
}
