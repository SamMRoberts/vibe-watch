package cmd

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/SamMRoberts/vibe-watch/internal/agents"
	"github.com/SamMRoberts/vibe-watch/internal/tui"
	"github.com/SamMRoberts/vibe-watch/internal/watcher"
)

var (
	flagAgent   string
	flagRefresh int
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Start the TUI dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		registry := agents.NewRegistry()
		w := watcher.New(registry, time.Duration(flagRefresh)*time.Second)

		m := tui.NewApp(w, flagAgent)
		p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

		if _, err := p.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "Error running TUI:", err)
			return err
		}
		return nil
	},
}

func init() {
	watchCmd.Flags().StringVar(&flagAgent, "agent", "", "Filter to specific agent (claude, codex, copilot, chat, amazonq)")
	watchCmd.Flags().IntVar(&flagRefresh, "refresh", 2, "Refresh interval in seconds")
}
