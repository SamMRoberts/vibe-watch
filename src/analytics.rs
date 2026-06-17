//! Typed analytics derived from a parsed session: per-turn tokens, credits,
//! percentages, timeline, and tool/skill/subagent frequencies.

use std::collections::HashMap;

use serde::Serialize;

use crate::chat_log::{ChatModel, ChatSession};
use crate::cli_log::CliSession;
use crate::pricing::{builtin_rates, ModelRates};

/// Where the active credit rates came from.
#[derive(Debug, Clone, Copy, PartialEq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum RatesSource {
    /// Pricing embedded in the session log header.
    Embedded,
    /// Pricing from the built-in table.
    Builtin,
    /// No pricing available; credits cannot be computed.
    Unknown,
}

/// A named frequency count (tools, skills, subagents).
#[derive(Debug, Clone, Serialize)]
pub struct Aggregate {
    pub name: String,
    pub count: usize,
}

/// Per-model token usage and cost reported for a session.
#[derive(Debug, Clone, Serialize)]
pub struct ModelUsage {
    pub name: String,
    pub requests: u64,
    pub input_tokens: u64,
    pub output_tokens: u64,
    pub cache_read_tokens: u64,
    pub cache_write_tokens: u64,
    pub reasoning_tokens: u64,
    /// Cost reported by the agent runtime, in its own units (not normalized).
    pub reported_cost: Option<f64>,
    /// Full AIC credits from the built-in rate table, when the model is known.
    pub credits: Option<f64>,
}

/// Metrics for a single request/response turn.
#[derive(Debug, Clone, Serialize)]
pub struct TurnMetrics {
    pub index: usize,
    pub request_id: Option<String>,
    pub model: Option<String>,
    pub mode: Option<String>,
    pub timestamp_ms: Option<i64>,
    pub output_tokens: u64,
    pub input_tokens: Option<u64>,
    pub cached_tokens: Option<u64>,
    pub elapsed_ms: Option<i64>,
    pub first_progress_ms: Option<i64>,
    pub output_credits: f64,
    /// Full credits (input + output + cache) when all token counts are known.
    pub credits: Option<f64>,
    pub pct_output_tokens: f64,
    pub pct_time: f64,
    pub tools: Vec<String>,
    pub subagents: Vec<String>,
    pub skills: Vec<String>,
    pub terminal_commands: Vec<String>,
    pub had_reasoning: bool,
}

/// Aggregated analytics for an entire session.
#[derive(Debug, Clone, Serialize)]
pub struct SessionAnalytics {
    pub session_id: Option<String>,
    pub repository_path: Option<String>,
    pub title: Option<String>,
    pub model_id: Option<String>,
    pub model_name: Option<String>,
    pub rates: Option<ModelRates>,
    pub rates_source: RatesSource,
    /// True when only output-side tokens/credits are known (chat format).
    pub credits_partial: bool,
    pub turn_count: usize,
    pub total_output_tokens: u64,
    pub total_input_tokens: Option<u64>,
    pub total_cached_tokens: Option<u64>,
    pub total_output_credits: f64,
    pub total_credits: Option<f64>,
    pub total_elapsed_ms: i64,
    pub wall_clock_ms: Option<i64>,
    pub turns: Vec<TurnMetrics>,
    pub per_model: Vec<ModelUsage>,
    pub tool_usage: Vec<Aggregate>,
    pub skill_usage: Vec<Aggregate>,
    pub subagent_usage: Vec<Aggregate>,
}

impl SessionAnalytics {
    /// Build analytics from a parsed VS Code chat session.
    ///
    /// The chat format exposes only output (completion) tokens, so credits are
    /// output-only and [`SessionAnalytics::credits_partial`] is `true`.
    pub fn from_chat(session: &ChatSession) -> Self {
        let (rates, rates_source) = resolve_rates(&session.model);

        let total_output_tokens: u64 = session.requests.iter().map(|r| r.completion_tokens).sum();
        let total_elapsed_ms: i64 = session.requests.iter().filter_map(|r| r.elapsed_ms).sum();

        let mut turns = Vec::with_capacity(session.requests.len());
        let mut tool_counts: HashMap<String, usize> = HashMap::new();
        let mut skill_counts: HashMap<String, usize> = HashMap::new();
        let mut subagent_counts: HashMap<String, usize> = HashMap::new();
        let mut total_output_credits = 0.0;

        for (index, request) in session.requests.iter().enumerate() {
            let output_credits = rates
                .map(|r| r.output_credits(request.completion_tokens))
                .unwrap_or(0.0);
            total_output_credits += output_credits;

            for tool in &request.tools {
                *tool_counts.entry(tool.clone()).or_default() += 1;
            }
            for skill in &request.skills {
                *skill_counts.entry(skill.clone()).or_default() += 1;
            }
            for subagent in &request.subagents {
                *subagent_counts.entry(subagent.clone()).or_default() += 1;
            }

            turns.push(TurnMetrics {
                index,
                request_id: request.request_id.clone(),
                model: session.model.id.clone(),
                mode: None,
                timestamp_ms: request.timestamp_ms,
                output_tokens: request.completion_tokens,
                input_tokens: None,
                cached_tokens: None,
                elapsed_ms: request.elapsed_ms,
                first_progress_ms: request.first_progress_ms,
                output_credits,
                credits: None,
                pct_output_tokens: percent(request.completion_tokens, total_output_tokens),
                pct_time: percent_i64(request.elapsed_ms.unwrap_or(0), total_elapsed_ms),
                tools: request.tools.clone(),
                subagents: request.subagents.clone(),
                skills: request.skills.clone(),
                terminal_commands: request.terminal_commands.clone(),
                had_reasoning: request.had_reasoning,
            });
        }

        SessionAnalytics {
            session_id: session.session_id.clone(),
            repository_path: session.repository_path.clone(),
            title: session.custom_title.clone(),
            model_id: session.model.id.clone(),
            model_name: session.model.name.clone(),
            rates,
            rates_source,
            credits_partial: true,
            turn_count: session.requests.len(),
            total_output_tokens,
            total_input_tokens: None,
            total_cached_tokens: None,
            total_output_credits,
            total_credits: None,
            total_elapsed_ms,
            wall_clock_ms: wall_clock_ms(session),
            turns,
            per_model: Vec::new(),
            tool_usage: sorted_aggregates(tool_counts),
            skill_usage: sorted_aggregates(skill_counts),
            subagent_usage: sorted_aggregates(subagent_counts),
        }
    }

