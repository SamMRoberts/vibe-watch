package cmd

import (
	"github.com/SamMRoberts/vibe-watch/internal/report"
	"github.com/spf13/cobra"
)

func newStatsCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show aggregate Codex session statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := runAnalysis(opts)
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts, result.Metrics, func() string {
				return report.FormatMetrics(result.Metrics)
			})
		},
	}
}
