package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newTUICommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the interactive session browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "Interactive TUI is planned; use scan, stats, report, or suggest for the initial CLI workflow.")
			return err
		},
	}
}
