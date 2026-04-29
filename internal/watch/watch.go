package watch

import (
	"context"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/codex"
	"github.com/SamMRoberts/vibe-watch/internal/sessions"
)

const DefaultBatchSize = 20

type Options struct {
	Root  string
	Days  int
	Limit int
	Now   time.Time
}

type Batch struct {
	Sessions []sessions.Summary
	Offset   int
	Total    int
	Done     bool
}

func LoadBatch(ctx context.Context, opts Options, offset, batchSize int) (Batch, error) {
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	files, err := sessions.Discover(sessions.DiscoverOptions{
		Root:  opts.Root,
		Days:  opts.Days,
		Limit: opts.Limit,
		Now:   now,
	})
	if err != nil {
		return Batch{}, err
	}
	if offset >= len(files) {
		return Batch{Offset: offset, Total: len(files), Done: true}, nil
	}
	end := offset + batchSize
	if end > len(files) {
		end = len(files)
	}
	batch := Batch{Offset: offset, Total: len(files), Done: end >= len(files)}
	for _, file := range files[offset:end] {
		select {
		case <-ctx.Done():
			return Batch{}, ctx.Err()
		default:
		}
		summary, err := codex.ParseFile(file.Path, now)
		if err != nil {
			summary = sessions.Summary{
				ID:        file.ID,
				Path:      file.Path,
				Agent:     "codex",
				UpdatedAt: file.ModTime,
				Status:    sessions.StatusErrored,
				Diagnostics: []sessions.Diagnostic{{
					Message: err.Error(),
				}},
			}
		}
		batch.Sessions = append(batch.Sessions, summary)
	}
	return batch, nil
}
