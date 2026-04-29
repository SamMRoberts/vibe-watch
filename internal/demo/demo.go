// Package demo provides hardcoded simulated sessions for screenshot and demo purposes.
package demo

import (
	"time"

	"github.com/SamMRoberts/vibe-watch/internal/models"
)

// Sessions returns a rich set of simulated sessions spanning two days across
// Copilot CLI, Copilot Chat, and Codex CLI agents.
func Sessions() []*models.Session {
	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	yesterday := today.Add(-24 * time.Hour)

	var all []*models.Session

	// ── Today ────────────────────────────────────────────────────────────────

	// Active Copilot CLI session — vibe-watch project
	all = append(all, &models.Session{
		ID:          "cp-vw-2026-0001",
		AgentType:   models.AgentCopilot,
		ProjectPath: "~/dev/vibe-watch",
		StartTime:   today.Add(9 * time.Hour),
		LastUpdated: now.Add(-30 * time.Second),
		IsActive:    true,
		LogPath:     "/home/user/.copilot/session-state/cp-vw-2026-0001/events.jsonl",
		TotalTokens: models.TokenUsage{
			InputTokens:  52_840,
			OutputTokens: 18_320,
			CacheReads:   14_200,
			CacheWrites:  6_480,
		},
		Messages: copilotVibeWatchMessages(today.Add(9 * time.Hour), now),
	})

	// Completed Copilot CLI session — api-gateway project
	all = append(all, &models.Session{
		ID:          "cp-api-2026-0002",
		AgentType:   models.AgentCopilot,
		ProjectPath: "~/dev/api-gateway",
		StartTime:   today.Add(7*time.Hour + 15*time.Minute),
		LastUpdated: today.Add(8*time.Hour + 42*time.Minute),
		IsActive:    false,
		LogPath:     "/home/user/.copilot/session-state/cp-api-2026-0002/events.jsonl",
		TotalTokens: models.TokenUsage{
			InputTokens:  23_150,
			OutputTokens: 7_890,
			CacheReads:   8_340,
			CacheWrites:  2_100,
		},
		Messages: copilotAPIGatewayMessages(today.Add(7*time.Hour+15*time.Minute), today.Add(8*time.Hour+42*time.Minute)),
	})

	// Active Codex CLI session — ml-pipeline project
	all = append(all, &models.Session{
		ID:          "cdx-ml-2026-0003",
		AgentType:   models.AgentCodex,
		ProjectPath: "~/dev/ml-pipeline",
		StartTime:   today.Add(10*time.Hour + 5*time.Minute),
		LastUpdated: now.Add(-90 * time.Second),
		IsActive:    true,
		LogPath:     "/home/user/.codex/sessions/2026/04/29/rollout-session.jsonl",
		TotalTokens: models.TokenUsage{
			InputTokens:  89_420,
			OutputTokens: 31_680,
			CacheReads:   22_140,
			CacheWrites:  9_870,
		},
		Messages: codexMLPipelineMessages(today.Add(10*time.Hour+5*time.Minute), now),
	})

	// ── Yesterday ────────────────────────────────────────────────────────────

	// Completed Copilot Chat session — frontend-app project
	all = append(all, &models.Session{
		ID:          "vscode-copilot-chat-fe-2026-0004",
		AgentType:   models.AgentCopilotChat,
		ProjectPath: "~/dev/frontend-app",
		StartTime:   yesterday.Add(14*time.Hour + 30*time.Minute),
		LastUpdated: yesterday.Add(17*time.Hour + 12*time.Minute),
		IsActive:    false,
		LogPath:     "/home/user/.vscode/workspaceStorage/abc123/state.vscdb",
		TotalTokens: models.TokenUsage{
			InputTokens:  112_780,
			OutputTokens: 44_230,
			CacheReads:   38_410,
			CacheWrites:  15_690,
		},
		Messages: copilotChatFrontendMessages(yesterday.Add(14*time.Hour+30*time.Minute), yesterday.Add(17*time.Hour+12*time.Minute)),
	})

	// Completed Codex CLI session — data-sync project
	all = append(all, &models.Session{
		ID:          "cdx-ds-2026-0005",
		AgentType:   models.AgentCodex,
		ProjectPath: "~/dev/data-sync",
		StartTime:   yesterday.Add(11*time.Hour + 20*time.Minute),
		LastUpdated: yesterday.Add(11*time.Hour + 58*time.Minute),
		IsActive:    false,
		LogPath:     "/home/user/.codex/sessions/2026/04/28/rollout-session.jsonl",
		TotalTokens: models.TokenUsage{
			InputTokens:  19_340,
			OutputTokens: 6_150,
			CacheReads:   4_820,
			CacheWrites:  1_640,
		},
		Messages: codexDataSyncMessages(yesterday.Add(11*time.Hour+20*time.Minute), yesterday.Add(11*time.Hour+58*time.Minute)),
	})

	// Completed Copilot CLI session — infra-tools yesterday
	all = append(all, &models.Session{
		ID:          "cp-infra-2026-0006",
		AgentType:   models.AgentCopilot,
		ProjectPath: "~/dev/infra-tools",
		StartTime:   yesterday.Add(9*time.Hour + 45*time.Minute),
		LastUpdated: yesterday.Add(10*time.Hour + 31*time.Minute),
		IsActive:    false,
		LogPath:     "/home/user/.copilot/session-state/cp-infra-2026-0006/events.jsonl",
		TotalTokens: models.TokenUsage{
			InputTokens:  31_200,
			OutputTokens: 10_450,
			CacheReads:   9_870,
			CacheWrites:  3_280,
		},
		Messages: copilotInfraMessages(yesterday.Add(9*time.Hour+45*time.Minute), yesterday.Add(10*time.Hour+31*time.Minute)),
	})

	return all
}

