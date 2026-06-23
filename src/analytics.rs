//! Typed analytics derived from a parsed session: per-turn tokens, credits,
//! percentages, timeline, and tool/skill/subagent frequencies.

use std::collections::{HashMap, HashSet};

use serde::{Deserialize, Serialize};

use crate::chat_log::{ChatModel, ChatSession};
use crate::cli_log::CliSession;
use crate::pricing::{builtin_rates, ModelRates};

/// Where the active credit rates came from.
#[derive(Debug, Clone, Copy, PartialEq, Deserialize, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum RatesSource {
    /// Pricing embedded in the session log header.
    Embedded,
    /// Pricing from the built-in table.
    Builtin,
    /// No pricing available; credits cannot be computed.
    Unknown,
}

/// Where the displayed total credit value came from.
#[derive(Debug, Clone, Copy, PartialEq, Deserialize, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum CreditSource {
    /// Explicit cost/credit values reported by the session log.
    Reported,
    /// Computed from token usage and known model rates.
    Estimated,
    /// Some models reported credits while others needed estimates.
    Mixed,
    /// Only output-side credits are known.
    OutputOnly,
    /// No usable credit value is available.
    Unknown,
}

/// A named frequency count (tools, skills, subagents).
#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Aggregate {
    pub name: String,
    pub count: usize,
}

/// One activity observed within a turn, in source-log order.
#[derive(Debug, Clone, PartialEq, Deserialize, Serialize)]
pub struct ActivityEvent {
    pub kind: String,
    pub name: String,
}

/// Usage associated with activity calls in their enclosing request or turn.
#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ActivityUsage {
    pub name: String,
    pub calls: usize,
    pub request_count: usize,
    pub input_tokens: Option<u64>,
    pub output_tokens: u64,
    pub cached_tokens: Option<u64>,
    pub cache_read_tokens: Option<u64>,
    pub cache_write_tokens: Option<u64>,
    pub reasoning_tokens: Option<u64>,
    pub output_credits: f64,
    pub credits: Option<f64>,
    pub credit_source: CreditSource,
}

/// Per-model token usage and cost reported for a session.
#[derive(Debug, Clone, Deserialize, Serialize)]
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
    /// Full AIC credit estimate from the built-in rate table, when the model is known.
    pub estimated_credits: Option<f64>,
    /// Effective credits used for display: reported cost first, estimate second.
    pub credits: Option<f64>,
}

/// Metrics for a single request/response turn.
#[derive(Debug, Clone, Deserialize, Serialize)]
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
    pub activity_events: Vec<ActivityEvent>,
    pub had_reasoning: bool,
}

impl TurnMetrics {
    /// Best available credit value for this turn.
    ///
    /// Prefers full credits (input + output + cache) when available; falls back
    /// to output-only credits when rates are known but full data is not.
    pub fn credit_value(&self) -> Option<f64> {
        self.credits.or(if self.output_credits > 0.0 {
            Some(self.output_credits)
        } else {
            None
        })
    }

    /// Human-readable AIC string for display (e.g. `"3.5"` or `"n/a"`).
    pub fn credit_display(&self) -> String {
        self.credit_value()
            .map(|v| format!("{v:.1}"))
            .unwrap_or_else(|| "n/a".to_string())
    }
}

/// Aggregated analytics for an entire session.
#[derive(Debug, Clone, Deserialize, Serialize)]
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
    pub total_cache_read_tokens: Option<u64>,
    pub total_cache_write_tokens: Option<u64>,
    pub total_reasoning_tokens: Option<u64>,
    pub total_output_credits: f64,
    pub total_estimated_credits: Option<f64>,
    pub total_credits: Option<f64>,
    pub credit_source: CreditSource,
    pub total_elapsed_ms: i64,
    pub wall_clock_ms: Option<i64>,
    pub turns: Vec<TurnMetrics>,
    pub per_model: Vec<ModelUsage>,
    pub tool_usage: Vec<Aggregate>,
    pub skill_usage: Vec<Aggregate>,
    pub subagent_usage: Vec<Aggregate>,
    pub tool_activity_usage: Vec<ActivityUsage>,
    pub skill_activity_usage: Vec<ActivityUsage>,
    pub subagent_activity_usage: Vec<ActivityUsage>,
}

