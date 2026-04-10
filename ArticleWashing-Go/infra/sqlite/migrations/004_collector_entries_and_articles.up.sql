CREATE TABLE IF NOT EXISTS collector_entries (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    source_id TEXT NOT NULL,
    external_id TEXT NOT NULL DEFAULT '',
    canonical_url TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    author TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending_detail',
    rank INTEGER,
    published_at TEXT,
    raw_json TEXT NOT NULL DEFAULT '{}',
    normalized_json TEXT NOT NULL DEFAULT '{}',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(run_id) REFERENCES collector_runs(id),
    FOREIGN KEY(source_id) REFERENCES collector_sources(id)
);

CREATE INDEX IF NOT EXISTS idx_collector_entries_run_id ON collector_entries(run_id);
CREATE INDEX IF NOT EXISTS idx_collector_entries_source_external ON collector_entries(source_id, external_id);
CREATE INDEX IF NOT EXISTS idx_collector_entries_source_url ON collector_entries(source_id, canonical_url);

CREATE TABLE IF NOT EXISTS collector_articles (
    id TEXT PRIMARY KEY,
    entry_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    source_id TEXT NOT NULL,
    external_id TEXT NOT NULL DEFAULT '',
    canonical_url TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    author TEXT NOT NULL DEFAULT '',
    bridge_status TEXT NOT NULL DEFAULT 'bridge_pending',
    workspace_id TEXT,
    published_at TEXT,
    raw_json TEXT NOT NULL DEFAULT '{}',
    normalized_json TEXT NOT NULL DEFAULT '{}',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(entry_id) REFERENCES collector_entries(id),
    FOREIGN KEY(run_id) REFERENCES collector_runs(id),
    FOREIGN KEY(source_id) REFERENCES collector_sources(id)
);

CREATE INDEX IF NOT EXISTS idx_collector_articles_entry_id ON collector_articles(entry_id);
CREATE INDEX IF NOT EXISTS idx_collector_articles_run_id ON collector_articles(run_id);
CREATE INDEX IF NOT EXISTS idx_collector_articles_bridge_status ON collector_articles(bridge_status);
