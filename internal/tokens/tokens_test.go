package tokens

import "testing"

func TestFromInfoDistinguishesKnownZeroTokensFromMissing(t *testing.T) {
	missing := FromInfo(map[string]any{})
	if missing.Known {
		t.Fatalf("missing token data should not be known")
	}
	known := FromInfo(map[string]any{
		"total_token_usage": map[string]any{
			"input_tokens":            float64(0),
			"cached_input_tokens":     float64(0),
			"output_tokens":           float64(0),
			"reasoning_output_tokens": float64(0),
			"total_tokens":            float64(0),
		},
		"last_token_usage": map[string]any{
			"input_tokens":            float64(1),
			"cached_input_tokens":     float64(2),
			"output_tokens":           float64(3),
			"reasoning_output_tokens": float64(4),
			"total_tokens":            float64(10),
		},
		"model_context_window": float64(128000),
	})
	if !known.Known {
		t.Fatalf("zero token data should be known")
	}
	if known.Total.TotalTokens != 0 || known.Latest.TotalTokens != 10 || known.ModelContextWindow != 128000 {
		t.Fatalf("unexpected token summary: %#v", known)
	}
}