impl SessionAnalytics {
    /// Build analytics from a parsed VS Code chat session.
    ///
    /// The chat format exposes only output (completion) tokens, so credits are
    /// output-only and [`SessionAnalytics::credits_partial`] is `true`.
    pub fn from_chat(session: &ChatSession) -> Self {
        let (rates, rates_source) = resolve_rates(&session.model);

        let total_output_tokens: u64 = session.requests.iter().map(|r| r.completion_tokens).sum();
        let total_input_tokens = sum_optional_u64(session.requests.iter().map(|r| r.prompt_tokens));
        let total_elapsed_ms: i64 = session.requests.iter().filter_map(|r| r.elapsed_ms).sum();

        let mut turns = Vec::with_capacity(session.requests.len());
        let mut tool_counts: HashMap<String, usize> = HashMap::new();
        let mut skill_counts: HashMap<String, usize> = HashMap::new();
        let mut subagent_counts: HashMap<String, usize> = HashMap::new();
        let mut tool_activity: HashMap<String, ActivityUsageAccumulator> = HashMap::new();
        let mut skill_activity: HashMap<String, ActivityUsageAccumulator> = HashMap::new();
        let mut subagent_activity: HashMap<String, ActivityUsageAccumulator> = HashMap::new();
        let mut total_output_credits = 0.0;
        let mut total_reported_credits = 0.0;
        let mut has_reported_credits = false;

        for (index, request) in session.requests.iter().enumerate() {
            let output_credits = rates
                .map(|r| r.output_credits(request.completion_tokens))
                .unwrap_or(0.0);
            total_output_credits += output_credits;
            if let Some(reported_credits) = request.reported_credits {
                total_reported_credits += reported_credits;
                has_reported_credits = true;
            }

            for tool in &request.tools {
                *tool_counts.entry(tool.clone()).or_default() += 1;
            }
            record_activity_calls(&mut tool_activity, &request.tools);
            record_chat_activity_usage(&mut tool_activity, &request.tools, request, output_credits);
            for skill in &request.skills {
                *skill_counts.entry(skill.clone()).or_default() += 1;
            }
            record_activity_calls(&mut skill_activity, &request.skills);
            record_chat_activity_usage(
                &mut skill_activity,
                &request.skills,
                request,
                output_credits,
            );
            for subagent in &request.subagents {
                *subagent_counts.entry(subagent.clone()).or_default() += 1;
            }
            record_activity_calls(&mut subagent_activity, &request.subagents);
            record_chat_activity_usage(
                &mut subagent_activity,
                &request.subagents,
                request,
                output_credits,
            );

            turns.push(TurnMetrics {
                index,
                request_id: request.request_id.clone(),
                model: session.model.id.clone(),
                mode: None,
                timestamp_ms: request.timestamp_ms,
                output_tokens: request.completion_tokens,
                input_tokens: request.prompt_tokens,
                cached_tokens: None,
                elapsed_ms: request.elapsed_ms,
                first_progress_ms: request.first_progress_ms,
                output_credits,
                credits: request.reported_credits,
                pct_output_tokens: percent(request.completion_tokens, total_output_tokens),
                pct_time: percent_i64(request.elapsed_ms.unwrap_or(0), total_elapsed_ms),
                tools: request.tools.clone(),
                subagents: request.subagents.clone(),
                skills: request.skills.clone(),
                terminal_commands: request.terminal_commands.clone(),
                activity_events: request.activity_events.clone(),
                had_reasoning: request.had_reasoning,
            });
        }

        let total_estimated_credits = if has_reported_credits && rates.is_some() {
            Some(total_output_credits)
        } else {
            None
        };
        let total_credits = has_reported_credits.then_some(total_reported_credits);
        let credit_source = if has_reported_credits {
            CreditSource::Reported
        } else if rates.is_some() {
            CreditSource::OutputOnly
        } else {
            CreditSource::Unknown
        };

        SessionAnalytics {
            session_id: session.session_id.clone(),
            repository_path: session.repository_path.clone(),
            title: session.custom_title.clone(),
            model_id: session.model.id.clone(),
            model_name: session.model.name.clone(),
            rates,
            rates_source,
            credits_partial: !has_reported_credits,
            turn_count: session.requests.len(),
            total_output_tokens,
            total_input_tokens,
            total_cached_tokens: None,
            total_cache_read_tokens: None,
            total_cache_write_tokens: None,
            total_reasoning_tokens: None,
            total_output_credits,
            total_estimated_credits,
            total_credits,
            credit_source,
            total_elapsed_ms,
            wall_clock_ms: wall_clock_ms(session),
            turns,
            per_model: Vec::new(),
            tool_usage: sorted_aggregates(tool_counts),
            skill_usage: sorted_aggregates(skill_counts),
            subagent_usage: sorted_aggregates(subagent_counts),
            tool_activity_usage: sorted_activity_usage(tool_activity),
            skill_activity_usage: sorted_activity_usage(skill_activity),
            subagent_activity_usage: sorted_activity_usage(subagent_activity),
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
        let mut tool_activity: HashMap<String, ActivityUsageAccumulator> = HashMap::new();
        let mut skill_activity: HashMap<String, ActivityUsageAccumulator> = HashMap::new();
        let mut subagent_activity: HashMap<String, ActivityUsageAccumulator> = HashMap::new();
        let mut total_output_credits = 0.0;

        for turn in &session.turns {
            let output_credits = rates
                .map(|r| r.output_credits(turn.output_tokens))
                .unwrap_or(0.0);
            total_output_credits += output_credits;

            for tool in &turn.tools {
                *tool_counts.entry(tool.clone()).or_default() += 1;
            }
            record_activity_calls(&mut tool_activity, &turn.tools);
            record_cli_activity_usage(
                &mut tool_activity,
                &turn.tools,
                turn.output_tokens,
                output_credits,
            );
            for skill in &turn.skills {
                *skill_counts.entry(skill.clone()).or_default() += 1;
            }
            record_activity_calls(&mut skill_activity, &turn.skills);
            record_cli_activity_usage(
                &mut skill_activity,
                &turn.skills,
                turn.output_tokens,
                output_credits,
            );
            for subagent in &turn.subagents {
                *subagent_counts.entry(subagent.clone()).or_default() += 1;
            }
            record_activity_calls(&mut subagent_activity, &turn.subagents);
            record_cli_activity_usage(
                &mut subagent_activity,
                &turn.subagents,
                turn.output_tokens,
                output_credits,
            );

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
                activity_events: turn.activity_events.clone(),
                had_reasoning: false,
            });
        }

