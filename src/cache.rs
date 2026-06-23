//! Local SQLite cache for derived session analytics.

use std::collections::hash_map::DefaultHasher;
use std::fs;
use std::hash::{Hash, Hasher};
use std::path::{Path, PathBuf};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use anyhow::{Context, Result};
use rusqlite::{params, Connection, OptionalExtension};

use crate::analytics::SessionAnalytics;
use crate::session_scan::DetectedFormat;

pub const CACHE_SCHEMA_VERSION: i64 = 1;
// v2: ActivityEvent gained a `details` field; old cached rows have details: []
// Bump this string whenever the parsed analytics schema changes in a way that
// would cause stale cached data to be visually incorrect.
pub const CACHE_COMPAT_VERSION: &str = "session-cache-v2";
const PRICING_CONFIG_JSON: &str = include_str!("../config/model_pricing.json");
const LOG_FIELDS_CONFIG_JSON: &str = include_str!("../config/log_fields.json");

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CacheConfig {
    pub enabled: bool,
    pub refresh: bool,
    pub db_path: Option<PathBuf>,
}

impl CacheConfig {
    pub fn enabled_default() -> Self {
        Self {
            enabled: true,
            refresh: false,
            db_path: None,
        }
    }

    pub fn disabled() -> Self {
        Self {
            enabled: false,
            refresh: false,
            db_path: None,
        }
    }

    pub fn enabled_at(db_path: PathBuf) -> Self {
        Self {
            enabled: true,
            refresh: false,
            db_path: Some(db_path),
        }
    }

    pub fn with_refresh(mut self, refresh: bool) -> Self {
        self.refresh = refresh;
        self
    }

    pub fn with_db_path(mut self, db_path: Option<PathBuf>) -> Self {
        self.db_path = db_path;
        self
    }
}

