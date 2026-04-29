package cmd

import (
	"fmt"
	"time"

	watchtui "github.com/SamMRoberts/vibe-watch/internal/tui"
	"github.com/SamMRoberts/vibe-watch/internal/watcher"
	"github.com/spf13/cobra"
)

func newTUICommand(opts *options) *cobra.Command {
	var interval time.Duration
	var eventLimit int
	var once bool
	command := &cobra.Command{
		Use:   "tui",
		Short: "Open the real-time Codex session monitor",
		RunE: func(cmd *cobra.Command, args []string) error {
			if once {
				snapshot, err := watcher.Capture(watcher.Options{
					Root:       opts.sessionRoot,
					Limit:      opts.limit,
					EventLimit: eventLimit,
				})
				if err != nil {
					return err
				}
				_, err = fmt.Fprint(cmd.OutOrStdout(), watchtui.RenderSnapshot(snapshot))
				return err
			}
			return watchtui.Run(watchtui.RunOptions{
				SessionRoot: opts.sessionRoot,
				Limit:       opts.limit,
				EventLimit:  eventLimit,
				Interval:    interval,
			})
		},
	}
	command.Flags().DurationVar(&interval, "interval", 2*time.Second, "polling interval for live JSONL updates")
	command.Flags().IntVar(&eventLimit, "event-limit", 12, "maximum recent events to display")
	command.Flags().BoolVar(&once, "once", false, "render one sanitized snapshot and exit")
	return command
}