// ── Copilot CLI: vibe-watch (active) ─────────────────────────────────────────

func copilotVibeWatchMessages(start, now time.Time) []models.Message {
	t := start
	tick := func(d time.Duration) time.Time { t = t.Add(d); return t }

	return []models.Message{
		{Role: "user", Timestamp: tick(0), Content: "Add a --demo flag to vibe-watch that loads hardcoded simulated sessions so we can take screenshots without needing real agent log files."},
		{Role: "assistant", Timestamp: tick(8 * time.Second), Content: "I'll add a --demo flag to the watch command. Let me first explore the codebase to understand the watcher and session model before making changes.", Tokens: models.TokenUsage{InputTokens: 4820, OutputTokens: 420}},
		toolStart(tick(0), "read_file", "cp-t1", "cp-ia", "cmd/watch.go"),
		toolEnd(tick(2*time.Second), "cp-t1", "cp-ia", "read_file", "cmd/watch.go — 43 lines"),
		toolStart(tick(0), "read_file", "cp-t2", "cp-ia", "internal/watcher/watcher.go"),
		toolEnd(tick(2*time.Second), "cp-t2", "cp-ia", "read_file", "internal/watcher/watcher.go — 98 lines"),
		{Role: "assistant", Timestamp: tick(5 * time.Second), Content: "I'll add a NewStatic constructor to the watcher that immediately publishes hardcoded sessions without polling. Then I'll create an internal/demo package with rich simulated sessions for Copilot CLI, Copilot Chat, and Codex.", Tokens: models.TokenUsage{InputTokens: 6140, OutputTokens: 680}},
		toolStart(tick(0), "write_file", "cp-t3", "cp-ia", "internal/demo/demo.go"),
		toolEnd(tick(3*time.Second), "cp-t3", "cp-ia", "write_file", "created 240 lines"),
		toolStart(tick(0), "write_file", "cp-t4", "cp-ia", "internal/watcher/watcher.go"),
		toolEnd(tick(2*time.Second), "cp-t4", "cp-ia", "write_file", "modified — added NewStatic"),
		toolStart(tick(0), "run_shell", "cp-t5", "cp-ia", "go build ./..."),
		toolEnd(tick(4*time.Second), "cp-t5", "cp-ia", "run_shell", "exit: 0"),
		toolStart(tick(0), "run_shell", "cp-t6", "cp-ia", "go test ./..."),
		toolEnd(tick(6*time.Second), "cp-t6", "cp-ia", "run_shell", "exit: 0  ok github.com/SamMRoberts/vibe-watch/..."),
		{Role: "assistant", Timestamp: tick(3 * time.Second), Content: "Done. The --demo flag is now available. Run `vibe-watch --demo` to launch the TUI with simulated data. All tests pass.", Tokens: models.TokenUsage{InputTokens: 7200, OutputTokens: 310}},
		{Role: "user", Timestamp: tick(2 * time.Minute), Content: "Great! Now write a Python script to capture 5 screenshots of the TUI in different states using pyte and Pillow."},
		{Role: "assistant", Timestamp: tick(10 * time.Second), Content: "I'll create a screenshot capture script that opens the binary in a PTY, navigates between views, and saves each screen state as a PNG image.", Tokens: models.TokenUsage{InputTokens: 8340, OutputTokens: 520}},
		toolStart(tick(0), "write_file", "cp-t7", "cp-ib", "/tmp/screenshot.py"),
		toolEnd(tick(4*time.Second), "cp-t7", "cp-ib", "write_file", "created 180 lines"),
		toolStart(tick(0), "run_shell", "cp-t8", "cp-ib", "python3 /tmp/screenshot.py"),
		toolEnd(tick(15*time.Second), "cp-t8", "cp-ib", "run_shell", "saved 5 screenshots → screenshots/"),
		{Role: "assistant", Timestamp: now.Add(-30 * time.Second), Content: "Screenshots captured and saved. Now updating the README to reference the new images…", Tokens: models.TokenUsage{InputTokens: 9120, OutputTokens: 280}},
	}
}

