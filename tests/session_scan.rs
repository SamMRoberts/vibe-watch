use std::fs;
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

use vibe_watch::session_scan::{
    discover_sessions, load_analytics, scan_candidates_with, scan_sessions, FormatFilter,
    ScanEvent, SessionCandidate,
};

fn fixture_path(relative: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("tests")
        .join("fixtures")
        .join(relative)
}

fn unique_temp_dir(name: &str) -> PathBuf {
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("time")
        .as_nanos();
    let dir = std::env::temp_dir().join(format!("vibe-watch-{name}-{nanos}"));
    fs::create_dir_all(&dir).expect("create temp dir");
    dir
}

fn copy_fixture(relative: &str, target: &Path) {
    if let Some(parent) = target.parent() {
        fs::create_dir_all(parent).expect("create parent");
    }
    fs::copy(fixture_path(relative), target).expect("copy fixture");
}

#[test]
fn load_analytics_reuses_repository_sidecars() {
    let vscode = load_analytics(
        &fixture_path("vscode_session/chatSessions/chat_min.jsonl"),
        FormatFilter::Vscode,
    )
    .expect("load vscode");
    assert_eq!(
        vscode.repository_path.as_deref(),
        Some("/tmp/vibe-watch-vscode")
    );

    let cli = load_analytics(&fixture_path("cli_session/events.jsonl"), FormatFilter::Cli)
        .expect("load cli");
    assert_eq!(
        cli.repository_path.as_deref(),
        Some("/tmp/vibe-watch-cli-root")
    );
}

#[test]
fn load_analytics_falls_back_to_cli_cwd_when_sidecar_is_missing() {
    let cli = load_analytics(&fixture_path("cli_min.jsonl"), FormatFilter::Cli).expect("load cli");

    assert_eq!(cli.repository_path.as_deref(), Some("/repo"));
}

#[test]
fn discover_sessions_finds_supported_logs_under_directory() {
    let dir = unique_temp_dir("discover");
    copy_fixture(
        "vscode_session/chatSessions/chat_min.jsonl",
        &dir.join("workspaceStorage/abc/chatSessions/chat_min.jsonl"),
    );
    copy_fixture(
        "cli_session/events.jsonl",
        &dir.join("cli-session/events.jsonl"),
    );
    fs::write(dir.join("notes.txt"), "not a session").expect("write noise");

    let candidates = discover_sessions(Some(&dir), FormatFilter::Auto).expect("discover");
    let paths: Vec<String> = candidates
        .iter()
        .map(|candidate| candidate.path.display().to_string())
        .collect();

    assert_eq!(candidates.len(), 2, "paths: {paths:?}");
    assert!(paths.iter().any(|path| path.ends_with("chat_min.jsonl")));
    assert!(paths.iter().any(|path| path.ends_with("events.jsonl")));

    fs::remove_dir_all(&dir).ok();
}

#[test]
fn discover_sessions_sorts_newest_first_with_path_tiebreak() {
    let dir = unique_temp_dir("ordering");
    copy_fixture("cli_session/events.jsonl", &dir.join("b/events.jsonl"));
    copy_fixture("cli_session/events.jsonl", &dir.join("a/events.jsonl"));

    let candidates = discover_sessions(Some(&dir), FormatFilter::Cli).expect("discover");
    let file_names: Vec<String> = candidates
        .iter()
        .map(|candidate| candidate.path.display().to_string())
        .collect();

    assert_eq!(candidates.len(), 2, "paths: {file_names:?}");
    assert!(
        file_names[0].contains("/a/events.jsonl") || file_names[0].contains("\\a\\events.jsonl"),
        "expected path tie-break to sort a before b: {file_names:?}"
    );

    fs::remove_dir_all(&dir).ok();
}

#[test]
fn scan_sessions_reports_loaded_and_error_rows() {
    let dir = unique_temp_dir("scan");
    copy_fixture("cli_session/events.jsonl", &dir.join("good/events.jsonl"));
    fs::create_dir_all(dir.join("bad")).expect("bad dir");
    fs::write(dir.join("bad/events.jsonl"), "not jsonl").expect("bad session");

    let events = scan_sessions(Some(&dir), FormatFilter::Auto).expect("scan");
    let mut loaded = 0;
    let mut errors = 0;
    let mut progress = None;
    let mut finished = false;

    for event in events {
        match event {
            ScanEvent::Progress(next) => progress = Some(next),
            ScanEvent::Loaded(session) => {
                loaded += 1;
                assert_eq!(session.analytics.session_id.as_deref(), Some("cli-session"));
            }
            ScanEvent::Error(error) => {
                errors += 1;
                assert!(error.message.contains("unrecognized log format"));
            }
            ScanEvent::Finished => finished = true,
        }
    }

    assert_eq!(loaded, 1);
    assert_eq!(errors, 1);
    assert_eq!(progress.expect("progress").processed, 2);
    assert!(finished);

    fs::remove_dir_all(&dir).ok();
}

#[test]
fn scan_candidates_streams_loaded_events_before_later_errors_finish() {
    let dir = unique_temp_dir("stream");
    let good = dir.join("good/events.jsonl");
    let bad = dir.join("bad/events.jsonl");
    copy_fixture("cli_session/events.jsonl", &good);
    fs::create_dir_all(bad.parent().expect("bad parent")).expect("bad parent dir");
    fs::write(&bad, "not jsonl").expect("bad session");

    let candidates = vec![
        SessionCandidate {
            path: good,
            modified: UNIX_EPOCH,
        },
        SessionCandidate {
            path: bad,
            modified: UNIX_EPOCH,
        },
    ];
    let mut seen_loaded_before_error = false;
    let mut saw_error = false;

    scan_candidates_with(candidates, FormatFilter::Auto, |event| {
        match event {
            ScanEvent::Loaded(_) => {
                seen_loaded_before_error = !saw_error;
            }
            ScanEvent::Error(_) => saw_error = true,
            _ => {}
        }
        true
    });

    assert!(
        seen_loaded_before_error,
        "loaded sessions should be emitted before later candidates finish"
    );

    fs::remove_dir_all(&dir).ok();
}
