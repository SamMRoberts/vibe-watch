//! Parser for VS Code Copilot Chat `.jsonl` session logs.
//!
//! The chat log is a delta journal. Each line is a JSON object:
//! - `{"kind":0,"v": <full object>}` — the initial session snapshot.
//! - `{"kind":1,"k":[path...],"v": <value>}` — set the value at `path`.
//! - `{"kind":2,"k":[path...],"v": [items]}` — extend the array at `path`.
//!
//! We reconstruct the final session state by applying every mutation onto a
//! [`serde_json::Value`], then extract the typed fields we report on. This keeps
//! the parser robust to fields we do not care about.

use anyhow::{Context, Result};
use serde_json::{Map, Value};

use crate::analytics::ActivityEvent;
use crate::log_fields::{log_fields, ChatFields};

/// A reconstructed VS Code Copilot Chat session.
#[derive(Debug, Clone)]
pub struct ChatSession {
    pub session_id: Option<String>,
    pub repository_path: Option<String>,
    pub custom_title: Option<String>,
    pub created_at_ms: Option<i64>,
    pub model: ChatModel,
    pub requests: Vec<ChatRequest>,
}

/// The model selected for the session, including any embedded pricing.
#[derive(Debug, Clone, Default)]
pub struct ChatModel {
    pub id: Option<String>,
    pub name: Option<String>,
    pub input_per_m: Option<f64>,
    pub output_per_m: Option<f64>,
    pub cache_per_m: Option<f64>,
}

/// A single request/response turn within a chat session.
#[derive(Debug, Clone, Default)]
pub struct ChatRequest {
    pub request_id: Option<String>,
    pub timestamp_ms: Option<i64>,
    /// Prompt/input token count for the turn, when VS Code records it.
    pub prompt_tokens: Option<u64>,
    /// Final output (completion) token count for the turn.
    pub completion_tokens: u64,
    /// Credits reported by VS Code in `result.details`, when available.
    pub reported_credits: Option<f64>,
    pub elapsed_ms: Option<i64>,
    pub first_progress_ms: Option<i64>,
    pub total_elapsed_ms: Option<i64>,
    pub tools: Vec<String>,
    pub subagents: Vec<String>,
    pub skills: Vec<String>,
    pub terminal_commands: Vec<String>,
    pub activity_events: Vec<ActivityEvent>,
    pub had_reasoning: bool,
}

/// Heuristic: does `first_line` look like a VS Code chat journal header?
pub fn looks_like_chat_log(first_line: &str) -> bool {
    let Ok(value) = serde_json::from_str::<Value>(first_line) else {
        return false;
    };
    let f = &log_fields().chat;
    value.get(&f.journal.kind).and_then(Value::as_u64) == Some(0)
        && (value.pointer(&f.header.session_id).is_some()
            || value.pointer(&f.header.requests).is_some())
}

/// Parse a chat log from its full text contents.
pub fn parse_str(data: &str) -> Result<ChatSession> {
    let f = &log_fields().chat;
    let mut root = Value::Object(Map::new());
    for (line_no, raw) in data.lines().enumerate() {
        let line = raw.trim();
        if line.is_empty() {
            continue;
        }
        let record: Value = serde_json::from_str(line)
            .with_context(|| format!("invalid JSON on line {}", line_no + 1))?;
        let kind = record
            .get(&f.journal.kind)
            .and_then(Value::as_u64)
            .unwrap_or(1);
        let value = record.get(&f.journal.value).cloned().unwrap_or(Value::Null);
        match kind {
            0 => root = value,
            _ => {
                let path = record
                    .get(&f.journal.path)
                    .and_then(Value::as_array)
                    .cloned()
                    .unwrap_or_default();
                apply_mutation(&mut root, &path, value, kind);
            }
        }
    }
    Ok(extract(&root))
}

/// Navigate to (creating as needed) the slot at `path`, then set or extend it.
fn apply_mutation(root: &mut Value, path: &[Value], value: Value, kind: u64) {
    if path.is_empty() {
        *root = value;
        return;
    }
    let target = ensure_path(root, path);
    if kind == 2 {
        match target {
            Value::Array(existing) => {
                if let Value::Array(items) = value {
                    existing.extend(items);
                }
            }
            _ => *target = value,
        }
    } else {
        *target = value;
    }
}

