//! Command-line interface for vibe-watch.

use std::path::PathBuf;

use anyhow::{bail, Context, Result};
use clap::{Args, Parser, Subcommand, ValueEnum};

use crate::analytics::SessionAnalytics;
use crate::workspace::{resolve_repository_path, WorkspaceFormat};
use crate::{chat_log, cli_log, output, tui};

#[derive(Parser)]
#[command(
    name = "vibe-watch",
    version,
    about = "Monitor Copilot token & AI credit usage"
)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Analyze a session log and report token/credit/timeline metrics.
    Analyze(AnalyzeArgs),
    /// Explore a session log in an interactive terminal dashboard.
    Tui(TuiArgs),
}

#[derive(Args)]
struct AnalyzeArgs {
    /// Path to a session log (`.jsonl`).
    path: PathBuf,
    /// Emit JSON instead of a table.
    #[arg(long)]
    json: bool,
    /// Input format. `auto` detects from the file contents.
    #[arg(long, value_enum, default_value_t = Format::Auto)]
    format: Format,
}

#[derive(Args)]
struct TuiArgs {
    /// Path to a session log (`.jsonl`).
    path: PathBuf,
    /// Input format. `auto` detects from the file contents.
    #[arg(long, value_enum, default_value_t = Format::Auto)]
    format: Format,
}

#[derive(Clone, Copy, ValueEnum)]
enum Format {
    /// Detect the format from the file contents.
    Auto,
    /// VS Code Copilot Chat `.jsonl` delta journal.
    Vscode,
    /// Copilot CLI `events.jsonl` event log.
    Cli,
}

/// Parse arguments and dispatch to the requested command.
pub fn run() -> Result<()> {
    let cli = Cli::parse();
    match cli.command {
        Commands::Analyze(args) => analyze(args),
        Commands::Tui(args) => run_tui(args),
    }
}

fn analyze(args: AnalyzeArgs) -> Result<()> {
    let analytics = load_analytics(&args.path, args.format)?;
    if args.json {
        output::print_json(&analytics)?;
    } else {
        output::print_table(&analytics);
    }
    Ok(())
}

fn run_tui(args: TuiArgs) -> Result<()> {
    let analytics = load_analytics(&args.path, args.format)?;
    tui::run(&analytics)
}

/// Read a session log, detect its format, and compute analytics.
fn load_analytics(path: &std::path::Path, format: Format) -> Result<SessionAnalytics> {
    let data =
        std::fs::read_to_string(path).with_context(|| format!("reading {}", path.display()))?;

    let first_line = data
        .lines()
        .find(|line| !line.trim().is_empty())
        .unwrap_or("");

    let resolved = match format {
        Format::Vscode => DetectedFormat::Vscode,
        Format::Cli => DetectedFormat::Cli,
        Format::Auto => {
            if chat_log::looks_like_chat_log(first_line) {
                DetectedFormat::Vscode
            } else if cli_log::looks_like_cli_log(first_line) {
                DetectedFormat::Cli
            } else {
                bail!("unrecognized log format (expected VS Code chat or Copilot CLI .jsonl)");
            }
        }
    };

    let repository_path = resolve_repository_path(path, workspace_format(resolved))?;

    Ok(match resolved {
        DetectedFormat::Vscode => {
            let mut session = chat_log::parse_str(&data)?;
            session.repository_path = repository_path;
            SessionAnalytics::from_chat(&session)
        }
        DetectedFormat::Cli => {
            let mut session = cli_log::parse_str(&data)?;
            session.repository_path = repository_path;
            SessionAnalytics::from_cli(&session)
        }
    })
}

#[derive(Clone, Copy)]
enum DetectedFormat {
    Vscode,
    Cli,
}

fn workspace_format(format: DetectedFormat) -> WorkspaceFormat {
    match format {
        DetectedFormat::Vscode => WorkspaceFormat::Vscode,
        DetectedFormat::Cli => WorkspaceFormat::Cli,
    }
}
