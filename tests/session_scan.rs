use std::fs;
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

use vibe_watch::cache::CacheConfig;
use vibe_watch::session_scan::{
    discover_sessions, load_analytics, load_analytics_with_cache, scan_candidates_with,
    scan_candidates_with_cache, scan_sessions, FormatFilter, LoadOrigin, ScanEvent,
    SessionCandidate,
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

fn cache_config(path: &Path) -> CacheConfig {
    CacheConfig::enabled_at(path.to_path_buf())
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

#[test]
fn unchanged_session_loads_from_cache_after_first_parse() {
    let dir = unique_temp_dir("cache-hit");
    let session = dir.join("events.jsonl");
    let cache_db = dir.join("cache.sqlite");
    copy_fixture("cli_session/events.jsonl", &session);

    let first = load_analytics_with_cache(&session, FormatFilter::Cli, &cache_config(&cache_db))
        .expect("first load");
    let second = load_analytics_with_cache(&session, FormatFilter::Cli, &cache_config(&cache_db))
        .expect("second load");

    assert_eq!(first.origin, LoadOrigin::Parsed);
    assert_eq!(second.origin, LoadOrigin::Cached);
    assert_eq!(first.analytics.session_id, second.analytics.session_id);
    assert_eq!(
        first.analytics.repository_path,
        second.analytics.repository_path
    );
    assert_eq!(
        first.analytics.total_output_tokens,
        second.analytics.total_output_tokens
    );
    assert_eq!(
        first.analytics.total_credits,
        second.analytics.total_credits
    );

    fs::remove_dir_all(&dir).ok();
}

#[test]
fn source_file_change_reparses_and_updates_cache() {
    let dir = unique_temp_dir("cache-source-change");
    let session = dir.join("events.jsonl");
    let cache_db = dir.join("cache.sqlite");
    copy_fixture("cli_session/events.jsonl", &session);

    let first = load_analytics_with_cache(&session, FormatFilter::Cli, &cache_config(&cache_db))
        .expect("first load");
    fs::write(
        &session,
        r#"{"type":"session.start","data":{"sessionId":"changed","context":{"cwd":"/changed"}},"timestamp":"2026-01-01T00:00:00.000Z"}
{"type":"session.model_change","data":{"newModel":"gpt-5.5"},"timestamp":"2026-01-01T00:00:01.000Z"}
{"type":"user.message","data":{"agentMode":"interactive","content":"Do it"},"timestamp":"2026-01-01T00:00:02.000Z"}
{"type":"assistant.message","data":{"outputTokens":42,"toolRequests":[]},"timestamp":"2026-01-01T00:00:03.000Z"}
"#,
    )
    .expect("rewrite session");

    let second = load_analytics_with_cache(&session, FormatFilter::Cli, &cache_config(&cache_db))
        .expect("second load");
    let third = load_analytics_with_cache(&session, FormatFilter::Cli, &cache_config(&cache_db))
        .expect("third load");

    assert_eq!(first.origin, LoadOrigin::Parsed);
    assert_eq!(second.origin, LoadOrigin::Parsed);
    assert_eq!(second.analytics.session_id.as_deref(), Some("changed"));
    assert_eq!(second.analytics.total_output_tokens, 42);
    assert_eq!(third.origin, LoadOrigin::Cached);
    assert_eq!(third.analytics.session_id, second.analytics.session_id);

    fs::remove_dir_all(&dir).ok();
}

#[test]
fn vscode_workspace_sidecar_change_invalidates_cached_repository() {
    let dir = unique_temp_dir("cache-vscode-sidecar");
    let chat_sessions = dir.join("storage/chatSessions");
    let session = chat_sessions.join("chat_min.jsonl");
    let cache_db = dir.join("cache.sqlite");
    copy_fixture("chat_min.jsonl", &session);
    fs::write(
        dir.join("storage/workspace.json"),
        r#"{"folder":"/tmp/one"}"#,
    )
    .expect("workspace one");

    let first = load_analytics_with_cache(&session, FormatFilter::Vscode, &cache_config(&cache_db))
        .expect("first load");
    fs::write(
        dir.join("storage/workspace.json"),
        r#"{"folder":"/tmp/two"}"#,
    )
    .expect("workspace two");
    let second =
        load_analytics_with_cache(&session, FormatFilter::Vscode, &cache_config(&cache_db))
            .expect("second load");

    assert_eq!(first.origin, LoadOrigin::Parsed);
    assert_eq!(first.analytics.repository_path.as_deref(), Some("/tmp/one"));
    assert_eq!(second.origin, LoadOrigin::Parsed);
    assert_eq!(
        second.analytics.repository_path.as_deref(),
        Some("/tmp/two")
    );

    fs::remove_dir_all(&dir).ok();
}

#[test]
fn cli_workspace_sidecar_change_invalidates_cached_repository() {
    let dir = unique_temp_dir("cache-cli-sidecar");
    let session = dir.join("events.jsonl");
    let cache_db = dir.join("cache.sqlite");
    copy_fixture("cli_session/events.jsonl", &session);
    fs::write(dir.join("workspace.yaml"), "git_root: /tmp/one\n").expect("workspace one");

    let first = load_analytics_with_cache(&session, FormatFilter::Cli, &cache_config(&cache_db))
        .expect("first load");
    fs::write(dir.join("workspace.yaml"), "git_root: /tmp/two\n").expect("workspace two");
    let second = load_analytics_with_cache(&session, FormatFilter::Cli, &cache_config(&cache_db))
        .expect("second load");

    assert_eq!(first.origin, LoadOrigin::Parsed);
    assert_eq!(first.analytics.repository_path.as_deref(), Some("/tmp/one"));
    assert_eq!(second.origin, LoadOrigin::Parsed);
    assert_eq!(
        second.analytics.repository_path.as_deref(),
        Some("/tmp/two")
    );

    fs::remove_dir_all(&dir).ok();
}

#[test]
fn corrupt_cached_analytics_falls_back_to_parse() {
    let dir = unique_temp_dir("cache-corrupt");
    let session = dir.join("events.jsonl");
    let cache_db = dir.join("cache.sqlite");
    copy_fixture("cli_session/events.jsonl", &session);

    let first = load_analytics_with_cache(&session, FormatFilter::Cli, &cache_config(&cache_db))
        .expect("first load");
    assert_eq!(first.origin, LoadOrigin::Parsed);
    let connection = rusqlite::Connection::open(&cache_db).expect("open cache");
    connection
        .execute("UPDATE sessions SET analytics_json = 'not json'", [])
        .expect("corrupt cache");

    let second = load_analytics_with_cache(&session, FormatFilter::Cli, &cache_config(&cache_db))
        .expect("second load");
    assert_eq!(second.origin, LoadOrigin::Parsed);
    assert_eq!(second.analytics.session_id.as_deref(), Some("cli-session"));

    fs::remove_dir_all(&dir).ok();
}

#[test]
fn cache_aware_scan_preserves_loaded_error_and_progress_events() {
    let dir = unique_temp_dir("cache-scan");
    let good = dir.join("good/events.jsonl");
    let bad = dir.join("bad/events.jsonl");
    let cache_db = dir.join("cache.sqlite");
    copy_fixture("cli_session/events.jsonl", &good);
    fs::create_dir_all(bad.parent().expect("bad parent")).expect("bad parent dir");
    fs::write(&bad, "not jsonl").expect("bad session");

    let candidates = vec![
        SessionCandidate {
            path: good.clone(),
            modified: UNIX_EPOCH,
        },
        SessionCandidate {
            path: bad,
            modified: UNIX_EPOCH,
        },
    ];
    let mut first_events = Vec::new();
    scan_candidates_with_cache(
        candidates.clone(),
        FormatFilter::Auto,
        &cache_config(&cache_db),
        |event| {
            first_events.push(event);
            true
        },
    );
    let mut second_events = Vec::new();
    scan_candidates_with_cache(
        candidates,
        FormatFilter::Auto,
        &cache_config(&cache_db),
        |event| {
            second_events.push(event);
            true
        },
    );

    assert_scan_shape(&first_events);
    assert_scan_shape(&second_events);

    fs::remove_dir_all(&dir).ok();
}

fn assert_scan_shape(events: &[ScanEvent]) {
    let mut loaded = 0;
    let mut errors = 0;
    let mut progress = None;
    let mut finished = false;

    for event in events {
        match event {
            ScanEvent::Progress(next) => progress = Some(*next),
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
}
