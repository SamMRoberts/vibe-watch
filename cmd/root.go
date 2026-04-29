package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/analysis"
	"github.com/SamMRoberts/vibe-watch/internal/codex/sessions"
	"github.com/SamMRoberts/vibe-watch/internal/report"
	"github.com/spf13/cobra"
)

type options struct {
	sessionRoot string
	since       string
	until       string
	format      string
	limit       int
}

func Execute() error {
	return newRootCommand().Execute()
}

func newRootCommand() *cobra.Command {
	opts := &options{}
	root := &cobra.Command{
		Use:   "vibe-watch",
		Short: "Analyze local coding-agent session history",
		Long: "vibe-watch scans local Codex JSONL session history and reports aggregate metrics, " +
			"data-quality signals, and rule-based workflow suggestions without printing raw session content.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&opts.sessionRoot, "session-root", defaultSessionRoot(), "Codex session root")
	root.PersistentFlags().StringVar(&opts.since, "since", "", "include sessions on or after this date (YYYY-MM-DD)")
	root.PersistentFlags().StringVar(&opts.until, "until", "", "include sessions on or before this date (YYYY-MM-DD)")
	root.PersistentFlags().StringVar(&opts.format, "format", "text", "output format: text or json")
	root.PersistentFlags().IntVar(&opts.limit, "limit", 0, "maximum number of session files to scan (0 means no limit)")

	root.AddCommand(newScanCommand(opts))
	root.AddCommand(newStatsCommand(opts))
	root.AddCommand(newSuggestCommand(opts))
	root.AddCommand(newReportCommand(opts))
	root.AddCommand(newTUICommand(opts))

	return root
}

func defaultSessionRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.codex/sessions"
	}
	return filepath.Join(home, ".codex", "sessions")
}

func buildScanOptions(opts *options) (sessions.ScanOptions, error) {
	scanOpts := sessions.ScanOptions{
		Root:  opts.sessionRoot,
		Limit: opts.limit,
	}
	var err error
	scanOpts.Since, err = parseDateFlag("since", opts.since)
	if err != nil {
		return sessions.ScanOptions{}, err
	}
	scanOpts.Until, err = parseDateFlag("until", opts.until)
	if err != nil {
		return sessions.ScanOptions{}, err
	}
	if scanOpts.Since != nil && scanOpts.Until != nil && scanOpts.Since.After(*scanOpts.Until) {
		return sessions.ScanOptions{}, errors.New("--since must be on or before --until")
	}
	return scanOpts, nil
}

func parseDateFlag(name, value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return nil, fmt.Errorf("--%s must use YYYY-MM-DD: %w", name, err)
	}
	return &parsed, nil
}

func runAnalysis(opts *options) (analysis.Result, error) {
	scanOpts, err := buildScanOptions(opts)
	if err != nil {
		return analysis.Result{}, err
	}
	return analysis.Run(scanOpts)
}

func writeOutput(cmd *cobra.Command, opts *options, payload any, text func() string) error {
	switch opts.format {
	case "text":
		_, err := fmt.Fprint(cmd.OutOrStdout(), text())
		return err
	case "json":
		return report.WriteJSON(cmd.OutOrStdout(), payload)
	default:
		return fmt.Errorf("--format must be text or json, got %q", opts.format)
	}
}
