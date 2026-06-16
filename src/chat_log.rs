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

/// A reconstructed VS Code Copilot Chat session.
#[derive(Debug, Clone)]
pub struct ChatSession {
    pub session_id: Option<String>,
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
    /// Final output (completion) token count for the turn.
    pub completion_tokens: u64,
    pub elapsed_ms: Option<i64>,
    pub first_progress_ms: Option<i64>,
    pub total_elapsed_ms: Option<i64>,
    pub tools: Vec<String>,
    pub subagents: Vec<String>,
    pub skills: Vec<String>,
    pub terminal_commands: Vec<String>,
    pub had_reasoning: bool,
}

/// Heuristic: does `first_line` look like a VS Code chat journal header?
pub fn looks_like_chat_log(first_line: &str) -> bool {
    let Ok(value) = serde_json::from_str::<Value>(first_line) else {
        return false;
    };
    value.get("kind").and_then(Value::as_u64) == Some(0)
        && (value.pointer("/v/sessionId").is_some() || value.pointer("/v/requests").is_some())
}

/// Parse a chat log from its full text contents.
pub fn parse_str(data: &str) -> Result<ChatSession> {
    let mut root = Value::Object(Map::new());
    for (line_no, raw) in data.lines().enumerate() {
        let line = raw.trim();
        if line.is_empty() {
            continue;
        }
        let record: Value = serde_json::from_str(line)
            .with_context(|| format!("invalid JSON on line {}", line_no + 1))?;
        let kind = record.get("kind").and_then(Value::as_u64).unwrap_or(1);
        let value = record.get("v").cloned().unwrap_or(Value::Null);
        match kind {
            0 => root = value,
            _ => {
                let path = record
                    .get("k")
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
    let session_id = root
        .pointer("/sessionId")
        .and_then(Value::as_str)
        .map(String::from);
    let custom_title = root
        .pointer("/customTitle")
        .and_then(Value::as_str)
        .map(String::from);
    let created_at_ms = root.pointer("/creationDate").and_then(Value::as_i64);
    let model = extract_model(root);

    let mut requests = Vec::new();
    if let Some(array) = root.pointer("/requests").and_then(Value::as_array) {
        for request in array {
            if request.is_object() {
                requests.push(extract_request(request));
            }
        }
    }

    ChatSession {
        session_id,
        custom_title,
        created_at_ms,
        model,
        requests,
    }
}

fn extract_model(root: &Value) -> ChatModel {
    let metadata = root.pointer("/inputState/selectedModel/metadata");
    let get_str = |key: &str| {
        metadata
            .and_then(|m| m.get(key))
            .and_then(Value::as_str)
            .map(String::from)
    };
    let get_f64 = |key: &str| metadata.and_then(|m| m.get(key)).and_then(Value::as_f64);
    ChatModel {
        id: get_str("id"),
        name: get_str("name"),
        input_per_m: get_f64("inputCost"),
        output_per_m: get_f64("outputCost"),
        cache_per_m: get_f64("cacheCost"),
    }
}

fn extract_request(request: &Value) -> ChatRequest {
    let mut turn = ChatRequest {
        request_id: request
            .get("requestId")
            .and_then(Value::as_str)
            .map(String::from),
        timestamp_ms: request.get("timestamp").and_then(Value::as_i64),
        completion_tokens: request
            .get("completionTokens")
            .and_then(Value::as_u64)
            .unwrap_or(0),
        elapsed_ms: request.get("elapsedMs").and_then(Value::as_i64),
        first_progress_ms: request
            .pointer("/result/timings/firstProgress")
            .and_then(Value::as_i64),
        total_elapsed_ms: request
            .pointer("/result/timings/totalElapsed")
            .and_then(Value::as_i64),
        ..ChatRequest::default()
    };

    if let Some(response) = request.get("response").and_then(Value::as_array) {
        for item in response {
            match item.get("kind").and_then(Value::as_str) {
                Some("toolInvocationSerialized") => collect_tool(item, &mut turn),
                Some("thinking") => turn.had_reasoning = true,
                _ => {}
            }
        }
    }

    turn
}

fn collect_tool(item: &Value, turn: &mut ChatRequest) {
    if let Some(tool_id) = item.get("toolId").and_then(Value::as_str) {
        turn.tools.push(tool_id.to_string());
    }
    match item
        .pointer("/toolSpecificData/kind")
        .and_then(Value::as_str)
    {
        Some("subagent") => {
            if let Some(name) = item
                .pointer("/toolSpecificData/agentName")
                .and_then(Value::as_str)
            {
                turn.subagents.push(name.to_string());
            }
        }
        Some("terminal") => {
            if let Some(command) = item
                .pointer("/toolSpecificData/commandLine/original")
                .and_then(Value::as_str)
            {
                turn.terminal_commands.push(command.to_string());
            }
        }
        _ => {}
    }
    if let Some(skill) = detect_skill(item) {
        turn.skills.push(skill);
    }
}

/// Detect a `SKILL.md` read and return the skill folder name, if any.
fn detect_skill(item: &Value) -> Option<String> {
    let message = item.get("invocationMessage");
    let mut candidates: Vec<String> = Vec::new();
    match message {
        Some(Value::String(text)) => candidates.push(text.clone()),
        Some(Value::Object(object)) => {
            if let Some(text) = object.get("value").and_then(Value::as_str) {
                candidates.push(text.to_string());
            }
            if let Some(uris) = object.get("uris").and_then(Value::as_object) {
                candidates.extend(uris.keys().cloned());
            }
        }
        _ => {}
    }
    candidates.iter().find_map(|c| skill_name_from_path(c))
}

fn skill_name_from_path(path: &str) -> Option<String> {
    if !path.contains("SKILL.md") {
        return None;
    }
    let marker = "/skills/";
    let start = path.find(marker)? + marker.len();
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
