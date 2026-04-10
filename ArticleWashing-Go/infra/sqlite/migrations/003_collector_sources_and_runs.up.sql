CREATE TABLE IF NOT EXISTS collector_sources (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    schedule_enabled INTEGER NOT NULL DEFAULT 1,
    interval_minutes INTEGER NOT NULL DEFAULT 30,
    auth_mode TEXT NOT NULL DEFAULT 'none',
    timeout_ms INTEGER NOT NULL DEFAULT 10000,
    headers_json TEXT NOT NULL DEFAULT '{}',
    cookie_secret_ref TEXT NOT NULL DEFAULT '',
    header_secret_ref TEXT NOT NULL DEFAULT '',
    hotlist_limit INTEGER NOT NULL DEFAULT 50,
    detail_fetch_enabled INTEGER NOT NULL DEFAULT 1,
    concurrency INTEGER NOT NULL DEFAULT 1,
    retry_policy_json TEXT NOT NULL DEFAULT '{}',
    options_json TEXT NOT NULL DEFAULT '{}',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_collector_sources_enabled ON collector_sources(enabled);

CREATE TABLE IF NOT EXISTS collector_runs (
    id TEXT PRIMARY KEY,
    trigger TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    started_at TEXT,
    completed_at TEXT,
    error_code TEXT,
    error_message TEXT,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_collector_runs_status ON collector_runs(status);

CREATE TABLE IF NOT EXISTS collector_source_runs (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    source_id TEXT NOT NULL,
    stage TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    started_at TEXT,
    completed_at TEXT,
    error_code TEXT,
    error_message TEXT,
    discovered_count INTEGER NOT NULL DEFAULT 0,
    stored_count INTEGER NOT NULL DEFAULT 0,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(run_id) REFERENCES collector_runs(id),
    FOREIGN KEY(source_id) REFERENCES collector_sources(id)
);

CREATE INDEX IF NOT EXISTS idx_collector_source_runs_run_id ON collector_source_runs(run_id);
CREATE INDEX IF NOT EXISTS idx_collector_source_runs_source_id ON collector_source_runs(source_id);