    /// Build analytics from a parsed Copilot CLI `events.jsonl` session.
    ///
    /// CLI logs expose per-turn output tokens and, when the session has ended,
    /// full per-model usage (input/output/cache/reasoning) via `session.shutdown`.
    pub fn from_cli(session: &CliSession) -> Self {
        let primary = session.primary_model.clone();
        let rates = primary.as_deref().and_then(builtin_rates);

        let total_output_tokens: u64 = session.turns.iter().map(|t| t.output_tokens).sum();
        let total_elapsed_ms: i64 = session.turns.iter().map(|t| t.elapsed_ms()).sum();

        let mut turns = Vec::with_capacity(session.turns.len());
        let mut tool_counts: HashMap<String, usize> = HashMap::new();
        let mut skill_counts: HashMap<String, usize> = HashMap::new();
        let mut subagent_counts: HashMap<String, usize> = HashMap::new();
        let mut total_output_credits = 0.0;

        for turn in &session.turns {
            let output_credits = rates
                .map(|r| r.output_credits(turn.output_tokens))
                .unwrap_or(0.0);
            total_output_credits += output_credits;

            for tool in &turn.tools {
                *tool_counts.entry(tool.clone()).or_default() += 1;
            }
            for skill in &turn.skills {
                *skill_counts.entry(skill.clone()).or_default() += 1;
            }
            for subagent in &turn.subagents {
                *subagent_counts.entry(subagent.clone()).or_default() += 1;
            }

            turns.push(TurnMetrics {
                index: turn.index,
                request_id: None,
                model: turn.model.clone(),
                mode: turn.mode.clone(),
                timestamp_ms: turn.started_ms,
                output_tokens: turn.output_tokens,
                input_tokens: None,
                cached_tokens: None,
                elapsed_ms: Some(turn.elapsed_ms()),
                first_progress_ms: None,
                output_credits,
                credits: None,
                pct_output_tokens: percent(turn.output_tokens, total_output_tokens),
                pct_time: percent_i64(turn.elapsed_ms(), total_elapsed_ms),
                tools: turn.tools.clone(),
                subagents: turn.subagents.clone(),
                skills: turn.skills.clone(),
                terminal_commands: turn.terminal_commands.clone(),
                had_reasoning: false,
            });
        }

        let mut per_model = Vec::with_capacity(session.model_usage.len());
        let mut summed_input = 0u64;
        let mut summed_cached = 0u64;
        for usage in &session.model_usage {
            let model_rates = builtin_rates(&usage.name);
            let credits = model_rates.map(|r| {
                r.credits(
                    usage.input_tokens,
                    usage.output_tokens,
                    usage.cache_read_tokens + usage.cache_write_tokens,
                )
            });
            summed_input += usage.input_tokens;
            summed_cached += usage.cache_read_tokens + usage.cache_write_tokens;
            per_model.push(ModelUsage {
                name: usage.name.clone(),
                requests: usage.requests,
                input_tokens: usage.input_tokens,
                output_tokens: usage.output_tokens,
                cache_read_tokens: usage.cache_read_tokens,
                cache_write_tokens: usage.cache_write_tokens,
                reasoning_tokens: usage.reasoning_tokens,
                reported_cost: usage.reported_cost,
                credits,
            });
        }

        let has_full_usage = !per_model.is_empty();
        let (total_input_tokens, total_cached_tokens, total_credits) = if has_full_usage {
            let full: f64 = per_model.iter().filter_map(|m| m.credits).sum();
            let total_credits = if per_model.iter().any(|m| m.credits.is_some()) {
                Some(full)
            } else {
                None
            };
            (Some(summed_input), Some(summed_cached), total_credits)
        } else {
            (None, None, None)
        };

        let rates_source = if rates.is_some() {
            RatesSource::Builtin
        } else {
            RatesSource::Unknown
        };

        SessionAnalytics {
            session_id: session.session_id.clone(),
            repository_path: session.repository_path.clone(),
            title: session.cwd.clone(),
            model_id: primary.clone(),
            model_name: primary,
            rates,
            rates_source,
            credits_partial: !has_full_usage,
            turn_count: session.turns.len(),
            total_output_tokens,
            total_input_tokens,
            total_cached_tokens,
            total_output_credits,
            total_credits,
            total_elapsed_ms,
            wall_clock_ms: cli_wall_clock_ms(session),
            turns,
            per_model,
            tool_usage: sorted_aggregates(tool_counts),
            skill_usage: sorted_aggregates(skill_counts),
            subagent_usage: sorted_aggregates(subagent_counts),
        }
    }
}

