//! Session discovery and loading for the multi-session TUI browser.

use std::cmp::Reverse;
use std::fs::File;
use std::io::{BufRead, BufReader};
use std::path::{Path, PathBuf};
use std::time::SystemTime;

use anyhow::{bail, Context, Result};

use crate::analytics::SessionAnalytics;
use crate::cache::{
    dependency_metadata, source_metadata, CacheConfig, DependencyMetadata, SessionCache,
};
use crate::workspace::{
    resolve_repository_context, resolve_repository_path, WorkspaceContext, WorkspaceFormat,
};
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

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LoadOrigin {
    Parsed,
    Cached,
}

#[derive(Debug, Clone)]
pub struct LoadedAnalytics {
    pub analytics: SessionAnalytics,
    pub origin: LoadOrigin,
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

pub fn load_analytics_with_cache(
    path: &Path,
    filter: FormatFilter,
    config: &CacheConfig,
) -> Result<LoadedAnalytics> {
    let mut cache = SessionCache::open(config)?;
    load_analytics_with_cache_handle(path, filter, config, cache.as_mut())
}

fn load_analytics_with_cache_handle(
    path: &Path,
    filter: FormatFilter,
    config: &CacheConfig,
    cache: Option<&mut SessionCache>,
) -> Result<LoadedAnalytics> {
    let initial_metadata = source_metadata(path)?;
    let format = detect_format_from_path(path, filter)?;
    let workspace_context = resolve_repository_context(path, workspace_format(format))?;
    let dependencies = dependency_metadata_for_context(&workspace_context)?;

    if let Some(cache) = cache {
        if !config.refresh {
            if let Some(analytics) = cache.lookup(&initial_metadata, format, &dependencies)? {
                return Ok(LoadedAnalytics {
                    analytics,
                    origin: LoadOrigin::Cached,
                });
            }
        }
        let data =
            std::fs::read_to_string(path).with_context(|| format!("reading {}", path.display()))?;
        let analytics = analytics_from_data(&data, format, workspace_context.repository_path)?;
        let final_metadata = source_metadata(path)?;
        if initial_metadata == final_metadata {
            cache.store(&final_metadata, format, &dependencies, &analytics)?;
        }
        return Ok(LoadedAnalytics {
            analytics,
            origin: LoadOrigin::Parsed,
        });
    }

    let data =
        std::fs::read_to_string(path).with_context(|| format!("reading {}", path.display()))?;
    Ok(LoadedAnalytics {
        analytics: analytics_from_data(&data, format, workspace_context.repository_path)?,
        origin: LoadOrigin::Parsed,
    })
}

fn analytics_from_data(
    data: &str,
    format: DetectedFormat,
    repository_path: Option<String>,
) -> Result<SessionAnalytics> {
    let repository_path = normalize_repository_path(repository_path);
    match format {
        DetectedFormat::Vscode => {
            let mut session = chat_log::parse_str(data)?;
            session.repository_path = repository_path;
            Ok(SessionAnalytics::from_chat(&session))
        }
        DetectedFormat::Cli => {
            let mut session = cli_log::parse_str(data)?;
            session.repository_path =
                repository_path.or_else(|| normalize_repository_path(session.cwd.clone()));
            Ok(SessionAnalytics::from_cli(&session))
        }
    }
}

fn dependency_metadata_for_context(context: &WorkspaceContext) -> Result<Vec<DependencyMetadata>> {
    context
        .dependencies
        .iter()
        .map(|dependency| dependency_metadata(dependency.kind.clone(), &dependency.path))
        .collect()
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
    let mut events = Vec::new();
    scan_candidates_with(candidates, filter, |event| {
        events.push(event);
        true
    });
    Ok(events)
}

pub fn scan_candidates(candidates: Vec<SessionCandidate>, filter: FormatFilter) -> Vec<ScanEvent> {
    let mut events = Vec::new();
    scan_candidates_with(candidates, filter, |event| {
        events.push(event);
        true
    });
    events
}

pub fn scan_candidates_with(
    candidates: Vec<SessionCandidate>,
    filter: FormatFilter,
    mut emit: impl FnMut(ScanEvent) -> bool,
) {
    let total = candidates.len();
    if !emit(ScanEvent::Progress(ScanProgress {
        processed: 0,
        total,
        finished: total == 0,
    })) {
        return;
    }

    for (index, candidate) in candidates.into_iter().enumerate() {
        let keep_going = match load_analytics(&candidate.path, filter) {
            Ok(analytics) => emit(ScanEvent::Loaded(Box::new(LoadedSession {
                path: candidate.path.clone(),
                modified: candidate.modified,
                analytics,
            }))),
            Err(error) => emit(ScanEvent::Error(SessionLoadError {
                path: candidate.path.clone(),
                modified: Some(candidate.modified),
                message: error.to_string(),
            })),
        };
        if !keep_going {
            return;
        }
        if !emit(ScanEvent::Progress(ScanProgress {
            processed: index + 1,
            total,
            finished: index + 1 == total,
        })) {
            return;
        }
    }

    let _ = emit(ScanEvent::Finished);
}

pub fn scan_candidates_with_cache(
    candidates: Vec<SessionCandidate>,
    filter: FormatFilter,
    config: &CacheConfig,
    mut emit: impl FnMut(ScanEvent) -> bool,
) {
    let cache = match SessionCache::open(config) {
        Ok(None) => {
            scan_candidates_with(candidates, filter, emit);
            return;
        }
        Ok(cache) => cache,
        Err(error) => {
            let _ = emit(ScanEvent::Progress(ScanProgress {
                processed: 0,
                total: candidates.len(),
                finished: candidates.is_empty(),
            }));
            let _ = emit(ScanEvent::Error(SessionLoadError {
                path: PathBuf::from("(cache)"),
                modified: None,
                message: error.to_string(),
            }));
            let _ = emit(ScanEvent::Finished);
            return;
        }
    };
    let mut cache = cache.expect("cache open returned Some when enabled");
    scan_candidates_with_cache_handle(candidates, filter, config, Some(&mut cache), emit);
}

pub fn scan_candidates_with_cache_handle(
    candidates: Vec<SessionCandidate>,
    filter: FormatFilter,
    config: &CacheConfig,
    mut cache: Option<&mut SessionCache>,
    mut emit: impl FnMut(ScanEvent) -> bool,
) {
    let total = candidates.len();
    if !emit(ScanEvent::Progress(ScanProgress {
        processed: 0,
        total,
        finished: total == 0,
    })) {
        return;
    }

    for (index, candidate) in candidates.into_iter().enumerate() {
        let keep_going = match load_analytics_with_cache_handle(
            &candidate.path,
            filter,
            config,
            cache.as_deref_mut(),
        ) {
            Ok(loaded) => emit(ScanEvent::Loaded(Box::new(LoadedSession {
                path: candidate.path.clone(),
                modified: candidate.modified,
                analytics: loaded.analytics,
            }))),
            Err(error) => emit(ScanEvent::Error(SessionLoadError {
                path: candidate.path.clone(),
                modified: Some(candidate.modified),
                message: error.to_string(),
            })),
        };
        if !keep_going {
            return;
        }
        if !emit(ScanEvent::Progress(ScanProgress {
            processed: index + 1,
            total,
            finished: index + 1 == total,
        })) {
            return;
        }
    }

    let _ = emit(ScanEvent::Finished);
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

fn detect_format_from_path(path: &Path, filter: FormatFilter) -> Result<DetectedFormat> {
    match filter {
        FormatFilter::Vscode => Ok(DetectedFormat::Vscode),
        FormatFilter::Cli => Ok(DetectedFormat::Cli),
        FormatFilter::Auto => {
            let file = File::open(path).with_context(|| format!("reading {}", path.display()))?;
            let first_line = BufReader::new(file)
                .lines()
                .map_while(Result::ok)
                .find(|line| !line.trim().is_empty())
                .unwrap_or_default();
            detect_format(&first_line, filter)
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