        let mut per_model = Vec::with_capacity(session.model_usage.len());
        let mut summed_input = 0u64;
        let mut summed_output = 0u64;
        let mut summed_cache_read = 0u64;
        let mut summed_cache_write = 0u64;
        let mut summed_cached = 0u64;
        let mut summed_reasoning = 0u64;
        for usage in &session.model_usage {
            let model_rates = builtin_rates(&usage.name);
            let estimated_credits = model_rates.map(|r| {
                r.credits(
                    usage.input_tokens,
                    usage.output_tokens,
                    usage.cache_read_tokens + usage.cache_write_tokens,
                )
            });
            let credits = usage.reported_cost.or(estimated_credits);
            summed_input += usage.input_tokens;
            summed_output += usage.output_tokens;
            summed_cache_read += usage.cache_read_tokens;
            summed_cache_write += usage.cache_write_tokens;
            summed_cached += usage.cache_read_tokens + usage.cache_write_tokens;
            summed_reasoning += usage.reasoning_tokens;
            per_model.push(ModelUsage {
                name: usage.name.clone(),
                requests: usage.requests,
                input_tokens: usage.input_tokens,
                output_tokens: usage.output_tokens,
                cache_read_tokens: usage.cache_read_tokens,
                cache_write_tokens: usage.cache_write_tokens,
                reasoning_tokens: usage.reasoning_tokens,
                reported_cost: usage.reported_cost,
                estimated_credits,
                credits,
            });
        }

        let has_full_usage = !per_model.is_empty();
        let (
            total_output_tokens,
            total_input_tokens,
            total_cached_tokens,
            total_cache_read_tokens,
            total_cache_write_tokens,
            total_reasoning_tokens,
            total_estimated_credits,
            total_credits,
            credit_source,
        ) = if has_full_usage {
            let estimated = sum_optional(per_model.iter().map(|m| m.estimated_credits));
            let reported_count = per_model
                .iter()
                .filter(|m| m.reported_cost.is_some())
                .count();
            let effective = sum_optional(per_model.iter().map(|m| m.credits));
            let credit_source = match (reported_count, effective) {
                (0, Some(_)) => CreditSource::Estimated,
                (count, Some(_)) if count == per_model.len() => CreditSource::Reported,
                (_, Some(_)) => CreditSource::Mixed,
                (_, None) => CreditSource::Unknown,
            };
            (
                summed_output,
                Some(summed_input),
                Some(summed_cached),
                Some(summed_cache_read),
                Some(summed_cache_write),
                Some(summed_reasoning),
                estimated,
                effective,
                credit_source,
            )
        } else {
            (
                total_output_tokens,
                None,
                None,
                None,
                None,
                None,
                None,
                None,
                CreditSource::Unknown,
            )
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
            total_cache_read_tokens,
            total_cache_write_tokens,
            total_reasoning_tokens,
            total_output_credits,
            total_estimated_credits,
            total_credits,
            credit_source,
            total_elapsed_ms,
            wall_clock_ms: cli_wall_clock_ms(session),
            turns,
            per_model,
            tool_usage: sorted_aggregates(tool_counts),
            skill_usage: sorted_aggregates(skill_counts),
            subagent_usage: sorted_aggregates(subagent_counts),
            tool_activity_usage: sorted_activity_usage(tool_activity),
            skill_activity_usage: sorted_activity_usage(skill_activity),
            subagent_activity_usage: sorted_activity_usage(subagent_activity),
        }
    }
}

