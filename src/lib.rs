//! vibe-watch: monitor Copilot coding-agent token and AI credit usage.
//!
//! The library is split into focused modules:
//! - [`pricing`]: model AIC rate tables and credit math.
//! - [`chat_log`]: parser for VS Code Copilot Chat `.jsonl` session logs.
//! - [`analytics`]: typed metrics (per-turn tokens, credits, percentages, timeline).
//! - [`output`]: human-readable table and JSON rendering.
//! - [`cli`]: command-line entry point.

pub mod analytics;
pub mod chat_log;
pub mod cli;
pub mod output;
pub mod pricing;
