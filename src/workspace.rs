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

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WorkspaceContext {
    pub repository_path: Option<String>,
    pub dependencies: Vec<WorkspaceDependency>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WorkspaceDependency {
    pub kind: String,
    pub path: PathBuf,
}

pub fn resolve_repository_path(log_path: &Path, format: WorkspaceFormat) -> Result<Option<String>> {
    Ok(resolve_repository_context(log_path, format)?.repository_path)
}

pub fn resolve_repository_context(
    log_path: &Path,
    format: WorkspaceFormat,
) -> Result<WorkspaceContext> {
    match format {
        WorkspaceFormat::Vscode => read_vscode_workspace(log_path),
        WorkspaceFormat::Cli => read_cli_workspace(log_path),
    }
}

fn read_vscode_workspace(log_path: &Path) -> Result<WorkspaceContext> {
    let mut dependencies = Vec::new();
    let Some(sidecar) = existing_file(vscode_sidecar_path(log_path)) else {
        return Ok(WorkspaceContext {
            repository_path: None,
            dependencies,
        });
    };
    dependencies.push(WorkspaceDependency {
        kind: "vscode_workspace_json".to_string(),
        path: sidecar.clone(),
    });

    let data = std::fs::read_to_string(&sidecar)
        .with_context(|| format!("reading {}", sidecar.display()))?;
    let root: Value =
        serde_json::from_str(&data).with_context(|| format!("parsing {}", sidecar.display()))?;
    if let Some(folder) = root.get("folder").and_then(Value::as_str) {
        return Ok(WorkspaceContext {
            repository_path: normalize_workspace_path(folder),
            dependencies,
        });
    }

    let Some(workspace) = root.get("workspace").and_then(Value::as_str) else {
        return Ok(WorkspaceContext {
            repository_path: None,
            dependencies,
        });
    };
    let repository_path = read_vscode_workspace_file(workspace, &mut dependencies)?;
    Ok(WorkspaceContext {
        repository_path,
        dependencies,
    })
}

fn read_vscode_workspace_file(
    workspace: &str,
    dependencies: &mut Vec<WorkspaceDependency>,
) -> Result<Option<String>> {
    let Some(path) = workspace_uri_to_path(workspace) else {
        return Ok(normalize_workspace_path(workspace));
    };
    let Some(path) = existing_file(Some(path)) else {
        return Ok(None);
    };
    dependencies.push(WorkspaceDependency {
        kind: "vscode_code_workspace".to_string(),
        path: path.clone(),
    });

    let data =
        std::fs::read_to_string(&path).with_context(|| format!("reading {}", path.display()))?;
    let root: Value =
        serde_json::from_str(&data).with_context(|| format!("parsing {}", path.display()))?;
    let Some(folders) = root.get("folders").and_then(Value::as_array) else {
        return Ok(None);
    };

    folders
        .iter()
        .filter_map(|folder| folder.get("path").and_then(Value::as_str))
        .find_map(normalize_workspace_folder)
        .map(Some)
        .map(Ok)
        .unwrap_or(Ok(None))
}

fn normalize_workspace_folder(path: &str) -> Option<String> {
    normalize_workspace_path(path).and_then(|value| {
        let trimmed = value.trim();
        (!trimmed.is_empty()).then(|| trimmed.to_string())
    })
}

fn workspace_uri_to_path(uri: &str) -> Option<PathBuf> {
    let rest = uri.strip_prefix("file://")?;
    percent_decode(rest).map(PathBuf::from)
}

fn read_cli_workspace(log_path: &Path) -> Result<WorkspaceContext> {
    let mut dependencies = Vec::new();
    let sidecar = log_path.parent().map(|dir| dir.join("workspace.yaml"));
    let Some(sidecar) = existing_file(sidecar) else {
        return Ok(WorkspaceContext {
            repository_path: None,
            dependencies,
        });
    };
    dependencies.push(WorkspaceDependency {
        kind: "cli_workspace_yaml".to_string(),
        path: sidecar.clone(),
    });

    let data = std::fs::read_to_string(&sidecar)
        .with_context(|| format!("reading {}", sidecar.display()))?;
    let fields = parse_simple_yaml(&data);
    Ok(WorkspaceContext {
        repository_path: fields
            .get("git_root")
            .cloned()
            .or_else(|| fields.get("cwd").cloned()),
        dependencies,
    })
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

    #[test]
    fn resolves_vscode_workspace_uri_to_first_folder() {
        let root = std::env::temp_dir().join(format!(
            "vibe-watch-workspace-{}",
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .expect("time")
                .as_nanos()
        ));
        let chat_sessions = root.join("storage/chatSessions");
        let workspace_file = root.join("Workspaces/test.code-workspace");
        std::fs::create_dir_all(&chat_sessions).expect("chat dir");
        std::fs::create_dir_all(workspace_file.parent().expect("workspace parent"))
            .expect("workspace dir");
        let workspace_uri = format!("file://{}", workspace_file.display());
        std::fs::write(
            root.join("storage/workspace.json"),
            format!(r#"{{"workspace":"{}"}}"#, workspace_uri),
        )
        .expect("sidecar");
        std::fs::write(
            &workspace_file,
            r#"{"folders":[{"path":"file:///tmp/vibe-watch-workspace-uri"}]}"#,
        )
        .expect("workspace file");
        let log_path = chat_sessions.join("session.jsonl");
        std::fs::write(&log_path, "").expect("log");

        assert_eq!(
            resolve_repository_path(&log_path, WorkspaceFormat::Vscode).expect("resolve"),
            Some("/tmp/vibe-watch-workspace-uri".to_string())
        );

        std::fs::remove_dir_all(root).ok();
    }
}
