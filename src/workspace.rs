//! Sidecar workspace metadata readers for session logs.

use std::collections::HashMap;
use std::path::{Path, PathBuf};

use anyhow::{Context, Result};
use serde_json::Value;

#[derive(Debug, Clone, Copy)]
pub enum WorkspaceFormat {
    Vscode,
    Cli,
}

pub fn resolve_repository_path(log_path: &Path, format: WorkspaceFormat) -> Result<Option<String>> {
    match format {
        WorkspaceFormat::Vscode => read_vscode_workspace(log_path),
        WorkspaceFormat::Cli => read_cli_workspace(log_path),
    }
}

fn read_vscode_workspace(log_path: &Path) -> Result<Option<String>> {
    let Some(sidecar) = existing_file(vscode_sidecar_path(log_path)) else {
        return Ok(None);
    };

    let data = std::fs::read_to_string(&sidecar)
        .with_context(|| format!("reading {}", sidecar.display()))?;
    let root: Value =
        serde_json::from_str(&data).with_context(|| format!("parsing {}", sidecar.display()))?;
    let Some(folder) = root.get("folder").and_then(Value::as_str) else {
        return Ok(None);
    };

    Ok(normalize_workspace_path(folder))
}

fn read_cli_workspace(log_path: &Path) -> Result<Option<String>> {
    let sidecar = log_path.parent().map(|dir| dir.join("workspace.yaml"));
    let Some(sidecar) = existing_file(sidecar) else {
        return Ok(None);
    };

    let data = std::fs::read_to_string(&sidecar)
        .with_context(|| format!("reading {}", sidecar.display()))?;
    let fields = parse_simple_yaml(&data);
    Ok(fields
        .get("git_root")
        .cloned()
        .or_else(|| fields.get("cwd").cloned()))
}

fn vscode_sidecar_path(log_path: &Path) -> Option<PathBuf> {
    let parent = log_path.parent()?;
    if parent.file_name().and_then(|name| name.to_str()) == Some("chatSessions") {
        return parent.parent().map(|dir| dir.join("workspace.json"));
    }
    Some(parent.join("workspace.json"))
}

fn existing_file(path: Option<PathBuf>) -> Option<PathBuf> {
    let path = path?;
    path.is_file().then_some(path)
}

fn normalize_workspace_path(path: &str) -> Option<String> {
    if let Some(rest) = path.strip_prefix("file://") {
        return percent_decode(rest).or_else(|| Some(rest.to_string()));
    }
    Some(path.to_string())
}

fn percent_decode(value: &str) -> Option<String> {
    let bytes = value.as_bytes();
    let mut decoded = Vec::with_capacity(bytes.len());
    let mut index = 0;

    while index < bytes.len() {
        if bytes[index] == b'%' {
            if index + 2 >= bytes.len() {
                return None;
            }
            let high = from_hex(bytes[index + 1])?;
            let low = from_hex(bytes[index + 2])?;
            decoded.push((high << 4) | low);
            index += 3;
        } else {
            decoded.push(bytes[index]);
            index += 1;
        }
    }

    String::from_utf8(decoded).ok()
}

fn from_hex(value: u8) -> Option<u8> {
    match value {
        b'0'..=b'9' => Some(value - b'0'),
        b'a'..=b'f' => Some(value - b'a' + 10),
        b'A'..=b'F' => Some(value - b'A' + 10),
        _ => None,
    }
}

fn parse_simple_yaml(data: &str) -> HashMap<String, String> {
    let mut values = HashMap::new();

    for raw in data.lines() {
        let line = raw.trim();
        if line.is_empty() || line.starts_with('#') || raw.starts_with(' ') || raw.starts_with('\t')
        {
            continue;
        }
        let Some((key, value)) = line.split_once(':') else {
            continue;
        };
        values.insert(key.trim().to_string(), unquote_yaml_scalar(value.trim()));
    }

    values
}

fn unquote_yaml_scalar(value: &str) -> String {
    if value.len() >= 2
        && ((value.starts_with('"') && value.ends_with('"'))
            || (value.starts_with('\'') && value.ends_with('\'')))
    {
        return value[1..value.len() - 1].to_string();
    }
    value.to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn decodes_vscode_file_uri() {
        assert_eq!(
            normalize_workspace_path("file:///tmp/a%20b"),
            Some("/tmp/a b".to_string())
        );
    }

    #[test]
    fn parses_cli_scalar_values() {
        let parsed = parse_simple_yaml("git_root: '/tmp/root'\ncwd: \"/tmp/cwd\"\n");
        assert_eq!(
            parsed.get("git_root").map(String::as_str),
            Some("/tmp/root")
        );
        assert_eq!(parsed.get("cwd").map(String::as_str), Some("/tmp/cwd"));
    }
}
