//! Parser for Copilot CLI `events.jsonl` session logs.
//!
//! Each line is a flat event `{type, data, id, parentId, timestamp}` where
//! `timestamp` is an RFC 3339 / ISO-8601 UTC string. Turns are delimited by
//! `user.message` events; assistant output, tool calls, and skill invocations
//! between two user messages belong to the same turn. When a session has ended,
//! `session.shutdown` carries authoritative per-model token usage.

use anyhow::{Context, Result};
use serde_json::Value;

use crate::analytics::ActivityEvent;
use crate::log_fields::log_fields;

/// A reconstructed Copilot CLI session.
#[derive(Debug, Clone, Default)]
pub struct CliSession {
    pub session_id: Option<String>,
    pub repository_path: Option<String>,
    pub cwd: Option<String>,
    /// Model with the most requests (or the last selected), used for rates.
    pub primary_model: Option<String>,
    pub turns: Vec<CliTurn>,
    pub model_usage: Vec<CliModelUsage>,
}

/// A single user-prompt-to-response turn.
#[derive(Debug, Clone, Default)]
pub struct CliTurn {
    pub index: usize,
    pub mode: Option<String>,
    pub model: Option<String>,
    pub output_tokens: u64,
    pub started_ms: Option<i64>,
    pub ended_ms: Option<i64>,
    pub tools: Vec<String>,
    pub skills: Vec<String>,
    pub subagents: Vec<String>,
    pub terminal_commands: Vec<String>,
    pub activity_events: Vec<ActivityEvent>,
}

impl CliTurn {
    /// Elapsed milliseconds for the turn (0 when timing is unavailable).
    pub fn elapsed_ms(&self) -> i64 {
        match (self.started_ms, self.ended_ms) {
            (Some(start), Some(end)) if end >= start => end - start,
            _ => 0,
        }
    }
}

/// Full per-model token usage from `session.shutdown`.
#[derive(Debug, Clone, Default)]
pub struct CliModelUsage {
    pub name: String,
    pub requests: u64,
    pub reported_cost: Option<f64>,
    pub input_tokens: u64,
    pub output_tokens: u64,
    pub cache_read_tokens: u64,
    pub cache_write_tokens: u64,
    pub reasoning_tokens: u64,
}