// ── Copilot CLI: api-gateway (completed) ─────────────────────────────────────

func copilotAPIGatewayMessages(start, end time.Time) []models.Message {
	t := start
	tick := func(d time.Duration) time.Time { t = t.Add(d); return t }

	return []models.Message{
		{Role: "user", Timestamp: tick(0), Content: "Refactor the authentication middleware to support both JWT and API key strategies with a unified interface."},
		{Role: "assistant", Timestamp: tick(7 * time.Second), Content: "I'll refactor the auth middleware to use a Strategy pattern. Let me look at the current implementation first.", Tokens: models.TokenUsage{InputTokens: 3210, OutputTokens: 380}},
		toolStart(tick(0), "read_file", "a-t1", "a-ia", "middleware/auth.go"),
		toolEnd(tick(2*time.Second), "a-t1", "a-ia", "read_file", "middleware/auth.go — 87 lines"),
		toolStart(tick(0), "write_file", "a-t2", "a-ia", "middleware/auth.go"),
		toolEnd(tick(3*time.Second), "a-t2", "a-ia", "write_file", "modified — 142 lines"),
		toolStart(tick(0), "write_file", "a-t3", "a-ia", "middleware/auth_test.go"),
		toolEnd(tick(2*time.Second), "a-t3", "a-ia", "write_file", "created — 68 lines"),
		toolStart(tick(0), "run_shell", "a-t4", "a-ia", "go test ./middleware/..."),
		toolEnd(tick(4*time.Second), "a-t4", "a-ia", "run_shell", "exit: 0  ok .../middleware"),
		{Role: "assistant", Timestamp: end, Content: "Refactoring complete. The auth middleware now uses a Strategy interface with JWT and APIKey implementations. Tests pass.", Tokens: models.TokenUsage{InputTokens: 5840, OutputTokens: 420}},
	}
}

// ── Codex CLI: ml-pipeline (active) ──────────────────────────────────────────

