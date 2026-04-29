package tokens

import "github.com/SamMRoberts/vibe-watch/internal/sessions"

func FromInfo(info any) sessions.TokenSummary {
	m, ok := info.(map[string]any)
	if !ok {
		return sessions.TokenSummary{}
	}
	summary := sessions.TokenSummary{}
	if total := usageFromAny(m["total_token_usage"]); total.Known {
		summary.Total = total
		summary.Known = true
	}
	if latest := usageFromAny(m["last_token_usage"]); latest.Known {
		summary.Latest = latest
		summary.Known = true
	}
	if window, ok := intFromAny(m["model_context_window"]); ok {
		summary.ModelContextWindow = window
	}
	return summary
}

func usageFromAny(v any) sessions.TokenUsage {
	m, ok := v.(map[string]any)
	if !ok {
		return sessions.TokenUsage{}
	}
	usage := sessions.TokenUsage{Known: true}
	usage.InputTokens, _ = intFromAny(m["input_tokens"])
	usage.CachedInputTokens, _ = intFromAny(m["cached_input_tokens"])
	usage.OutputTokens, _ = intFromAny(m["output_tokens"])
	usage.ReasoningOutputTokens, _ = intFromAny(m["reasoning_output_tokens"])
	usage.TotalTokens, _ = intFromAny(m["total_tokens"])
	return usage
}

func intFromAny(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}
