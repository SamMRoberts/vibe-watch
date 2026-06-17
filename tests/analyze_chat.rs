//! End-to-end analytics test over a fixture chat log.

use vibe_watch::analytics::{CreditSource, RatesSource, SessionAnalytics};
use vibe_watch::chat_log;

use std::process::Command;

fn load_fixture() -> SessionAnalytics {
    let path = concat!(env!("CARGO_MANIFEST_DIR"), "/tests/fixtures/chat_min.jsonl");
    let data = std::fs::read_to_string(path).expect("read fixture");
    let session = chat_log::parse_str(&data).expect("parse fixture");
    SessionAnalytics::from_chat(&session)
}

fn fixture_path() -> String {
    format!(
        "{}/tests/fixtures/chat_min.jsonl",
        env!("CARGO_MANIFEST_DIR")
    )
}

#[test]
fn totals_and_credits_match() {
    let analytics = load_fixture();
    assert_eq!(analytics.session_id.as_deref(), Some("test-session"));
    assert_eq!(analytics.repository_path, None);
    assert_eq!(analytics.model_id.as_deref(), Some("gpt-5.5"));
    assert_eq!(analytics.rates_source, RatesSource::Embedded);
    assert_eq!(analytics.turn_count, 2);
    assert_eq!(analytics.total_output_tokens, 4000);
    assert_eq!(analytics.total_input_tokens, Some(5000));
    assert_eq!(analytics.total_cached_tokens, None);
    assert_eq!(analytics.total_cache_read_tokens, None);
    assert_eq!(analytics.total_cache_write_tokens, None);
    assert_eq!(analytics.total_reasoning_tokens, None);
    assert_eq!(analytics.credit_source, CreditSource::Reported);
    let total = analytics.total_credits.expect("reported credits");
    assert!((total - 12.0).abs() < 1e-9, "got {total}");
    let estimate = analytics.total_estimated_credits.expect("output estimate");
    assert!((estimate - 12.0).abs() < 1e-9, "got {estimate}");
    // 4000 output tokens at 3000 AIC / 1M == 12 AIC.
    assert!((analytics.total_output_credits - 12.0).abs() < 1e-9);
    assert!(!analytics.credits_partial);
}

#[test]
fn falls_back_to_output_only_when_reported_details_are_absent() {
    let data = r#"{"kind":0,"v":{"version":3,"creationDate":1000,"sessionId":"output-only","requests":[],"inputState":{"selectedModel":{"metadata":{"id":"gpt-5.5","name":"GPT-5.5","inputCost":500,"outputCost":3000,"cacheCost":50}}}}}
{"kind":2,"k":["requests"],"v":[{"requestId":"request_r0","timestamp":1000}]}
{"kind":1,"k":["requests",0,"completionTokens"],"v":4000}
"#;
    let session = chat_log::parse_str(data).expect("parse chat");
    let analytics = SessionAnalytics::from_chat(&session);

    assert_eq!(analytics.total_credits, None);
    assert_eq!(analytics.total_estimated_credits, None);
    assert_eq!(analytics.credit_source, CreditSource::OutputOnly);
    assert!((analytics.total_output_credits - 12.0).abs() < 1e-9);
    assert!(analytics.credits_partial);
}

#[test]
fn direct_prompt_tokens_are_also_counted_as_input_tokens() {
    let data = r#"{"kind":0,"v":{"version":3,"creationDate":1000,"sessionId":"direct-prompt","requests":[],"inputState":{"selectedModel":{"metadata":{"id":"gpt-5.5","name":"GPT-5.5","inputCost":500,"outputCost":3000,"cacheCost":50}}}}}
{"kind":2,"k":["requests"],"v":[{"requestId":"request_r0","timestamp":1000}]}
{"kind":1,"k":["requests",0,"promptTokens"],"v":1234}
{"kind":1,"k":["requests",0,"completionTokens"],"v":4000}
"#;
    let session = chat_log::parse_str(data).expect("parse chat");
    let analytics = SessionAnalytics::from_chat(&session);

    assert_eq!(analytics.total_input_tokens, Some(1234));
    assert_eq!(analytics.turns[0].input_tokens, Some(1234));
}

#[test]
fn per_turn_metrics_match() {
    let analytics = load_fixture();

    let t0 = &analytics.turns[0];
    assert_eq!(t0.input_tokens, Some(2000));
    assert_eq!(t0.output_tokens, 1000);
    assert!((t0.pct_output_tokens - 25.0).abs() < 1e-9);
    assert!((t0.pct_time - 25.0).abs() < 1e-9);
    assert!(t0.had_reasoning);
    assert_eq!(t0.tools, vec!["copilot_applyPatch"]);

    let t1 = &analytics.turns[1];
    assert_eq!(t1.input_tokens, Some(3000));
    assert_eq!(t1.output_tokens, 3000);
    assert!((t1.pct_output_tokens - 75.0).abs() < 1e-9);
    assert_eq!(t1.terminal_commands, vec!["cargo test"]);
    assert_eq!(t1.skills, vec!["demo-skill"]);
}

