package analysis

import (
	"os"

	"github.com/SamMRoberts/vibe-watch/internal/codex/events"
	"github.com/SamMRoberts/vibe-watch/internal/codex/sessions"
)

func Run(opts sessions.ScanOptions) (Result, error) {
	files, err := sessions.Discover(opts)
	if err != nil {
		return Result{}, err
	}
	builder := newBuilder(opts.Root)
	for _, file := range files {
		handle, err := os.Open(file.Path)
		if err != nil {
			builder.skippedFiles++
			continue
		}
		result := events.ParseJSONL(handle)
		_ = handle.Close()
		builder.AddFile(file, result)
	}
	return builder.Result(), nil
}
