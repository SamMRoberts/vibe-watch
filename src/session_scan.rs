//! Session discovery and loading for the multi-session TUI browser.

use std::cmp::Reverse;
use std::path::{Path, PathBuf};
use std::time::SystemTime;

use anyhow::{bail, Context, Result};

use crate::analytics::SessionAnalytics;
use crate::workspace::{resolve_repository_path, WorkspaceFormat};
use crate::{chat_log, cli_log};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FormatFilter {
    Auto,
    Vscode,
    Cli,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DetectedFormat {
    Vscode,
    Cli,
}

#[derive(Debug, Clone)]
pub struct SessionCandidate {
    pub path: PathBuf,
    pub modified: SystemTime,
}

#[derive(Debug, Clone)]
pub struct LoadedSession {
    pub path: PathBuf,
    pub modified: SystemTime,
    pub analytics: SessionAnalytics,
}

#[derive(Debug, Clone)]
pub struct SessionLoadError {
    pub path: PathBuf,
    pub modified: Option<SystemTime>,
    pub message: String,
}

#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct ScanProgress {
    pub processed: usize,
    pub total: usize,
    pub finished: bool,
}

#[derive(Debug, Clone)]
pub enum ScanEvent {
    Progress(ScanProgress),
    Loaded(Box<LoadedSession>),
    Error(SessionLoadError),
    Finished,
}

pub fn load_analytics(path: &Path, filter: FormatFilter) -> Result<SessionAnalytics> {
    let data =
        std::fs::read_to_string(path).with_context(|| format!("reading {}", path.display()))?;
    let format = detect_format(&data, filter)?;
    let repository_path =
        normalize_repository_path(resolve_repository_path(path, workspace_format(format))?);

    match format {
        DetectedFormat::Vscode => {
            let mut session = chat_log::parse_str(&data)?;
            session.repository_path = repository_path;
            Ok(SessionAnalytics::from_chat(&session))
        }
        DetectedFormat::Cli => {
            let mut session = cli_log::parse_str(&data)?;
            session.repository_path =
                repository_path.or_else(|| normalize_repository_path(session.cwd.clone()));
            Ok(SessionAnalytics::from_cli(&session))
        }
    }
}

fn normalize_repository_path(path: Option<String>) -> Option<String> {
    path.and_then(|value| {
        let trimmed = value.trim();
        (!trimmed.is_empty()).then(|| trimmed.to_string())
    })
}

pub fn discover_sessions(
    root: Option<&Path>,
    filter: FormatFilter,
) -> Result<Vec<SessionCandidate>> {
    let mut candidates = Vec::new();

    if let Some(root) = root {
        discover_from_path(root, filter, &mut candidates)?;
    } else {
        for default_root in default_roots(filter) {
            if default_root.exists() {
                discover_from_path(&default_root, filter, &mut candidates)?;
            }
        }
    }

    sort_candidates(&mut candidates);
    Ok(candidates)
}

pub fn scan_sessions(root: Option<&Path>, filter: FormatFilter) -> Result<Vec<ScanEvent>> {
    let candidates = discover_sessions(root, filter)?;
    Ok(scan_candidates(candidates, filter))
}

pub fn scan_candidates(candidates: Vec<SessionCandidate>, filter: FormatFilter) -> Vec<ScanEvent> {
    let total = candidates.len();
    let mut events = Vec::with_capacity(total.saturating_mul(2).saturating_add(2));
    events.push(ScanEvent::Progress(ScanProgress {
        processed: 0,
        total,
        finished: total == 0,
    }));

    for (index, candidate) in candidates.into_iter().enumerate() {
        match load_analytics(&candidate.path, filter) {
            Ok(analytics) => events.push(ScanEvent::Loaded(Box::new(LoadedSession {
                path: candidate.path.clone(),
                modified: candidate.modified,
                analytics,
            }))),
            Err(error) => events.push(ScanEvent::Error(SessionLoadError {
                path: candidate.path.clone(),
                modified: Some(candidate.modified),
                message: error.to_string(),
            })),
        }
        events.push(ScanEvent::Progress(ScanProgress {
            processed: index + 1,
            total,
            finished: index + 1 == total,
        }));
    }

    events.push(ScanEvent::Finished);
    events
}

