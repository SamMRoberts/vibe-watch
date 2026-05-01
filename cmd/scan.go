package cmd

import (
	"github.com/SamMRoberts/vibe-watch/internal/report"
	"github.com/spf13/cobra"
)

func newScanCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Scan Codex sessions and summarize data quality",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := runAnalysis(opts)
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts, result.Summary, func() string {
				return report.FormatScanSummary(result.Summary)
			})
		},
	}
}