func codexMLPipelineMessages(start, now time.Time) []models.Message {
	t := start
	tick := func(d time.Duration) time.Time { t = t.Add(d); return t }

	return []models.Message{
		sessionStartMsg(tick(0), "cdx-ml-2026-0003", "~/dev/ml-pipeline"),
		{Role: "user", Timestamp: tick(500 * time.Millisecond), Content: "Implement a distributed feature store with Redis caching, Postgres persistence, and a gRPC API for real-time feature serving."},
		{Role: "assistant", Timestamp: tick(9 * time.Second), Content: "I'll design and implement a distributed feature store. Starting with the gRPC service definition.", Tokens: models.TokenUsage{InputTokens: 5240, OutputTokens: 610}},
		toolStart(tick(0), "exec_command", "m-t1", "m-ia", "ls -la features/"),
		toolEnd(tick(2*time.Second), "m-t1", "m-ia", "exec_command", "exit: 0  (empty directory)"),
		toolStart(tick(0), "write_file", "m-t2", "m-ia", "features/proto/feature_store.proto"),
		toolEnd(tick(3*time.Second), "m-t2", "m-ia", "write_file", "created — 62 lines"),
		toolStart(tick(0), "exec_command", "m-t3", "m-ia", "protoc --go_out=. --go-grpc_out=. features/proto/*.proto"),
		toolEnd(tick(5*time.Second), "m-t3", "m-ia", "exec_command", "exit: 0"),
		toolStart(tick(0), "write_file", "m-t4", "m-ia", "features/store/redis_cache.go"),
		toolEnd(tick(3*time.Second), "m-t4", "m-ia", "write_file", "created — 124 lines"),
		toolStart(tick(0), "write_file", "m-t5", "m-ia", "features/store/postgres_store.go"),
		toolEnd(tick(4*time.Second), "m-t5", "m-ia", "write_file", "created — 198 lines"),
		{Role: "assistant", Timestamp: tick(6 * time.Second), Content: "Core store layers implemented. Now building the gRPC server and wiring up the cache-aside pattern.", Tokens: models.TokenUsage{InputTokens: 12480, OutputTokens: 890}},
		toolStart(tick(0), "write_file", "m-t6", "m-ib", "features/server/grpc.go"),
		toolEnd(tick(4*time.Second), "m-t6", "m-ib", "write_file", "created — 211 lines"),
		toolStart(tick(0), "exec_command", "m-t7", "m-ib", "go build ./features/..."),
		toolEnd(tick(5*time.Second), "m-t7", "m-ib", "exec_command", "exit: 0"),
		toolStart(tick(0), "exec_command", "m-t8", "m-ib", "go test ./features/..."),
		toolEnd(tick(8*time.Second), "m-t8", "m-ib", "exec_command", "exit: 0  ok .../features  coverage: 78.3%"),
		{Role: "user", Timestamp: tick(3 * time.Minute), Content: "Looks good. Now add a TTL-based eviction policy and metrics collection using Prometheus."},
		{Role: "assistant", Timestamp: now.Add(-90 * time.Second), Content: "Adding TTL eviction and Prometheus metrics. I'll instrument GetFeature, SetFeature, and cache hit/miss rates.", Tokens: models.TokenUsage{InputTokens: 18920, OutputTokens: 1240}},
		toolStart(tick(0), "write_file", "m-t9", "m-ic", "features/store/ttl_eviction.go"),
		toolEnd(tick(3*time.Second), "m-t9", "m-ic", "write_file", "created — 89 lines"),
		toolStart(tick(0), "write_file", "m-t10", "m-ic", "features/metrics/prometheus.go"),
		toolEnd(now.Add(-90*time.Second), "m-t10", "m-ic", "write_file", "created — 76 lines"),
	}
}

// ── Copilot Chat: frontend-app (completed yesterday) ─────────────────────────

