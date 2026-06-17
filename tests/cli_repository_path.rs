use std::process::Command;

use serde_json::Value;

fn fixture_path(relative: &str) -> String {
    format!("{}/tests/fixtures/{relative}", env!("CARGO_MANIFEST_DIR"))
}

fn run_analyze(args: &[&str]) -> Value {
    let output = Command::new(env!("CARGO_BIN_EXE_vibe-watch"))
        .args(args)
        .output()
        .expect("run vibe-watch");

    assert!(
        output.status.success(),
        "stderr:\n{}",
        String::from_utf8_lossy(&output.stderr)
    );

    serde_json::from_slice(&output.stdout).expect("parse json output")
}

#[test]
fn analyze_vscode_session_reports_repository_path() {
    let log_path = fixture_path("vscode_session/chatSessions/chat_min.jsonl");
    let json = run_analyze(&["analyze", &log_path, "--json", "--format", "vscode"]);

    assert_eq!(
        json.get("repository_path").and_then(Value::as_str),
        Some("/tmp/vibe-watch-vscode")
    );
}

#[test]
fn analyze_cli_session_prefers_git_root() {
    let log_path = fixture_path("cli_session/events.jsonl");
    let json = run_analyze(&["analyze", &log_path, "--json", "--format", "cli"]);

    assert_eq!(
        json.get("repository_path").and_then(Value::as_str),
        Some("/tmp/vibe-watch-cli-root")
    );
}

#[test]
fn analyze_cli_session_falls_back_to_cwd() {
    let log_path = fixture_path("cli_session_cwd/events.jsonl");
    let json = run_analyze(&["analyze", &log_path, "--json", "--format", "cli"]);

    assert_eq!(
        json.get("repository_path").and_then(Value::as_str),
        Some("/tmp/vibe-watch-cli-cwd")
    );
}
