//! Typed analytics derived from a parsed session: per-turn tokens, credits,
//! percentages, timeline, and tool/skill/subagent frequencies.

use std::collections::HashMap;

use serde::Serialize;

use crate::chat_log::{ChatModel, ChatSession};
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

/// Metrics for a single request/response turn.
#[derive(Debug, Clone, Serialize)]
pub struct TurnMetrics {
    pub index: usize,
    pub request_id: Option<String>,
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
            tool_usage: sorted_aggregates(tool_counts),
            skill_usage: sorted_aggregates(skill_counts),
            subagent_usage: sorted_aggregates(subagent_counts),
        }
    }
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