/// Walk `path`, creating intermediate objects/arrays, and return the final slot.
fn ensure_path<'a>(root: &'a mut Value, path: &[Value]) -> &'a mut Value {
    let mut current = root;
    for segment in path {
        match segment {
            Value::String(key) => {
                if !current.is_object() {
                    *current = Value::Object(Map::new());
                }
                let object = current.as_object_mut().expect("ensured object");
                current = object.entry(key.clone()).or_insert(Value::Null);
            }
            Value::Number(number) => {
                let index = number.as_u64().unwrap_or(0) as usize;
                if !current.is_array() {
                    *current = Value::Array(Vec::new());
                }
                let array = current.as_array_mut().expect("ensured array");
                if array.len() <= index {
                    array.resize(index + 1, Value::Null);
                }
                current = &mut array[index];
            }
            _ => {}
        }
    }
    current
}

fn extract(root: &Value) -> ChatSession {
    let f = &log_fields().chat;
    let session_id = root
        .pointer(&f.session.session_id)
        .and_then(Value::as_str)
        .map(String::from);
    let custom_title = root
        .pointer(&f.session.custom_title)
        .and_then(Value::as_str)
        .map(String::from);
    let created_at_ms = root
        .pointer(&f.session.creation_date)
        .and_then(Value::as_i64);
    let model = extract_model(root);

    let mut requests = Vec::new();
    if let Some(array) = root.pointer(&f.session.requests).and_then(Value::as_array) {
        for request in array {
            if request.is_object() {
                requests.push(extract_request(request));
            }
        }
    }

    ChatSession {
        session_id,
        repository_path: None,
        custom_title,
        created_at_ms,
        model,
        requests,
    }
}

fn extract_model(root: &Value) -> ChatModel {
    let f = &log_fields().chat;
    let metadata = root.pointer(&f.session.model_metadata);
    let get_str = |key: &str| {
        metadata
            .and_then(|m| m.get(key))
            .and_then(Value::as_str)
            .map(String::from)
    };
    let get_f64 = |key: &str| metadata.and_then(|m| m.get(key)).and_then(Value::as_f64);
    ChatModel {
        id: get_str(&f.model.id),
        name: get_str(&f.model.name),
        input_per_m: get_f64(&f.model.input_cost),
        output_per_m: get_f64(&f.model.output_cost),
        cache_per_m: get_f64(&f.model.cache_cost),
    }
}

fn extract_request(request: &Value) -> ChatRequest {
    let f = &log_fields().chat;
    let mut turn = ChatRequest {
        request_id: request
            .get(&f.request.request_id)
            .and_then(Value::as_str)
            .map(String::from),
        timestamp_ms: request.get(&f.request.timestamp).and_then(Value::as_i64),
        prompt_tokens: request
            .get(&f.request.prompt_tokens)
            .and_then(Value::as_u64)
            .or_else(|| {
                request
                    .pointer(&f.request.prompt_tokens_metadata)
                    .and_then(Value::as_u64)
            }),
        completion_tokens: request
            .get(&f.request.completion_tokens)
            .and_then(Value::as_u64)
            .unwrap_or(0),
        elapsed_ms: request.get(&f.request.elapsed_ms).and_then(Value::as_i64),
        first_progress_ms: request
            .pointer(&f.request.first_progress)
            .and_then(Value::as_i64),
        total_elapsed_ms: request
            .pointer(&f.request.total_elapsed)
            .and_then(Value::as_i64),
        reported_credits: request
            .pointer(&f.request.details)
            .and_then(Value::as_str)
            .and_then(parse_reported_credits),
        ..ChatRequest::default()
    };

    if let Some(response) = request.get(&f.request.response).and_then(Value::as_array) {
        for item in response {
            match item.get(&f.response.kind).and_then(Value::as_str) {
                Some(kind) if kind == f.response.tool_invocation_kind => {
                    collect_tool(item, &mut turn)
                }
                Some(kind) if kind == f.response.thinking_kind => turn.had_reasoning = true,
                _ => {}
            }
        }
    }

    turn
}

fn parse_reported_credits(details: &str) -> Option<f64> {
    let lower = details.to_ascii_lowercase();
    let credits_index = lower.find("credits")?;
    let before = details[..credits_index].trim_end();
    let start = before
        .rfind(|ch: char| !(ch.is_ascii_digit() || ch == '.'))
        .map_or(0, |index| index + 1);
    let number = before[start..].trim();
    if number.is_empty() {
        return None;
    }
    number.parse().ok()
}

