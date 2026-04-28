package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "vibe-watch",
	Short: "Monitor and analyze agentic coding sessions",
	Long: `vibe-watch is a TUI dashboard for monitoring session data from
agentic coding agents like Claude Code, Codex CLI, GitHub Copilot CLI,
and Amazon Q Developer CLI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return watchCmd.RunE(cmd, args)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(watchCmd)
}