/// Heuristic: does `first_line` look like a Copilot CLI event?
pub fn looks_like_cli_log(first_line: &str) -> bool {
    let Ok(value) = serde_json::from_str::<Value>(first_line) else {
        return false;
    };
    let f = &log_fields().cli.event;
    value.get(&f.r#type).and_then(Value::as_str).is_some()
        && value.get(&f.data).is_some()
        && value.get(&f.chat_kind).is_none()
}

/// Parse a CLI events log from its full text contents.
pub fn parse_str(data: &str) -> Result<CliSession> {
    let f = &log_fields().cli;
    let mut session = CliSession::default();
    let mut current_mode: Option<String> = None;
    let mut current_model: Option<String> = None;

    for (line_no, raw) in data.lines().enumerate() {
        let line = raw.trim();
        if line.is_empty() {
            continue;
        }
        let event: Value = serde_json::from_str(line)
            .with_context(|| format!("invalid JSON on line {}", line_no + 1))?;
        let event_type = event
            .get(&f.event.r#type)
            .and_then(Value::as_str)
            .unwrap_or("");
        let data = event.get(&f.event.data).cloned().unwrap_or(Value::Null);
        let ts = event
            .get(&f.event.timestamp)
            .and_then(Value::as_str)
            .and_then(parse_iso_ms);

        match event_type {
            t if t == f.types.session_start => {
                session.session_id = data
                    .get(&f.data.session_id)
                    .and_then(Value::as_str)
                    .map(String::from);
                session.cwd = data
                    .pointer(&f.data.cwd)
                    .and_then(Value::as_str)
                    .map(String::from);
            }
            t if t == f.types.session_model_change => {
                if let Some(model) = data.get(&f.data.new_model).and_then(Value::as_str) {
                    current_model = Some(model.to_string());
                }
            }
            t if t == f.types.session_mode_changed => {
                if let Some(mode) = data.get(&f.data.new_mode).and_then(Value::as_str) {
                    current_mode = Some(mode.to_string());
                }
            }
            t if t == f.types.user_message => {
                let mode = data
                    .get(&f.data.agent_mode)
                    .and_then(Value::as_str)
                    .map(String::from)
                    .or_else(|| current_mode.clone());
                session.turns.push(CliTurn {
                    index: session.turns.len(),
                    mode,
                    model: current_model.clone(),
                    started_ms: ts,
                    ended_ms: ts,
                    ..CliTurn::default()
                });
            }
            t if t == f.types.assistant_message => {
                if let Some(turn) = session.turns.last_mut() {
                    turn.output_tokens += data
                        .get(&f.data.output_tokens)
                        .and_then(Value::as_u64)
                        .unwrap_or(0);
                    bump_end(turn, ts);
                }
            }
            t if t == f.types.tool_execution_start => {
                if let Some(turn) = session.turns.last_mut() {
                    if let Some(name) = data.get(&f.data.tool_name).and_then(Value::as_str) {
                        turn.tools.push(name.to_string());
                        push_activity_event(turn, "tool", name);
                        if is_subagent(name) {
                            let agent = data
                                .pointer(&f.data.agent_name)
                                .and_then(Value::as_str)
                                .unwrap_or(name);
                            turn.subagents.push(agent.to_string());
                            push_activity_event(turn, "agent", agent);
                        }
                    }
                    if let Some(command) = data.pointer(&f.data.command).and_then(Value::as_str) {
                        turn.terminal_commands.push(command.to_string());
                        push_activity_event(turn, "cmd", command);
                    }
                    bump_end(turn, ts);
                }
            }
            t if t == f.types.skill_invoked => {
                if let Some(turn) = session.turns.last_mut() {
                    if let Some(name) = data.get(&f.data.skill_name).and_then(Value::as_str) {
                        turn.skills.push(name.to_string());
                        push_activity_event(turn, "skill", name);
                    }
                    bump_end(turn, ts);
                }
            }
            t if t == f.types.session_shutdown => {
                if let Some(metrics) = data.get(&f.data.model_metrics).and_then(Value::as_object) {
                    for (name, value) in metrics {
                        session.model_usage.push(model_usage_from(name, value));
                    }
                }
            }
            _ => {
                if let Some(turn) = session.turns.last_mut() {
                    bump_end(turn, ts);
                }
            }
        }
    }

    session.primary_model = pick_primary_model(&session, current_model);
    Ok(session)
}

fn push_activity_event(turn: &mut CliTurn, kind: &str, name: &str) {
    turn.activity_events.push(ActivityEvent {
        kind: kind.to_string(),
        name: name.to_string(),
    });
}

fn bump_end(turn: &mut CliTurn, ts: Option<i64>) {
    if let Some(ts) = ts {
        turn.ended_ms = Some(turn.ended_ms.map_or(ts, |end| end.max(ts)));
    }
}

fn is_subagent(tool_name: &str) -> bool {
    tool_name.to_lowercase().contains("subagent")
}

fn model_usage_from(name: &str, value: &Value) -> CliModelUsage {
    let f = &log_fields().cli.model_metrics;
    let get = |key: &str| {
        value
            .pointer(&format!("{}{key}", f.usage_prefix))
            .and_then(Value::as_u64)
            .unwrap_or(0)
    };
    CliModelUsage {
        name: name.to_string(),
        requests: value
            .pointer(&f.request_count)
            .and_then(Value::as_u64)
            .unwrap_or(0),
        reported_cost: value.pointer(&f.request_cost).and_then(Value::as_f64),
        input_tokens: get(&f.input_tokens),
        output_tokens: get(&f.output_tokens),
        cache_read_tokens: get(&f.cache_read_tokens),
        cache_write_tokens: get(&f.cache_write_tokens),
        reasoning_tokens: get(&f.reasoning_tokens),
    }
}

fn pick_primary_model(session: &CliSession, last_selected: Option<String>) -> Option<String> {
    session
        .model_usage
        .iter()
        .max_by_key(|m| m.requests)
        .map(|m| m.name.clone())
        .or(last_selected)
        .or_else(|| session.turns.iter().rev().find_map(|t| t.model.clone()))
}

/// Parse an RFC 3339 / ISO-8601 UTC timestamp (e.g. `2026-04-28T14:07:53.006Z`)
/// to epoch milliseconds. Returns `None` on malformed input.
fn parse_iso_ms(text: &str) -> Option<i64> {
    let text = text.strip_suffix('Z').unwrap_or(text);
    let (date, time) = text.split_once('T')?;
    let mut date_parts = date.split('-');
    let year: i64 = date_parts.next()?.parse().ok()?;
    let month: i64 = date_parts.next()?.parse().ok()?;
    let day: i64 = date_parts.next()?.parse().ok()?;

    let (clock, fraction) = match time.split_once('.') {
        Some((clock, frac)) => (clock, frac),
        None => (time, ""),
    };
    let mut clock_parts = clock.split(':');
    let hour: i64 = clock_parts.next()?.parse().ok()?;
    let minute: i64 = clock_parts.next()?.parse().ok()?;
    let second: i64 = clock_parts.next().unwrap_or("0").parse().ok()?;

    let millis: i64 = if fraction.is_empty() {
        0
    } else {
        let trimmed: String = fraction.chars().take(3).collect();
        let padded = format!("{trimmed:0<3}");
        padded.parse().ok()?
    };

    let days = days_from_civil(year, month, day);
    let seconds = ((days * 24 + hour) * 60 + minute) * 60 + second;
    Some(seconds * 1000 + millis)
}

/// Days since 1970-01-01 for a civil date (Howard Hinnant's algorithm).
fn days_from_civil(year: i64, month: i64, day: i64) -> i64 {
    let year = if month <= 2 { year - 1 } else { year };
    let era = if year >= 0 { year } else { year - 399 } / 400;
    let yoe = year - era * 400;
    let doy = (153 * (if month > 2 { month - 3 } else { month + 9 }) + 2) / 5 + day - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    era * 146097 + doe - 719468
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn detects_cli_log() {
        assert!(looks_like_cli_log(
            r#"{"type":"session.start","data":{"sessionId":"x"},"timestamp":"2026-01-01T00:00:00Z"}"#
        ));
        assert!(!looks_like_cli_log(r#"{"kind":0,"v":{"sessionId":"x"}}"#));
        assert!(!looks_like_cli_log("not json"));
    }

    #[test]
    fn parses_iso_to_epoch_ms() {
        // 1970-01-01T00:00:00Z == 0.
        assert_eq!(parse_iso_ms("1970-01-01T00:00:00Z"), Some(0));
        // One second + 500ms.
        assert_eq!(parse_iso_ms("1970-01-01T00:00:01.500Z"), Some(1500));
        // A known date: 2026-04-28T14:07:53.006Z.
        let a = parse_iso_ms("2026-04-28T14:07:53.006Z").unwrap();
        let b = parse_iso_ms("2026-04-28T14:07:55.006Z").unwrap();
        assert_eq!(b - a, 2000);
    }

    #[test]
    fn groups_turns_and_usage() {
        let data = concat!(
            r#"{"type":"session.start","data":{"sessionId":"s","context":{"cwd":"/tmp"}},"timestamp":"2026-01-01T00:00:00Z"}"#,
            "\n",
            r#"{"type":"session.model_change","data":{"newModel":"gpt-5.5"},"timestamp":"2026-01-01T00:00:01Z"}"#,
            "\n",
            r#"{"type":"user.message","data":{"agentMode":"plan","content":"hi"},"timestamp":"2026-01-01T00:00:02Z"}"#,
            "\n",
            r#"{"type":"assistant.message","data":{"outputTokens":100},"timestamp":"2026-01-01T00:00:04Z"}"#,
            "\n",
            r#"{"type":"tool.execution_start","data":{"toolName":"shell","arguments":{"command":"ls"}},"timestamp":"2026-01-01T00:00:05Z"}"#,
            "\n",
            r#"{"type":"skill.invoked","data":{"name":"scope-guard"},"timestamp":"2026-01-01T00:00:06Z"}"#,
            "\n",
            r#"{"type":"session.shutdown","data":{"modelMetrics":{"gpt-5.5":{"requests":{"count":3,"cost":12},"usage":{"inputTokens":1000,"outputTokens":100,"cacheReadTokens":500,"cacheWriteTokens":0,"reasoningTokens":20}}}},"timestamp":"2026-01-01T00:00:07Z"}"#,
            "\n",
        );
        let session = parse_str(data).unwrap();
        assert_eq!(session.session_id.as_deref(), Some("s"));
        assert_eq!(session.cwd.as_deref(), Some("/tmp"));
        assert_eq!(session.primary_model.as_deref(), Some("gpt-5.5"));
        assert_eq!(session.turns.len(), 1);
        let turn = &session.turns[0];
        assert_eq!(turn.mode.as_deref(), Some("plan"));
        assert_eq!(turn.model.as_deref(), Some("gpt-5.5"));
        assert_eq!(turn.output_tokens, 100);
        assert_eq!(turn.tools, vec!["shell"]);
        assert_eq!(turn.skills, vec!["scope-guard"]);
        assert_eq!(turn.terminal_commands, vec!["ls"]);
        assert_eq!(turn.elapsed_ms(), 4000);
        assert_eq!(session.model_usage.len(), 1);
        assert_eq!(session.model_usage[0].input_tokens, 1000);
        assert_eq!(session.model_usage[0].reasoning_tokens, 20);
    }
}