func copilotChatFrontendMessages(start, end time.Time) []models.Message {
	t := start
	tick := func(d time.Duration) time.Time { t = t.Add(d); return t }

	return []models.Message{
		{Role: "user", Timestamp: tick(0), Content: "Help me migrate this React app from class components to functional components with hooks. Start with the Dashboard component."},
		{Role: "assistant", Timestamp: tick(6 * time.Second), Content: "I'll help you migrate the Dashboard component. Let me review the current implementation.", Tokens: models.TokenUsage{InputTokens: 4120, OutputTokens: 340}},
		toolStart(tick(0), "read_file", "f-t1", "f-ia", "src/components/Dashboard.jsx"),
		toolEnd(tick(2*time.Second), "f-t1", "f-ia", "read_file", "Dashboard.jsx — 234 lines"),
		toolStart(tick(0), "write_file", "f-t2", "f-ia", "src/components/Dashboard.jsx"),
		toolEnd(tick(4*time.Second), "f-t2", "f-ia", "write_file", "modified — 187 lines"),
		{Role: "user", Timestamp: tick(5 * time.Minute), Content: "Great! Now migrate the UserProfile and Settings components too."},
		{Role: "assistant", Timestamp: tick(8 * time.Second), Content: "Migrating UserProfile and Settings components.", Tokens: models.TokenUsage{InputTokens: 8340, OutputTokens: 560}},
		toolStart(tick(0), "read_file", "f-t3", "f-ib", "src/components/UserProfile.jsx"),
		toolEnd(tick(2*time.Second), "f-t3", "f-ib", "read_file", "UserProfile.jsx — 178 lines"),
		toolStart(tick(0), "write_file", "f-t4", "f-ib", "src/components/UserProfile.jsx"),
		toolEnd(tick(3*time.Second), "f-t4", "f-ib", "write_file", "modified — 142 lines"),
		toolStart(tick(0), "read_file", "f-t5", "f-ib", "src/components/Settings.jsx"),
		toolEnd(tick(2*time.Second), "f-t5", "f-ib", "read_file", "Settings.jsx — 312 lines"),
		toolStart(tick(0), "write_file", "f-t6", "f-ib", "src/components/Settings.jsx"),
		toolEnd(tick(5*time.Second), "f-t6", "f-ib", "write_file", "modified — 264 lines"),
		{Role: "user", Timestamp: tick(8 * time.Minute), Content: "Add error boundaries and suspense wrappers for the async data fetching hooks."},
		{Role: "assistant", Timestamp: tick(9 * time.Second), Content: "Adding ErrorBoundary and Suspense wrappers for async data fetching.", Tokens: models.TokenUsage{InputTokens: 16280, OutputTokens: 980}},
		toolStart(tick(0), "write_file", "f-t7", "f-ic", "src/components/ErrorBoundary.jsx"),
		toolEnd(tick(3*time.Second), "f-t7", "f-ic", "write_file", "created — 64 lines"),
		toolStart(tick(0), "write_file", "f-t8", "f-ic", "src/hooks/useAsyncData.js"),
		toolEnd(tick(2*time.Second), "f-t8", "f-ic", "write_file", "created — 48 lines"),
		toolStart(tick(0), "run_shell", "f-t9", "f-ic", "npm test -- --watchAll=false"),
		toolEnd(tick(12*time.Second), "f-t9", "f-ic", "run_shell", "exit: 0  Tests: 47 passed"),
		{Role: "assistant", Timestamp: end, Content: "Migration complete. All 3 components converted to functional with hooks. Error boundaries and Suspense wrappers added. 47 tests passing.", Tokens: models.TokenUsage{InputTokens: 24680, OutputTokens: 1480}},
	}
}

// ── Codex CLI: data-sync (completed yesterday) ───────────────────────────────

func codexDataSyncMessages(start, end time.Time) []models.Message {
	t := start
	tick := func(d time.Duration) time.Time { t = t.Add(d); return t }

	return []models.Message{
		sessionStartMsg(tick(0), "cdx-ds-2026-0005", "~/dev/data-sync"),
		{Role: "user", Timestamp: tick(500 * time.Millisecond), Content: "Fix the race condition in the bidirectional sync engine where concurrent writes to the same record cause data loss."},
		{Role: "assistant", Timestamp: tick(8 * time.Second), Content: "I'll investigate the sync engine for the race condition. Let me look at the write path.", Tokens: models.TokenUsage{InputTokens: 2840, OutputTokens: 320}},
		toolStart(tick(0), "read_file", "d-t1", "d-ia", "sync/engine.go"),
		toolEnd(tick(2*time.Second), "d-t1", "d-ia", "read_file", "engine.go — 186 lines"),
		toolStart(tick(0), "exec_command", "d-t2", "d-ia", "go test -race ./sync/..."),
		toolEnd(tick(6*time.Second), "d-t2", "d-ia", "exec_command", "exit: 1  DATA RACE detected in WriteRecord"),
		{Role: "assistant", Timestamp: tick(5 * time.Second), Content: "Found the race — concurrent writes to the same record key lack proper mutex locking. I'll add a per-key lock using sync.Map and fix the write path.", Tokens: models.TokenUsage{InputTokens: 5680, OutputTokens: 440}},
		toolStart(tick(0), "write_file", "d-t3", "d-ib", "sync/engine.go"),
		toolEnd(tick(4*time.Second), "d-t3", "d-ib", "write_file", "modified — 201 lines"),
		toolStart(tick(0), "exec_command", "d-t4", "d-ib", "go test -race ./sync/..."),
		toolEnd(tick(7*time.Second), "d-t4", "d-ib", "exec_command", "exit: 0  ok .../sync  no races detected"),
		{Role: "assistant", Timestamp: end, Content: "Race condition fixed. Per-key locking via sync.Map ensures safe concurrent writes. All sync tests pass with -race enabled.", Tokens: models.TokenUsage{InputTokens: 7920, OutputTokens: 360}},
	}
}