#[derive(Debug, Clone)]
struct ActivityUsageAccumulator {
    name: String,
    calls: usize,
    request_count: usize,
    input_tokens: u64,
    saw_input_tokens: bool,
    output_tokens: u64,
    cached_tokens: u64,
    saw_cached_tokens: bool,
    cache_read_tokens: u64,
    saw_cache_read_tokens: bool,
    cache_write_tokens: u64,
    saw_cache_write_tokens: bool,
    reasoning_tokens: u64,
    saw_reasoning_tokens: bool,
    output_credits: f64,
    credits: f64,
    reported_count: usize,
    estimated_count: usize,
    output_only_count: usize,
}

impl ActivityUsageAccumulator {
    fn new(name: String) -> Self {
        Self {
            name,
            calls: 0,
            request_count: 0,
            input_tokens: 0,
            saw_input_tokens: false,
            output_tokens: 0,
            cached_tokens: 0,
            saw_cached_tokens: false,
            cache_read_tokens: 0,
            saw_cache_read_tokens: false,
            cache_write_tokens: 0,
            saw_cache_write_tokens: false,
            reasoning_tokens: 0,
            saw_reasoning_tokens: false,
            output_credits: 0.0,
            credits: 0.0,
            reported_count: 0,
            estimated_count: 0,
            output_only_count: 0,
        }
    }

    fn add_call(&mut self) {
        self.calls += 1;
    }

    fn add_associated_usage(&mut self, usage: AssociatedUsage) {
        self.request_count += 1;
        if let Some(input_tokens) = usage.input_tokens {
            self.input_tokens += input_tokens;
            self.saw_input_tokens = true;
        }
        self.output_tokens += usage.output_tokens;
        if let Some(cached_tokens) = usage.cached_tokens {
            self.cached_tokens += cached_tokens;
            self.saw_cached_tokens = true;
        }
        if let Some(cache_read_tokens) = usage.cache_read_tokens {
            self.cache_read_tokens += cache_read_tokens;
            self.saw_cache_read_tokens = true;
        }
        if let Some(cache_write_tokens) = usage.cache_write_tokens {
            self.cache_write_tokens += cache_write_tokens;
            self.saw_cache_write_tokens = true;
        }
        if let Some(reasoning_tokens) = usage.reasoning_tokens {
            self.reasoning_tokens += reasoning_tokens;
            self.saw_reasoning_tokens = true;
        }
        self.output_credits += usage.output_credits;
        match usage.credit_source {
            CreditSource::Reported => {
                if let Some(credits) = usage.credits {
                    self.credits += credits;
                    self.reported_count += 1;
                }
            }
            CreditSource::Estimated => {
                if let Some(credits) = usage.credits {
                    self.credits += credits;
                    self.estimated_count += 1;
                }
            }
            CreditSource::Mixed => {
                if let Some(credits) = usage.credits {
                    self.credits += credits;
                    self.reported_count += 1;
                    self.estimated_count += 1;
                }
            }
            CreditSource::OutputOnly => self.output_only_count += 1,
            CreditSource::Unknown => {}
        }
    }

    fn into_usage(self) -> ActivityUsage {
        let credit_source = match (
            self.reported_count,
            self.estimated_count,
            self.output_only_count,
        ) {
            (0, 0, count) if count > 0 => CreditSource::OutputOnly,
            (0, 0, _) => CreditSource::Unknown,
            (reported, 0, 0) if reported > 0 => CreditSource::Reported,
            (0, estimated, 0) if estimated > 0 => CreditSource::Estimated,
            _ => CreditSource::Mixed,
        };
        ActivityUsage {
            name: self.name,
            calls: self.calls,
            request_count: self.request_count,
            input_tokens: self.saw_input_tokens.then_some(self.input_tokens),
            output_tokens: self.output_tokens,
            cached_tokens: self.saw_cached_tokens.then_some(self.cached_tokens),
            cache_read_tokens: self.saw_cache_read_tokens.then_some(self.cache_read_tokens),
            cache_write_tokens: self
                .saw_cache_write_tokens
                .then_some(self.cache_write_tokens),
            reasoning_tokens: self.saw_reasoning_tokens.then_some(self.reasoning_tokens),
            output_credits: self.output_credits,
            credits: matches!(
                credit_source,
                CreditSource::Reported | CreditSource::Estimated | CreditSource::Mixed
            )
            .then_some(self.credits),
            credit_source,
        }
    }
}

