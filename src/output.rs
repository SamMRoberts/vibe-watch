//! Human-readable table and JSON rendering of [`SessionAnalytics`].

use anyhow::Result;

use crate::analytics::{Aggregate, ModelUsage, RatesSource, SessionAnalytics};

/// Print analytics as a compact, aligned text report.
pub fn print_table(analytics: &SessionAnalytics) {
    println!(
        "Session : {}",
        analytics.session_id.as_deref().unwrap_or("(unknown)")
    );
    if let Some(path) = &analytics.repository_path {
        println!("Repo    : {path}");
    }
    if let Some(title) = &analytics.title {
        println!("Title   : {title}");
    }
    println!(
        "Model   : {} ({})",
        analytics.model_name.as_deref().unwrap_or("?"),
        analytics.model_id.as_deref().unwrap_or("?")
    );
    print_rates(analytics);
    println!();

    let credit_note = if analytics.credits_partial {
        "  (output-only; input/cache tokens not recorded in this format)"
    } else {
        ""
    };
    println!(
        "Turns   : {}   Output tokens: {}   Output credits: {:.2} AIC{}",
        analytics.turn_count,
        analytics.total_output_tokens,
        analytics.total_output_credits,
        credit_note
    );
    if let (Some(input), Some(cached)) =
        (analytics.total_input_tokens, analytics.total_cached_tokens)
    {
        println!(
            "Tokens  : input {}   cached {}   output {}",
            input, cached, analytics.total_output_tokens
        );
    }
    if let Some(total) = analytics.total_credits {
        println!("Credits : {total:.2} AIC total (input + output + cache)");
    }
    print!(
        "Time    : model {:.1}s",
        analytics.total_elapsed_ms as f64 / 1000.0
    );
    if let Some(wall) = analytics.wall_clock_ms {
        print!("   wall-clock {:.1}s", wall as f64 / 1000.0);
    }
    println!();
    println!();

    println!(
        "{:>3}  {:<12} {:>10} {:>7} {:>10} {:>7} {:>6}  skills/subagents",
        "#", "request", "out_tok", "%tok", "time", "%time", "tools"
    );
    for turn in &analytics.turns {
        let extras = format_extras(turn);
        println!(
            "{:>3}  {:<12} {:>10} {:>6.1}% {:>9.1}s {:>6.1}% {:>6}  {}",
            turn.index,
            short_id(turn.request_id.as_deref()),
            turn.output_tokens,
            turn.pct_output_tokens,
            turn.elapsed_ms.unwrap_or(0) as f64 / 1000.0,
            turn.pct_time,
            turn.tools.len(),
            extras
        );
    }

    print_models(&analytics.per_model);
    print_aggregate("Tools", &analytics.tool_usage);
    print_aggregate("Skills", &analytics.skill_usage);
    print_aggregate("Subagents", &analytics.subagent_usage);
}

/// Print analytics as pretty JSON.
pub fn print_json(analytics: &SessionAnalytics) -> Result<()> {
    println!("{}", serde_json::to_string_pretty(analytics)?);
    Ok(())
}

fn print_rates(analytics: &SessionAnalytics) {
    match (analytics.rates_source, analytics.rates) {
        (RatesSource::Unknown, _) | (_, None) => {
            println!("Rates   : unknown (credits unavailable)");
        }
        (source, Some(rates)) => {
            let label = match source {
                RatesSource::Embedded => "embedded",
                RatesSource::Builtin => "built-in",
                RatesSource::Unknown => "unknown",
            };
            println!(
                "Rates   : {} (AIC/1M) in {:.0}  out {:.0}  cache {:.0}",
                label, rates.input_per_m, rates.output_per_m, rates.cache_per_m
            );
        }
    }
}

fn format_extras(turn: &crate::analytics::TurnMetrics) -> String {
    let mut parts = Vec::new();
    if let Some(mode) = &turn.mode {
        parts.push(format!("mode: {mode}"));
    }
    if !turn.skills.is_empty() {
        parts.push(format!("skills: {}", turn.skills.join(", ")));
    }
    if !turn.subagents.is_empty() {
        parts.push(format!("subagents: {}", turn.subagents.join(", ")));
    }
    parts.join("  ")
}

fn print_models(models: &[ModelUsage]) {
    if models.is_empty() {
        return;
    }
    println!();
    println!("Per model:");
    for model in models {
        let credits = match model.credits {
            Some(value) => format!("{value:.2} AIC"),
            None => "n/a".to_string(),
        };
        println!(
            "  {:<18} {:>5} req   in {:>10}  out {:>9}  cache {:>10}  reason {:>8}   {}",
            model.name,
            model.requests,
            model.input_tokens,
            model.output_tokens,
            model.cache_read_tokens + model.cache_write_tokens,
            model.reasoning_tokens,
            credits
        );
    }
}

fn print_aggregate(label: &str, items: &[Aggregate]) {
    if items.is_empty() {
        return;
    }
    println!();
    println!("{label}:");
    for item in items {
        println!("  {:>4}  {}", item.count, item.name);
    }
}

/// Shorten a request id for table display.
fn short_id(request_id: Option<&str>) -> String {
    let Some(id) = request_id else {
        return "-".to_string();
    };
    let trimmed = id.strip_prefix("request_").unwrap_or(id);
    trimmed.chars().take(12).collect()
}