// ── Copilot CLI: infra-tools (completed yesterday) ───────────────────────────

func copilotInfraMessages(start, end time.Time) []models.Message {
	t := start
	tick := func(d time.Duration) time.Time { t = t.Add(d); return t }

	return []models.Message{
		{Role: "user", Timestamp: tick(0), Content: "Write a Terraform module for a multi-region EKS cluster with node auto-scaling, pod disruption budgets, and IRSA for S3 access."},
		{Role: "assistant", Timestamp: tick(9 * time.Second), Content: "I'll create a reusable Terraform module for multi-region EKS with the requested features.", Tokens: models.TokenUsage{InputTokens: 3640, OutputTokens: 480}},
		toolStart(tick(0), "write_file", "i-t1", "i-ia", "modules/eks/main.tf"),
		toolEnd(tick(5*time.Second), "i-t1", "i-ia", "write_file", "created — 298 lines"),
		toolStart(tick(0), "write_file", "i-t2", "i-ia", "modules/eks/variables.tf"),
		toolEnd(tick(3*time.Second), "i-t2", "i-ia", "write_file", "created — 84 lines"),
		toolStart(tick(0), "write_file", "i-t3", "i-ia", "modules/eks/outputs.tf"),
		toolEnd(tick(2*time.Second), "i-t3", "i-ia", "write_file", "created — 42 lines"),
		toolStart(tick(0), "run_shell", "i-t4", "i-ia", "terraform validate"),
		toolEnd(tick(3*time.Second), "i-t4", "i-ia", "run_shell", "exit: 0  Success! The configuration is valid."),
		{Role: "assistant", Timestamp: end, Content: "Terraform module ready. Includes multi-region EKS cluster, Cluster Autoscaler, PodDisruptionBudgets, and IRSA roles for S3 access. Validated successfully.", Tokens: models.TokenUsage{InputTokens: 9480, OutputTokens: 680}},
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func toolStart(ts time.Time, name, id, interactionID, label string) models.Message {
	return models.Message{
		Role:      "tool",
		Timestamp: ts,
		Content:   label,
		Meta: models.ActivityMeta{
			Kind:          models.ActivityKindTool,
			Lifecycle:     models.ActivityLifecycleStarted,
			ID:            id,
			InteractionID: interactionID,
			Label:         name,
		},
	}
}

func toolEnd(ts time.Time, id, interactionID, name, result string) models.Message {
	return models.Message{
		Role:      "tool",
		Timestamp: ts,
		Content:   result,
		Meta: models.ActivityMeta{
			Kind:          models.ActivityKindTool,
			Lifecycle:     models.ActivityLifecycleCompleted,
			ID:            id,
			InteractionID: interactionID,
			Label:         name,
		},
	}
}

func sessionStartMsg(ts time.Time, id, cwd string) models.Message {
	return models.Message{
		Role:      "tool",
		Timestamp: ts,
		Content:   "session started — " + cwd,
		Meta: models.ActivityMeta{
			Kind:      models.ActivityKindSession,
			Lifecycle: models.ActivityLifecycleStarted,
			ID:        id,
			Label:     "session",
		},
	}
}