fn collect_tool(item: &Value, turn: &mut ChatRequest) {
    let f = &log_fields().chat;
    let file_details = extract_invocation_uris(item, f);
    if let Some(tool_id) = item.get(&f.response.tool_id).and_then(Value::as_str) {
        turn.tools.push(tool_id.to_string());
        push_activity_event(turn, "tool", tool_id, file_details);
    }
    match item
        .pointer(&f.response.tool_specific_kind)
        .and_then(Value::as_str)
    {
        Some(kind) if kind == f.response.subagent_kind => {
            if let Some(name) = item.pointer(&f.response.agent_name).and_then(Value::as_str) {
                turn.subagents.push(name.to_string());
                push_activity_event(turn, "agent", name, vec![]);
            }
        }
        Some(kind) if kind == f.response.terminal_kind => {
            if let Some(command) = item
                .pointer(&f.response.command_original)
                .and_then(Value::as_str)
            {
                turn.terminal_commands.push(command.to_string());
                push_activity_event(turn, "cmd", command, vec![]);
            }
        }
        _ => {}
    }
    if let Some(skill) = detect_skill(item) {
        push_activity_event(turn, "skill", &skill, vec![]);
        turn.skills.push(skill);
    }
}

fn push_activity_event(turn: &mut ChatRequest, kind: &str, name: &str, details: Vec<String>) {
    turn.activity_events.push(ActivityEvent {
        kind: kind.to_string(),
        name: name.to_string(),
        details,
    });
}

/// Extract `file:` URIs from a tool invocation's `invocationMessage.uris` map.
///
/// The URIs are the **keys** of the `uris` object; the `file://` scheme prefix is
/// stripped so paths are shown relative to the filesystem root.
fn extract_invocation_uris(item: &Value, f: &ChatFields) -> Vec<String> {
    let Some(message) = item.get(&f.response.invocation_message) else {
        return Vec::new();
    };
    let uris_obj = match message {
        Value::Object(obj) => obj.get(&f.response.message_uris).and_then(Value::as_object),
        _ => None,
    };
    let Some(uris) = uris_obj else {
        return Vec::new();
    };
    uris.keys()
        .map(|u| u.strip_prefix("file://").unwrap_or(u).to_string())
        .collect()
}

/// Detect a `SKILL.md` read and return the skill folder name, if any.
fn detect_skill(item: &Value) -> Option<String> {
    let f = &log_fields().chat;
    let message = item.get(&f.response.invocation_message);
    let mut candidates: Vec<String> = Vec::new();
    match message {
        Some(Value::String(text)) => candidates.push(text.clone()),
        Some(Value::Object(object)) => {
            if let Some(text) = object
                .get(&f.response.message_value)
                .and_then(Value::as_str)
            {
                candidates.push(text.to_string());
            }
            if let Some(uris) = object
                .get(&f.response.message_uris)
                .and_then(Value::as_object)
            {
                candidates.extend(uris.keys().cloned());
            }
        }
        _ => {}
    }
    candidates.iter().find_map(|c| skill_name_from_path(c))
}

fn skill_name_from_path(path: &str) -> Option<String> {
    let f = &log_fields().chat;
    if !path.contains(&f.skill.marker_file) {
        return None;
    }
    let marker = &f.skill.path_segment;
    let start = path.find(marker.as_str())? + marker.len();
    let rest = &path[start..];
    let end = rest.find('/')?;
    Some(rest[..end].to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn detects_chat_header() {
        assert!(looks_like_chat_log(
            r#"{"kind":0,"v":{"sessionId":"x","requests":[]}}"#
        ));
        assert!(!looks_like_chat_log(r#"{"type":"event","data":{}}"#));
        assert!(!looks_like_chat_log("not json"));
    }

    #[test]
    fn applies_set_and_extend_mutations() {
        let data = concat!(
            r#"{"kind":0,"v":{"sessionId":"s","requests":[]}}"#,
            "\n",
            r#"{"kind":2,"k":["requests"],"v":[{"requestId":"r0"}]}"#,
            "\n",
            r#"{"kind":1,"k":["requests",0,"completionTokens"],"v":42}"#,
            "\n",
            r#"{"kind":1,"k":["requests",0,"completionTokens"],"v":123}"#,
            "\n",
        );
        let session = parse_str(data).unwrap();
        assert_eq!(session.session_id.as_deref(), Some("s"));
        assert_eq!(session.requests.len(), 1);
        // Last write wins for completion tokens.
        assert_eq!(session.requests[0].completion_tokens, 123);
    }

    #[test]
    fn extracts_skill_name() {
        assert_eq!(
            skill_name_from_path("file:///x/skills/demo-skill/SKILL.md"),
            Some("demo-skill".to_string())
        );
        assert_eq!(skill_name_from_path("file:///x/other/file.rs"), None);
    }
}