#[test]
fn aggregates_and_wall_clock_match() {
    let analytics = load_fixture();
    // Wall clock: max end (4000 + 6000) - min start (1000) == 9000.
    assert_eq!(analytics.wall_clock_ms, Some(9000));

    let tool_names: Vec<&str> = analytics
        .tool_usage
        .iter()
        .map(|a| a.name.as_str())
        .collect();
    assert!(tool_names.contains(&"copilot_applyPatch"));
    assert!(tool_names.contains(&"run_in_terminal"));
    assert!(tool_names.contains(&"copilot_readFile"));

    assert_eq!(analytics.skill_usage.len(), 1);
    assert_eq!(analytics.skill_usage[0].name, "demo-skill");
    assert_eq!(analytics.skill_usage[0].count, 1);
}

#[test]
fn activity_usage_breaks_down_associated_request_usage() {
    let analytics = load_fixture();

    let apply_patch = analytics
        .tool_activity_usage
        .iter()
        .find(|usage| usage.name == "copilot_applyPatch")
        .expect("applyPatch usage");
    assert_eq!(apply_patch.calls, 1);
    assert_eq!(apply_patch.request_count, 1);
    assert_eq!(apply_patch.input_tokens, Some(2000));
    assert_eq!(apply_patch.output_tokens, 1000);
    assert_eq!(apply_patch.cached_tokens, None);
    assert!((apply_patch.output_credits - 3.0).abs() < 1e-9);
    assert_eq!(apply_patch.credits, Some(4.5));
    assert_eq!(apply_patch.credit_source, CreditSource::Reported);

    let skill = analytics
        .skill_activity_usage
        .iter()
        .find(|usage| usage.name == "demo-skill")
        .expect("skill usage");
    assert_eq!(skill.calls, 1);
    assert_eq!(skill.request_count, 1);
    assert_eq!(skill.input_tokens, Some(3000));
    assert_eq!(skill.output_tokens, 3000);
    assert!((skill.output_credits - 9.0).abs() < 1e-9);
    assert_eq!(skill.credits, Some(7.5));
    assert_eq!(skill.credit_source, CreditSource::Reported);
}

#[test]
fn repeated_activity_calls_share_one_associated_request_usage() {
    let data = r#"{"kind":0,"v":{"version":3,"creationDate":1000,"sessionId":"repeat-tools","requests":[],"inputState":{"selectedModel":{"metadata":{"id":"gpt-5.5","name":"GPT-5.5","inputCost":500,"outputCost":3000,"cacheCost":50}}}}}
{"kind":2,"k":["requests"],"v":[{"requestId":"request_r0","timestamp":1000}]}
{"kind":1,"k":["requests",0,"completionTokens"],"v":1000}
{"kind":1,"k":["requests",0,"result"],"v":{"details":"GPT-5.5 • 2.5 credits","metadata":{"promptTokens":2000}}}
{"kind":2,"k":["requests",0,"response"],"v":[{"kind":"toolInvocationSerialized","toolId":"repeat_tool"},{"kind":"toolInvocationSerialized","toolId":"repeat_tool"}]}
"#;
    let session = chat_log::parse_str(data).expect("parse chat");
    let analytics = SessionAnalytics::from_chat(&session);

    let usage = analytics
        .tool_activity_usage
        .iter()
        .find(|usage| usage.name == "repeat_tool")
        .expect("repeat tool usage");
    assert_eq!(usage.calls, 2);
    assert_eq!(usage.request_count, 1);
    assert_eq!(usage.input_tokens, Some(2000));
    assert_eq!(usage.output_tokens, 1000);
    assert_eq!(usage.credits, Some(2.5));
}

#[test]
fn text_output_includes_activity_usage_sections() {
    let path = fixture_path();
    let output = Command::new(env!("CARGO_BIN_EXE_vibe-watch"))
        .args(["analyze", &path, "--format", "vscode"])
        .output()
        .expect("run vibe-watch");

    assert!(
        output.status.success(),
        "stderr:\n{}",
        String::from_utf8_lossy(&output.stderr)
    );

    let stdout = String::from_utf8_lossy(&output.stdout);
    assert!(
        stdout.contains("Tool request usage:"),
        "missing tool request usage:\n{stdout}"
    );
    assert!(
        stdout.contains("Skill request usage:"),
        "missing skill request usage:\n{stdout}"
    );
    assert!(
        stdout.contains("copilot_applyPatch"),
        "missing tool name:\n{stdout}"
    );
    assert!(
        stdout.contains("4.50 rpt"),
        "missing reported credits:\n{stdout}"
    );
}