/// Wall-clock span across CLI turns: latest end minus earliest start, in ms.
fn cli_wall_clock_ms(session: &CliSession) -> Option<i64> {
    let start = session.turns.iter().filter_map(|t| t.started_ms).min()?;
    let end = session.turns.iter().filter_map(|t| t.ended_ms).max()?;
    Some(end - start)
}

fn resolve_rates(model: &ChatModel) -> (Option<ModelRates>, RatesSource) {
    if let (Some(input), Some(output), Some(cache)) =
        (model.input_per_m, model.output_per_m, model.cache_per_m)
    {
        return (
            Some(ModelRates::new(input, output, cache)),
            RatesSource::Embedded,
        );
    }
    for key in [model.id.as_deref(), model.name.as_deref()]
        .into_iter()
        .flatten()
    {
        if let Some(rates) = builtin_rates(key) {
            return (Some(rates), RatesSource::Builtin);
        }
    }
    (None, RatesSource::Unknown)
}

/// Wall-clock span: latest turn end minus earliest turn start, in ms.
fn wall_clock_ms(session: &ChatSession) -> Option<i64> {
    let starts: Vec<i64> = session
        .requests
        .iter()
        .filter_map(|r| r.timestamp_ms)
        .collect();
    let start = *starts.iter().min()?;
    let end = session
        .requests
        .iter()
        .filter_map(|r| Some(r.timestamp_ms? + r.elapsed_ms.unwrap_or(0)))
        .max()?;
    Some(end - start)
}

fn percent(part: u64, whole: u64) -> f64 {
    if whole == 0 {
        0.0
    } else {
        part as f64 / whole as f64 * 100.0
    }
}

fn percent_i64(part: i64, whole: i64) -> f64 {
    if whole == 0 {
        0.0
    } else {
        part as f64 / whole as f64 * 100.0
    }
}

fn sorted_aggregates(counts: HashMap<String, usize>) -> Vec<Aggregate> {
    let mut aggregates: Vec<Aggregate> = counts
        .into_iter()
        .map(|(name, count)| Aggregate { name, count })
        .collect();
    aggregates.sort_by(|a, b| b.count.cmp(&a.count).then_with(|| a.name.cmp(&b.name)));
    aggregates
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::chat_log::{ChatModel, ChatRequest};

    fn session_with_rates() -> ChatSession {
        ChatSession {
            session_id: Some("s".into()),
            repository_path: None,
            custom_title: None,
            created_at_ms: Some(0),
            model: ChatModel {
                id: Some("gpt-5.5".into()),
                name: Some("GPT-5.5".into()),
                input_per_m: Some(500.0),
                output_per_m: Some(3000.0),
                cache_per_m: Some(50.0),
            },
            requests: vec![
                ChatRequest {
                    completion_tokens: 1000,
                    elapsed_ms: Some(2000),
                    timestamp_ms: Some(1000),
                    ..ChatRequest::default()
                },
                ChatRequest {
                    completion_tokens: 3000,
                    elapsed_ms: Some(6000),
                    timestamp_ms: Some(4000),
                    ..ChatRequest::default()
                },
            ],
        }
    }

    #[test]
    fn computes_totals_and_percentages() {
        let analytics = SessionAnalytics::from_chat(&session_with_rates());
        assert_eq!(analytics.total_output_tokens, 4000);
        assert_eq!(analytics.rates_source, RatesSource::Embedded);
        assert!((analytics.total_output_credits - 12.0).abs() < 1e-9);
        assert!((analytics.turns[0].pct_output_tokens - 25.0).abs() < 1e-9);
        assert!((analytics.turns[1].pct_output_tokens - 75.0).abs() < 1e-9);
        assert!((analytics.turns[0].pct_time - 25.0).abs() < 1e-9);
        // Wall clock: max end (4000+6000) - min start (1000) == 9000.
        assert_eq!(analytics.wall_clock_ms, Some(9000));
    }

    #[test]
    fn unknown_model_yields_no_credits() {
        let mut session = session_with_rates();
        session.model = ChatModel::default();
        let analytics = SessionAnalytics::from_chat(&session);
        assert_eq!(analytics.rates_source, RatesSource::Unknown);
        assert_eq!(analytics.total_output_credits, 0.0);
    }
}
