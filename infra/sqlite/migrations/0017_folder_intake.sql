CREATE TABLE IF NOT EXISTS source_documents (
  id TEXT PRIMARY KEY,
  source_type TEXT NOT NULL,
  original_filename TEXT NOT NULL,
  original_path TEXT NOT NULL,
  archived_path TEXT NOT NULL DEFAULT '',
  file_type TEXT NOT NULL,
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  summary TEXT NOT NULL DEFAULT '',
  metadata_json BLOB NOT NULL,
  hash TEXT NOT NULL,
  imported_at TEXT,
  status TEXT NOT NULL,
  workspace_article_id TEXT NOT NULL DEFAULT '',
  rewrite_run_id TEXT NOT NULL DEFAULT '',
  claimed_by TEXT NOT NULL DEFAULT '',
  claimed_at TEXT,
  processing_started_at TEXT,
  completed_at TEXT,
  error_summary TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_source_documents_hash ON source_documents(hash);
CREATE INDEX IF NOT EXISTS idx_source_documents_status ON source_documents(status, claimed_at, id);

CREATE TABLE IF NOT EXISTS import_runs (
  id TEXT PRIMARY KEY,
  source_type TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT NOT NULL,
  completed_at TEXT,
  imported_count INTEGER NOT NULL DEFAULT 0,
  failed_count INTEGER NOT NULL DEFAULT 0,
  error_summary TEXT NOT NULL DEFAULT '',
  metadata_json BLOB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_import_runs_started_at ON import_runs(started_at, id);