fn detect_format(data: &str, filter: FormatFilter) -> Result<DetectedFormat> {
    let first_line = data
        .lines()
        .find(|line| !line.trim().is_empty())
        .unwrap_or("");

    match filter {
        FormatFilter::Vscode => Ok(DetectedFormat::Vscode),
        FormatFilter::Cli => Ok(DetectedFormat::Cli),
        FormatFilter::Auto => {
            if chat_log::looks_like_chat_log(first_line) {
                Ok(DetectedFormat::Vscode)
            } else if cli_log::looks_like_cli_log(first_line) {
                Ok(DetectedFormat::Cli)
            } else {
                bail!("unrecognized log format (expected VS Code chat or Copilot CLI .jsonl)")
            }
        }
    }
}

fn workspace_format(format: DetectedFormat) -> WorkspaceFormat {
    match format {
        DetectedFormat::Vscode => WorkspaceFormat::Vscode,
        DetectedFormat::Cli => WorkspaceFormat::Cli,
    }
}

fn discover_from_path(
    path: &Path,
    filter: FormatFilter,
    candidates: &mut Vec<SessionCandidate>,
) -> Result<()> {
    if path.is_file() {
        maybe_push_candidate(path, filter, candidates)?;
        return Ok(());
    }

    if !path.is_dir() {
        bail!(
            "session scan path does not exist or is not readable: {}",
            path.display()
        );
    }

    discover_directory(path, filter, candidates)
}

fn discover_directory(
    dir: &Path,
    filter: FormatFilter,
    candidates: &mut Vec<SessionCandidate>,
) -> Result<()> {
    let mut entries = std::fs::read_dir(dir)
        .with_context(|| format!("reading directory {}", dir.display()))?
        .collect::<std::io::Result<Vec<_>>>()?;
    entries.sort_by_key(|entry| entry.path());

    for entry in entries {
        let path = entry.path();
        let file_type = entry
            .file_type()
            .with_context(|| format!("reading file type for {}", path.display()))?;
        if file_type.is_dir() {
            discover_directory(&path, filter, candidates)?;
        } else if file_type.is_file() {
            maybe_push_candidate(&path, filter, candidates)?;
        }
    }

    Ok(())
}

fn maybe_push_candidate(
    path: &Path,
    filter: FormatFilter,
    candidates: &mut Vec<SessionCandidate>,
) -> Result<()> {
    if !is_candidate_path(path, filter) {
        return Ok(());
    }

    let modified = std::fs::metadata(path)
        .and_then(|metadata| metadata.modified())
        .unwrap_or(SystemTime::UNIX_EPOCH);
    candidates.push(SessionCandidate {
        path: path.to_path_buf(),
        modified,
    });
    Ok(())
}

fn is_candidate_path(path: &Path, filter: FormatFilter) -> bool {
    let file_name = path
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or("");
    match filter {
        FormatFilter::Cli => file_name == "events.jsonl",
        FormatFilter::Vscode => file_name.ends_with(".jsonl") && file_name != "events.jsonl",
        FormatFilter::Auto => file_name == "events.jsonl" || file_name.ends_with(".jsonl"),
    }
}

fn sort_candidates(candidates: &mut [SessionCandidate]) {
    candidates.sort_by_key(|candidate| (Reverse(candidate.modified), candidate.path.clone()));
}

fn default_roots(filter: FormatFilter) -> Vec<PathBuf> {
    let mut roots = Vec::new();
    if matches!(filter, FormatFilter::Auto | FormatFilter::Vscode) {
        if let Some(home) = home_dir() {
            roots.push(
                home.join("Library")
                    .join("Application Support")
                    .join("Code")
                    .join("User")
                    .join("workspaceStorage"),
            );
        }
    }
    if matches!(filter, FormatFilter::Auto | FormatFilter::Cli) {
        if let Some(home) = home_dir() {
            roots.push(home.join(".copilot").join("session-state"));
        }
    }
    roots
}

fn home_dir() -> Option<PathBuf> {
    std::env::var_os("HOME").map(PathBuf::from)
}
