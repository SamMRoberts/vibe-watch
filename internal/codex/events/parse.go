package events

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

type Event struct {
	Line int            `json:"line"`
	Raw  map[string]any `json:"-"`
}

type FileResult struct {
	Events         []Event `json:"events"`
	Lines          int     `json:"lines"`
	MalformedLines int     `json:"malformed_lines"`
}

func ParseJSONL(r io.Reader) FileResult {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var result FileResult
	for scanner.Scan() {
		result.Lines++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			result.MalformedLines++
			continue
		}
		result.Events = append(result.Events, Event{
			Line: result.Lines,
			Raw:  raw,
		})
	}
	if scanner.Err() != nil {
		result.MalformedLines++
	}
	return result
}