impl Default for CacheConfig {
    fn default() -> Self {
        Self::enabled_default()
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SourceMetadata {
    pub path: PathBuf,
    pub size_bytes: u64,
    pub modified_ms: i64,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DependencyMetadata {
    pub kind: String,
    pub path: PathBuf,
    pub size_bytes: u64,
    pub modified_ms: i64,
    pub fingerprint: String,
}

pub struct SessionCache {
    connection: Connection,
}

impl SessionCache {
    pub fn open(config: &CacheConfig) -> Result<Option<Self>> {
        if !config.enabled {
            return Ok(None);
        }
        let path = config
            .db_path
            .clone()
            .or_else(default_cache_path)
            .context("could not determine cache database path")?;
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)
                .with_context(|| format!("creating cache directory {}", parent.display()))?;
        }
        Self::open_path(&path).map(Some)
    }

    pub fn open_path(path: &Path) -> Result<Self> {
        let connection = Connection::open(path)
            .with_context(|| format!("opening cache database {}", path.display()))?;
        connection.busy_timeout(Duration::from_secs(2))?;
        connection.pragma_update(None, "foreign_keys", "ON")?;
        connection.pragma_update(None, "journal_mode", "WAL")?;
        connection.pragma_update(None, "user_version", CACHE_SCHEMA_VERSION)?;
        let cache = Self { connection };
        cache.initialize_schema()?;
        Ok(cache)
    }

    pub fn lookup(
        &self,
        source: &SourceMetadata,
        format: DetectedFormat,
        dependencies: &[DependencyMetadata],
    ) -> Result<Option<SessionAnalytics>> {
        let mut statement = self.connection.prepare(
            "SELECT id, analytics_json FROM sessions
             WHERE source_path = ?1
               AND format = ?2
               AND size_bytes = ?3
               AND modified_ms = ?4
               AND compat_version = ?5",
        )?;
        let compat_key = cache_compat_key();
        let row = statement
            .query_row(
                params![
                    canonical_key(&source.path),
                    format.as_str(),
                    source.size_bytes as i64,
                    source.modified_ms,
                    compat_key,
                ],
                |row| Ok((row.get::<_, i64>(0)?, row.get::<_, String>(1)?)),
            )
            .optional()?;
        let Some((session_row_id, analytics_json)) = row else {
            return Ok(None);
        };
        if !self.dependencies_match(session_row_id, dependencies)? {
            return Ok(None);
        }
        match serde_json::from_str::<SessionAnalytics>(&analytics_json) {
            Ok(analytics) => Ok(Some(analytics)),
            Err(_) => Ok(None),
        }
    }

    pub fn store(
        &mut self,
        source: &SourceMetadata,
        format: DetectedFormat,
        dependencies: &[DependencyMetadata],
        analytics: &SessionAnalytics,
    ) -> Result<()> {
        let transaction = self.connection.transaction()?;
        let source_path = canonical_key(&source.path);
        let compat_key = cache_compat_key();
        transaction.execute(
            "DELETE FROM sessions WHERE source_path = ?1 AND format = ?2",
            params![source_path, format.as_str()],
        )?;
        let analytics_json = serde_json::to_string(analytics)?;
        transaction.execute(
            "INSERT INTO sessions (
                source_path,
                format,
                size_bytes,
                modified_ms,
                compat_version,
                session_id,
                repository_path,
                title,
                model_id,
                model_name,
                turn_count,
                total_output_tokens,
                total_credits,
                analytics_json,
                scanned_at_ms
             ) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14, ?15)",
            params![
                source_path,
                format.as_str(),
                source.size_bytes as i64,
                source.modified_ms,
                compat_key,
                analytics.session_id.as_deref(),
                analytics.repository_path.as_deref(),
                analytics.title.as_deref(),
                analytics.model_id.as_deref(),
                analytics.model_name.as_deref(),
                analytics.turn_count as i64,
                analytics.total_output_tokens as i64,
                analytics.total_credits,
                analytics_json,
                now_ms(),
            ],
        )?;
        let session_row_id = transaction.last_insert_rowid();
        for dependency in dependencies {
            transaction.execute(
                "INSERT INTO source_dependencies (
                    session_row_id,
                    kind,
                    path,
                    size_bytes,
                    modified_ms,
                    fingerprint
                 ) VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
                params![
                    session_row_id,
                    dependency.kind,
                    canonical_key(&dependency.path),
                    dependency.size_bytes as i64,
                    dependency.modified_ms,
                    dependency.fingerprint,
                ],
            )?;
        }
        transaction.commit()?;
        Ok(())
    }

    fn initialize_schema(&self) -> Result<()> {
        self.connection.execute_batch(
            "CREATE TABLE IF NOT EXISTS sessions (
                id INTEGER PRIMARY KEY,
                source_path TEXT NOT NULL,
                format TEXT NOT NULL,
                size_bytes INTEGER NOT NULL,
                modified_ms INTEGER NOT NULL,
                compat_version TEXT NOT NULL,
                session_id TEXT,
                repository_path TEXT,
                title TEXT,
                model_id TEXT,
                model_name TEXT,
                turn_count INTEGER NOT NULL,
                total_output_tokens INTEGER NOT NULL,
                total_credits REAL,
                analytics_json TEXT NOT NULL,
                scanned_at_ms INTEGER NOT NULL,
                UNIQUE(source_path, format)
            );
            CREATE INDEX IF NOT EXISTS idx_sessions_repo ON sessions(repository_path);
            CREATE INDEX IF NOT EXISTS idx_sessions_source_validity
                ON sessions(source_path, format, size_bytes, modified_ms, compat_version);
            CREATE TABLE IF NOT EXISTS source_dependencies (
                id INTEGER PRIMARY KEY,
                session_row_id INTEGER NOT NULL,
                kind TEXT NOT NULL,
                path TEXT NOT NULL,
                size_bytes INTEGER NOT NULL,
                modified_ms INTEGER NOT NULL,
                fingerprint TEXT NOT NULL,
                FOREIGN KEY(session_row_id) REFERENCES sessions(id) ON DELETE CASCADE
            );
            CREATE INDEX IF NOT EXISTS idx_source_dependencies_session
                ON source_dependencies(session_row_id);",
        )?;
        Ok(())
    }

    fn dependencies_match(
        &self,
        session_row_id: i64,
        dependencies: &[DependencyMetadata],
    ) -> Result<bool> {
        let mut statement = self.connection.prepare(
            "SELECT kind, path, size_bytes, modified_ms, fingerprint
             FROM source_dependencies
             WHERE session_row_id = ?1
             ORDER BY kind, path",
        )?;
        let mut cached = statement
            .query_map(params![session_row_id], |row| {
                Ok(DependencyFingerprint {
                    kind: row.get(0)?,
                    path: row.get(1)?,
                    size_bytes: row.get(2)?,
                    modified_ms: row.get(3)?,
                    fingerprint: row.get(4)?,
                })
            })?
            .collect::<rusqlite::Result<Vec<_>>>()?;
        let mut current = dependencies
            .iter()
            .map(|dependency| DependencyFingerprint {
                kind: dependency.kind.clone(),
                path: canonical_key(&dependency.path),
                size_bytes: dependency.size_bytes as i64,
                modified_ms: dependency.modified_ms,
                fingerprint: dependency.fingerprint.clone(),
            })
            .collect::<Vec<_>>();
        cached.sort();
        current.sort();
        Ok(cached == current)
    }
}

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
struct DependencyFingerprint {
    kind: String,
    path: String,
    size_bytes: i64,
    modified_ms: i64,
    fingerprint: String,
}

