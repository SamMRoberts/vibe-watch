//! Property references for the session logs vibe-watch reads.
//!
//! Every key, event-type literal, and JSON pointer that the [`chat_log`] and
//! [`cli_log`] parsers look up is defined in `config/log_fields.json` and loaded
//! here, rather than being hard-coded in the parsers. If a log format renames a
//! field, edit that JSON file -- no Rust changes are required.
//!
//! [`chat_log`]: crate::chat_log
//! [`cli_log`]: crate::cli_log

use std::sync::OnceLock;

use serde::Deserialize;

/// Field references embedded at compile time from `config/log_fields.json`.
///
/// Edit that file to track log-format changes; no Rust changes are required.
const LOG_FIELDS_JSON: &str = include_str!("../config/log_fields.json");

/// All log field references, grouped by log type.
#[derive(Debug, Clone, Deserialize)]
pub struct LogFields {
    pub chat: ChatFields,
    pub cli: CliFields,
}

/// References for the VS Code Copilot Chat `.jsonl` delta journal.
#[derive(Debug, Clone, Deserialize)]
pub struct ChatFields {
    pub journal: ChatJournalFields,
    pub header: ChatHeaderFields,
    pub session: ChatSessionFields,
    pub model: ChatModelFields,
    pub request: ChatRequestFields,
    pub response: ChatResponseFields,
    pub skill: ChatSkillFields,
}

/// Top-level keys on every chat journal line.
#[derive(Debug, Clone, Deserialize)]
pub struct ChatJournalFields {
    pub kind: String,
    pub value: String,
    pub path: String,
}

/// JSON pointers used to recognise a chat log header line.
#[derive(Debug, Clone, Deserialize)]
pub struct ChatHeaderFields {
    pub session_id: String,
    pub requests: String,
}

/// JSON pointers into the reconstructed chat session root object.
#[derive(Debug, Clone, Deserialize)]
pub struct ChatSessionFields {
    pub session_id: String,
    pub custom_title: String,
    pub creation_date: String,
    pub requests: String,
    pub model_metadata: String,
}

/// Keys within the chat model metadata object.
#[derive(Debug, Clone, Deserialize)]
pub struct ChatModelFields {
    pub id: String,
    pub name: String,
    pub input_cost: String,
    pub output_cost: String,
    pub cache_cost: String,
}

/// Keys and pointers within a single chat request object.
#[derive(Debug, Clone, Deserialize)]
pub struct ChatRequestFields {
    pub request_id: String,
    pub timestamp: String,
    pub prompt_tokens: String,
    pub completion_tokens: String,
    pub elapsed_ms: String,
    pub first_progress: String,
    pub total_elapsed: String,
    pub details: String,
    pub response: String,
}

/// Keys, literal `kind` values, and pointers within a request's response items.
#[derive(Debug, Clone, Deserialize)]
pub struct ChatResponseFields {
    pub kind: String,
    pub tool_invocation_kind: String,
    pub thinking_kind: String,
    pub tool_id: String,
    pub tool_specific_kind: String,
    pub subagent_kind: String,
    pub terminal_kind: String,
    pub agent_name: String,
    pub command_original: String,
    pub invocation_message: String,
    pub message_value: String,
    pub message_uris: String,
}

/// Markers used to detect a skill read.
#[derive(Debug, Clone, Deserialize)]
pub struct ChatSkillFields {
    pub marker_file: String,
    pub path_segment: String,
}

/// References for the Copilot CLI `events.jsonl` log.
#[derive(Debug, Clone, Deserialize)]
pub struct CliFields {
    pub event: CliEventFields,
    pub types: CliTypeFields,
    pub data: CliDataFields,
    pub model_metrics: CliModelMetricsFields,
}

/// Top-level keys on every CLI event line.
#[derive(Debug, Clone, Deserialize)]
pub struct CliEventFields {
    pub r#type: String,
    pub data: String,
    pub timestamp: String,
    pub chat_kind: String,
}

/// Literal values of the CLI event `type` field the parser handles.
#[derive(Debug, Clone, Deserialize)]
pub struct CliTypeFields {
    pub session_start: String,
    pub session_model_change: String,
    pub session_mode_changed: String,
    pub user_message: String,
    pub assistant_message: String,
    pub tool_execution_start: String,
    pub skill_invoked: String,
    pub session_shutdown: String,
}

/// Keys and pointers within a CLI event's `data` object.
#[derive(Debug, Clone, Deserialize)]
pub struct CliDataFields {
    pub session_id: String,
    pub cwd: String,
    pub new_model: String,
    pub new_mode: String,
    pub agent_mode: String,
    pub output_tokens: String,
    pub tool_name: String,
    pub agent_name: String,
    pub command: String,
    pub skill_name: String,
    pub model_metrics: String,
}

/// Keys and pointers within a single `modelMetrics` entry.
#[derive(Debug, Clone, Deserialize)]
pub struct CliModelMetricsFields {
    pub request_count: String,
    pub request_cost: String,
    pub usage_prefix: String,
    pub input_tokens: String,
    pub output_tokens: String,
    pub cache_read_tokens: String,
    pub cache_write_tokens: String,
    pub reasoning_tokens: String,
}

/// The parsed log field references, loaded once on first use.
pub fn log_fields() -> &'static LogFields {
    static FIELDS: OnceLock<LogFields> = OnceLock::new();
    FIELDS.get_or_init(|| {
        serde_json::from_str::<LogFields>(LOG_FIELDS_JSON)
            .expect("embedded config/log_fields.json must be valid JSON")
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn config_loads_and_exposes_references() {
        let fields = log_fields();
        assert_eq!(fields.chat.header.session_id, "/v/sessionId");
        assert_eq!(fields.chat.session.creation_date, "/creationDate");
        assert_eq!(fields.cli.types.session_start, "session.start");
        assert_eq!(fields.cli.data.cwd, "/context/cwd");
        assert_eq!(fields.cli.model_metrics.usage_prefix, "/usage/");
    }
}