#[derive(Clone, Copy)]
struct AssociatedUsage {
    input_tokens: Option<u64>,
    output_tokens: u64,
    cached_tokens: Option<u64>,
    cache_read_tokens: Option<u64>,
    cache_write_tokens: Option<u64>,
    reasoning_tokens: Option<u64>,
    output_credits: f64,
    credits: Option<f64>,
    credit_source: CreditSource,
}

fn record_activity_calls(
    activity: &mut HashMap<String, ActivityUsageAccumulator>,
    names: &[String],
) {
    for name in names {
        activity
            .entry(name.clone())
            .or_insert_with(|| ActivityUsageAccumulator::new(name.clone()))
            .add_call();
    }
}

fn record_chat_activity_usage(
    activity: &mut HashMap<String, ActivityUsageAccumulator>,
    names: &[String],
    request: &crate::chat_log::ChatRequest,
    output_credits: f64,
) {
    let usage = AssociatedUsage {
        input_tokens: request.prompt_tokens,
        output_tokens: request.completion_tokens,
        cached_tokens: None,
        cache_read_tokens: None,
        cache_write_tokens: None,
        reasoning_tokens: None,
        output_credits,
        credits: request.reported_credits,
        credit_source: if request.reported_credits.is_some() {
            CreditSource::Reported
        } else if output_credits > 0.0 {
            CreditSource::OutputOnly
        } else {
            CreditSource::Unknown
        },
    };
    record_associated_usage(activity, names, usage);
}

fn record_cli_activity_usage(
    activity: &mut HashMap<String, ActivityUsageAccumulator>,
    names: &[String],
    output_tokens: u64,
    output_credits: f64,
) {
    let usage = AssociatedUsage {
        input_tokens: None,
        output_tokens,
        cached_tokens: None,
        cache_read_tokens: None,
        cache_write_tokens: None,
        reasoning_tokens: None,
        output_credits,
        credits: None,
        credit_source: if output_credits > 0.0 {
            CreditSource::OutputOnly
        } else {
            CreditSource::Unknown
        },
    };
    record_associated_usage(activity, names, usage);
}

fn record_associated_usage(
    activity: &mut HashMap<String, ActivityUsageAccumulator>,
    names: &[String],
    usage: AssociatedUsage,
) {
    let mut unique_names = HashSet::new();
    for name in names {
        if unique_names.insert(name.as_str()) {
            activity
                .entry(name.clone())
                .or_insert_with(|| ActivityUsageAccumulator::new(name.clone()))
                .add_associated_usage(usage);
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

fn sum_optional(values: impl Iterator<Item = Option<f64>>) -> Option<f64> {
    let mut total = 0.0;
    let mut saw_value = false;
    for value in values.flatten() {
        total += value;
        saw_value = true;
    }
    saw_value.then_some(total)
}

fn sum_optional_u64(values: impl Iterator<Item = Option<u64>>) -> Option<u64> {
    let mut total = 0u64;
    let mut saw_value = false;
    for value in values.flatten() {
        total += value;
        saw_value = true;
    }
    saw_value.then_some(total)
}

fn sorted_aggregates(counts: HashMap<String, usize>) -> Vec<Aggregate> {
    let mut aggregates: Vec<Aggregate> = counts
        .into_iter()
        .map(|(name, count)| Aggregate { name, count })
        .collect();
    aggregates.sort_by(|a, b| b.count.cmp(&a.count).then_with(|| a.name.cmp(&b.name)));
    aggregates
}

fn sorted_activity_usage(
    activity: HashMap<String, ActivityUsageAccumulator>,
) -> Vec<ActivityUsage> {
    let mut usages: Vec<ActivityUsage> = activity
        .into_values()
        .map(ActivityUsageAccumulator::into_usage)
        .collect();
    usages.sort_by(|a, b| b.calls.cmp(&a.calls).then_with(|| a.name.cmp(&b.name)));
    usages
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