pub fn source_metadata(path: &Path) -> Result<SourceMetadata> {
    let metadata =
        fs::metadata(path).with_context(|| format!("reading metadata for {}", path.display()))?;
    Ok(SourceMetadata {
        path: path.to_path_buf(),
        size_bytes: metadata.len(),
        modified_ms: modified_ms(&metadata),
    })
}

pub fn dependency_metadata(kind: impl Into<String>, path: &Path) -> Result<DependencyMetadata> {
    let data = fs::read(path).with_context(|| format!("reading dependency {}", path.display()))?;
    let metadata =
        fs::metadata(path).with_context(|| format!("reading metadata for {}", path.display()))?;
    Ok(DependencyMetadata {
        kind: kind.into(),
        path: path.to_path_buf(),
        size_bytes: metadata.len(),
        modified_ms: modified_ms(&metadata),
        fingerprint: fingerprint_bytes(&data),
    })
}

pub fn default_cache_path() -> Option<PathBuf> {
    if cfg!(target_os = "macos") {
        return home_dir().map(|home| home.join("Library/Caches/vibe-watch/sessions.sqlite"));
    }
    if cfg!(target_os = "windows") {
        return std::env::var_os("LOCALAPPDATA")
            .map(PathBuf::from)
            .map(|base| base.join("vibe-watch").join("sessions.sqlite"));
    }
    if let Some(base) = std::env::var_os("XDG_CACHE_HOME").map(PathBuf::from) {
        return Some(base.join("vibe-watch").join("sessions.sqlite"));
    }
    home_dir().map(|home| home.join(".cache/vibe-watch/sessions.sqlite"))
}

fn canonical_key(path: &Path) -> String {
    path.canonicalize()
        .unwrap_or_else(|_| path.to_path_buf())
        .to_string_lossy()
        .into_owned()
}

fn home_dir() -> Option<PathBuf> {
    std::env::var_os("HOME").map(PathBuf::from)
}

fn modified_ms(metadata: &fs::Metadata) -> i64 {
    metadata
        .modified()
        .ok()
        .and_then(|modified| modified.duration_since(UNIX_EPOCH).ok())
        .map(|duration| duration.as_millis().min(i64::MAX as u128) as i64)
        .unwrap_or(0)
}

fn now_ms() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis().min(i64::MAX as u128) as i64)
        .unwrap_or(0)
}

fn fingerprint_bytes(data: &[u8]) -> String {
    let mut hasher = DefaultHasher::new();
    data.hash(&mut hasher);
    format!("{:016x}", hasher.finish())
}

fn cache_compat_key() -> String {
    format!(
        "{}:{}:{}",
        CACHE_COMPAT_VERSION,
        fingerprint_bytes(PRICING_CONFIG_JSON.as_bytes()),
        fingerprint_bytes(LOG_FIELDS_CONFIG_JSON.as_bytes())
    )
}

impl DetectedFormat {
    pub fn as_str(self) -> &'static str {
        match self {
            DetectedFormat::Vscode => "vscode",
            DetectedFormat::Cli => "cli",
        }
    }
}
