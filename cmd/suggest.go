package cmd

import (
	"github.com/SamMRoberts/vibe-watch/internal/report"
	"github.com/spf13/cobra"
)

func newSuggestCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "suggest",
		Short: "Generate evidence-backed workflow suggestions",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := runAnalysis(opts)
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts, result.Suggestions, func() string {
				return report.FormatSuggestions(result.Suggestions)
			})
		},
	}
}
