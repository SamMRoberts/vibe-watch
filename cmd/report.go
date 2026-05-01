package cmd

import (
	"github.com/SamMRoberts/vibe-watch/internal/report"
	"github.com/spf13/cobra"
)

func newReportCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "report",
		Short: "Show scan summary, metrics, and suggestions",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := runAnalysis(opts)
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts, result, func() string {
				return report.FormatFull(result)
			})
		},
	}
}
