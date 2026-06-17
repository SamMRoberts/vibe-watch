//! End-to-end analytics test over a fixture Copilot CLI events log.

use vibe_watch::analytics::{CreditSource, RatesSource, SessionAnalytics};
use vibe_watch::cli_log;

fn load_fixture() -> SessionAnalytics {
    let path = concat!(env!("CARGO_MANIFEST_DIR"), "/tests/fixtures/cli_min.jsonl");
    let data = std::fs::read_to_string(path).expect("read fixture");
    let session = cli_log::parse_str(&data).expect("parse fixture");
    SessionAnalytics::from_cli(&session)
}

#[test]
fn full_usage_yields_full_credits() {
    let analytics = load_fixture();
    assert_eq!(analytics.session_id.as_deref(), Some("cli-session"));
    assert_eq!(analytics.repository_path, None);
    assert_eq!(analytics.model_id.as_deref(), Some("gpt-5.5"));
    assert_eq!(analytics.rates_source, RatesSource::Builtin);
    // session.shutdown provides full token breakdown, so credits are not partial.
    assert!(!analytics.credits_partial);
    assert_eq!(analytics.total_input_tokens, Some(10000));
    assert_eq!(analytics.total_cached_tokens, Some(7000));
    assert_eq!(analytics.total_cache_read_tokens, Some(5000));
    assert_eq!(analytics.total_cache_write_tokens, Some(2000));
    assert_eq!(analytics.total_reasoning_tokens, Some(100));
    assert_eq!(analytics.credit_source, CreditSource::Reported);
    // Explicit CLI cost is authoritative when present.
    let total = analytics.total_credits.expect("total credits");
    assert!((total - 3.0).abs() < 1e-9, "got {total}");
    // The pricing-based estimate remains available as fallback metadata.
    let estimate = analytics
        .total_estimated_credits
        .expect("estimated credits");
    assert!((estimate - 7.75).abs() < 1e-9, "got {estimate}");
}

#[test]
fn estimates_full_credits_when_reported_cost_is_absent() {
    let data = r#"{"type":"session.start","data":{"sessionId":"cli-estimate","context":{"cwd":"/repo"}},"timestamp":"2026-01-01T00:00:00.000Z"}
{"type":"session.model_change","data":{"newModel":"gpt-5.5"},"timestamp":"2026-01-01T00:00:01.000Z"}
{"type":"user.message","data":{"agentMode":"interactive","content":"Do it"},"timestamp":"2026-01-01T00:00:02.000Z"}
{"type":"assistant.message","data":{"outputTokens":800,"toolRequests":[]},"timestamp":"2026-01-01T00:00:05.000Z"}
{"type":"session.shutdown","data":{"modelMetrics":{"gpt-5.5":{"requests":{"count":1},"usage":{"inputTokens":10000,"outputTokens":800,"cacheReadTokens":5000,"cacheWriteTokens":2000,"reasoningTokens":100}}}},"timestamp":"2026-01-01T00:00:06.000Z"}
"#;
    let session = cli_log::parse_str(data).expect("parse fixture");
    let analytics = SessionAnalytics::from_cli(&session);

    assert_eq!(analytics.credit_source, CreditSource::Estimated);
    assert_eq!(analytics.total_cached_tokens, Some(7000));
    let total = analytics.total_credits.expect("total credits");
    assert!((total - 7.75).abs() < 1e-9, "got {total}");
    assert_eq!(analytics.total_estimated_credits, analytics.total_credits);
}

#[test]
fn per_turn_metrics_and_modes() {
    let analytics = load_fixture();
    assert_eq!(analytics.turn_count, 2);
    assert_eq!(analytics.total_output_tokens, 800);

    let t0 = &analytics.turns[0];
    assert_eq!(t0.output_tokens, 200);
    assert!((t0.pct_output_tokens - 25.0).abs() < 1e-9);
    assert_eq!(t0.mode.as_deref(), Some("plan"));
    assert_eq!(t0.elapsed_ms, Some(3800));
    assert_eq!(t0.tools, vec!["shell"]);
    assert_eq!(t0.skills, vec!["scope-guard"]);
    assert_eq!(t0.terminal_commands, vec!["ls -la"]);

    let t1 = &analytics.turns[1];
    assert_eq!(t1.output_tokens, 600);
    assert!((t1.pct_output_tokens - 75.0).abs() < 1e-9);
    assert_eq!(t1.mode.as_deref(), Some("interactive"));
}

#[test]
fn per_model_breakdown_present() {
    let analytics = load_fixture();
    assert_eq!(analytics.per_model.len(), 1);
    let model = &analytics.per_model[0];
    assert_eq!(model.name, "gpt-5.5");
    assert_eq!(model.requests, 5);
    assert_eq!(model.input_tokens, 10000);
    assert_eq!(model.cache_read_tokens, 5000);
    assert_eq!(model.cache_write_tokens, 2000);
    assert_eq!(model.reasoning_tokens, 100);
    assert_eq!(model.reported_cost, Some(3.0));
    assert!((model.credits.expect("credits") - 3.0).abs() < 1e-9);
    assert!((model.estimated_credits.expect("estimate") - 7.75).abs() < 1e-9);
    // Wall clock: min start 2s, max end 13s == 11000 ms.
    assert_eq!(analytics.wall_clock_ms, Some(11000));
}
