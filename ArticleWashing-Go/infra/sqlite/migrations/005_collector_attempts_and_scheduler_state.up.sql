CREATE TABLE IF NOT EXISTS collector_attempts (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    source_run_id TEXT NOT NULL,
    entry_id TEXT,
    article_id TEXT,
    stage TEXT NOT NULL,
    attempt_number INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'pending',
    request_url TEXT NOT NULL DEFAULT '',
    request_method TEXT NOT NULL DEFAULT 'GET',
    response_status_code INTEGER NOT NULL DEFAULT 0,
    error_code TEXT,
    error_message TEXT,
    raw_json TEXT NOT NULL DEFAULT '{}',
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY(run_id) REFERENCES collector_runs(id),
    FOREIGN KEY(source_run_id) REFERENCES collector_source_runs(id),
    FOREIGN KEY(entry_id) REFERENCES collector_entries(id),
    FOREIGN KEY(article_id) REFERENCES collector_articles(id)
);

CREATE INDEX IF NOT EXISTS idx_collector_attempts_source_run_id ON collector_attempts(source_run_id);
CREATE INDEX IF NOT EXISTS idx_collector_attempts_entry_id ON collector_attempts(entry_id);

CREATE TABLE IF NOT EXISTS collector_scheduler_state (
    name TEXT PRIMARY KEY,
    status TEXT NOT NULL DEFAULT 'idle',
    last_run_id TEXT,
    last_heartbeat TEXT,
    last_run_at TEXT,
    next_run_at TEXT,
    error_message TEXT,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    updated_at TEXT NOT NULL
);
