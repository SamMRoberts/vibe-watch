package events

import (
	"strings"
	"testing"
)

func TestParseJSONLToleratesMalformedLines(t *testing.T) {
	result := ParseJSONL(strings.NewReader("{\"type\":\"one\"}\nnot-json\n{\"type\":\"two\"}\n"))
	if result.Lines != 3 {
		t.Fatalf("expected 3 lines, got %d", result.Lines)
	}
	if result.MalformedLines != 1 {
		t.Fatalf("expected 1 malformed line, got %d", result.MalformedLines)
	}
	if len(result.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result.Events))
	}
}
