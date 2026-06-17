//! End-to-end analytics test over a fixture chat log.

use vibe_watch::analytics::{CreditSource, RatesSource, SessionAnalytics};
use vibe_watch::chat_log;

fn load_fixture() -> SessionAnalytics {
    let path = concat!(env!("CARGO_MANIFEST_DIR"), "/tests/fixtures/chat_min.jsonl");
    let data = std::fs::read_to_string(path).expect("read fixture");
    let session = chat_log::parse_str(&data).expect("parse fixture");
    SessionAnalytics::from_chat(&session)
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
    assert_eq!(analytics.total_input_tokens, None);
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
fn per_turn_metrics_match() {
    let analytics = load_fixture();

    let t0 = &analytics.turns[0];
    assert_eq!(t0.output_tokens, 1000);
    assert!((t0.pct_output_tokens - 25.0).abs() < 1e-9);
    assert!((t0.pct_time - 25.0).abs() < 1e-9);
    assert!(t0.had_reasoning);
    assert_eq!(t0.tools, vec!["copilot_applyPatch"]);

    let t1 = &analytics.turns[1];
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
