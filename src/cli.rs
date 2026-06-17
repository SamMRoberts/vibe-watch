//! Command-line interface for vibe-watch.

use std::path::PathBuf;

use anyhow::Result;
use clap::{Args, Parser, Subcommand, ValueEnum};

use crate::session_scan::{self, FormatFilter};
use crate::{output, tui};

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
    /// Browse repositories and sessions in an interactive terminal dashboard.
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
    /// Optional session log, session directory, or scan root. Omit to scan default roots.
    path: Option<PathBuf>,
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
    let analytics = session_scan::load_analytics(&args.path, args.format.into())?;
    if args.json {
        output::print_json(&analytics)?;
    } else {
        output::print_table(&analytics);
    }
    Ok(())
}

fn run_tui(args: TuiArgs) -> Result<()> {
    tui::run_browser(args.path, args.format.into())
}

impl From<Format> for FormatFilter {
    fn from(value: Format) -> Self {
        match value {
            Format::Auto => FormatFilter::Auto,
            Format::Vscode => FormatFilter::Vscode,
            Format::Cli => FormatFilter::Cli,
        }
    }
}
