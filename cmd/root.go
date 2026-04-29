package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/codex"
	"github.com/SamMRoberts/vibe-watch/internal/sessions"
	"github.com/SamMRoberts/vibe-watch/internal/theme"
	"github.com/SamMRoberts/vibe-watch/internal/tui"
	"github.com/spf13/cobra"
)

type Config struct {
	SessionsRoot string
	Days         int
	Limit        int
	Refresh      time.Duration
	Theme        string
}

type runFunc func(context.Context, Config) error

func Execute() error {
	return NewRootCommand(nil).Execute()
}

func NewRootCommand(run runFunc) *cobra.Command {
	cfg := Config{
		SessionsRoot: sessions.DefaultRoot(),
		Days:         7,
		Limit:        200,
		Refresh:      2 * time.Second,
		Theme:        "auto",
	}
	if run == nil {
		run = func(ctx context.Context, cfg Config) error {
			return tui.Run(ctx, tui.Options{
				SessionsRoot: cfg.SessionsRoot,
				Days:         cfg.Days,
				Limit:        cfg.Limit,
				Refresh:      cfg.Refresh,
				Theme:        cfg.Theme,
			})
		}
	}

	root := &cobra.Command{
		Use:          "vibe-watch",
		Short:        "Monitor local coding-agent sessions",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateConfig(cfg); err != nil {
				return err
			}
			return run(cmd.Context(), cfg)
		},
	}
	root.PersistentFlags().StringVar(&cfg.SessionsRoot, "sessions-root", cfg.SessionsRoot, "root directory for Codex sessions")
	root.PersistentFlags().IntVar(&cfg.Days, "days", cfg.Days, "number of recent days to scan")
	root.PersistentFlags().IntVar(&cfg.Limit, "limit", cfg.Limit, "maximum sessions to load")
	root.PersistentFlags().DurationVar(&cfg.Refresh, "refresh", cfg.Refresh, "polling refresh interval")
	root.PersistentFlags().StringVar(&cfg.Theme, "theme", cfg.Theme, "theme name: auto, dark, light, mono")

	root.AddCommand(newSessionsCommand(&cfg))
	return root
}

func newSessionsCommand(cfg *Config) *cobra.Command {
	sessionsCmd := &cobra.Command{
		Use:   "sessions",
		Short: "Inspect Codex sessions without launching the TUI",
	}
	sessionsCmd.AddCommand(newSessionsListCommand(cfg), newSessionsInspectCommand(cfg))
	return sessionsCmd
}

func newSessionsListCommand(cfg *Config) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List discovered Codex sessions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateConfig(*cfg); err != nil {
				return err
			}
			summaries, err := loadSummaries(*cfg)
			if err != nil {
				return err
			}
			if jsonOut {
				return writeJSON(cmd.OutOrStdout(), summaries)
			}
			for _, summary := range summaries {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", summary.ID, summary.Agent, summary.Status, summary.PromptPreview)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "write JSON output")
	return cmd
}

func newSessionsInspectCommand(cfg *Config) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "inspect <session-id-or-path>",
		Short: "Inspect one Codex session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateConfig(*cfg); err != nil {
				return err
			}
			path, err := sessions.Resolve(cfg.SessionsRoot, args[0], sessions.DiscoverOptions{
				Root: cfg.SessionsRoot,
				Days: cfg.Days,
				Now:  time.Now(),
			})
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("session not found: %s", args[0])
				}
				return err
			}
			summary, err := codex.ParseFile(path, time.Now())
			if err != nil {
				return err
			}
			if jsonOut {
				return writeJSON(cmd.OutOrStdout(), summary)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n%s\n", summary.ID, summary.Agent, summary.Status, summary.PromptPreview)
			for _, activity := range summary.Activities {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s: %s\n", activity.Kind, activity.Title)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "write JSON output")
	return cmd
}

func validateConfig(cfg Config) error {
	if cfg.SessionsRoot == "" {
		return errors.New("--sessions-root is required")
	}
	if cfg.Days < 0 {
		return errors.New("--days must be zero or greater")
	}
	if cfg.Limit < 0 {
		return errors.New("--limit must be zero or greater")
	}
	if cfg.Refresh <= 0 {
		return errors.New("--refresh must be greater than zero")
	}
	for _, name := range theme.Names() {
		if cfg.Theme == name {
			return nil
		}
	}
	return fmt.Errorf("unknown theme %q", cfg.Theme)
}

func loadSummaries(cfg Config) ([]sessions.Summary, error) {
	files, err := sessions.Discover(sessions.DiscoverOptions{
		Root:  cfg.SessionsRoot,
		Days:  cfg.Days,
		Limit: cfg.Limit,
		Now:   time.Now(),
	})
	if err != nil {
		return nil, err
	}
	summaries := make([]sessions.Summary, 0, len(files))
	now := time.Now()
	for _, file := range files {
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
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
