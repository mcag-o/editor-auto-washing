CREATE TABLE IF NOT EXISTS workflow_runs (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL,
    workflow_version TEXT NOT NULL,
    workspace_article_id TEXT NOT NULL,
    status TEXT NOT NULL,
    current_node_id TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL,
    completed_at TEXT,
    error_summary TEXT NOT NULL DEFAULT '',
    final_failure_class TEXT NOT NULL DEFAULT '',
    resumable INTEGER NOT NULL DEFAULT 0,
    resume_from_checkpoint_id TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_workflow_runs_started_at ON workflow_runs(started_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS workflow_checkpoints (
    id TEXT PRIMARY KEY,
    workflow_run_id TEXT NOT NULL,
    node_execution_id TEXT NOT NULL DEFAULT '',
    node_id TEXT NOT NULL,
    state TEXT NOT NULL,
    resumable INTEGER NOT NULL DEFAULT 0,
    resume_token TEXT NOT NULL DEFAULT '',
    payload_json TEXT NOT NULL DEFAULT '',
    failure_class TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    consumed_at TEXT,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    FOREIGN KEY(workflow_run_id) REFERENCES workflow_runs(id)
);

CREATE INDEX IF NOT EXISTS idx_workflow_checkpoints_workflow_run_id ON workflow_checkpoints(workflow_run_id, created_at ASC, id ASC);
