//! End-to-end analytics test over a fixture Copilot CLI events log.

use vibe_watch::analytics::{RatesSource, SessionAnalytics};
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
    assert_eq!(analytics.model_id.as_deref(), Some("gpt-5.5"));
    assert_eq!(analytics.rates_source, RatesSource::Builtin);
    // session.shutdown provides full token breakdown, so credits are not partial.
    assert!(!analytics.credits_partial);
    assert_eq!(analytics.total_input_tokens, Some(10000));
    assert_eq!(analytics.total_cached_tokens, Some(5000));
    // Full credits: input 10000*500/1M + output 800*3000/1M + cache 5000*50/1M
    //             = 5.0 + 2.4 + 0.25 = 7.65 AIC.
    let total = analytics.total_credits.expect("total credits");
    assert!((total - 7.65).abs() < 1e-9, "got {total}");
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
    assert_eq!(model.reasoning_tokens, 100);
    assert_eq!(model.reported_cost, Some(3.0));
    assert!((model.credits.expect("credits") - 7.65).abs() < 1e-9);
    // Wall clock: min start 2s, max end 13s == 11000 ms.
    assert_eq!(analytics.wall_clock_ms, Some(11000));
}
