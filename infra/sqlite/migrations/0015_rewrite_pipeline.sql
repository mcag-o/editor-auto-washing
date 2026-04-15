CREATE TABLE IF NOT EXISTS rewrite_pipeline_profiles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    target_type TEXT NOT NULL,
    source_profile TEXT NOT NULL,
    version TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    stages_json TEXT NOT NULL DEFAULT '[]',
    default_llm_profile TEXT NOT NULL DEFAULT '',
    quality_policy_ref TEXT NOT NULL DEFAULT '',
    materialization_policy TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    UNIQUE(target_type, source_profile, version)
);

CREATE INDEX IF NOT EXISTS idx_rewrite_pipeline_profiles_lookup ON rewrite_pipeline_profiles(target_type, source_profile, version);

CREATE TABLE IF NOT EXISTS rewrite_pipeline_runs (
    id TEXT PRIMARY KEY,
    profile_id TEXT NOT NULL,
    profile_version TEXT NOT NULL,
    workspace_article_id TEXT NOT NULL,
    collector_article_id TEXT NOT NULL,
    target_type TEXT NOT NULL,
    source_profile TEXT NOT NULL,
    status TEXT NOT NULL,
    current_stage TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL,
    completed_at TEXT,
    final_draft_id TEXT NOT NULL DEFAULT '',
    error_summary TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_rewrite_pipeline_runs_started_at ON rewrite_pipeline_runs(started_at);

CREATE TABLE IF NOT EXISTS rewrite_stage_runs (
    id TEXT PRIMARY KEY,
    pipeline_run_id TEXT NOT NULL,
    stage_name TEXT NOT NULL,
    stage_type TEXT NOT NULL,
    prompt_ref TEXT NOT NULL DEFAULT '',
    llm_profile_ref TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 1,
    input_json TEXT NOT NULL DEFAULT '',
    output_json TEXT NOT NULL DEFAULT '',
    error_summary TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    started_at TEXT NOT NULL,
    completed_at TEXT,
    FOREIGN KEY(pipeline_run_id) REFERENCES rewrite_pipeline_runs(id)
);

CREATE INDEX IF NOT EXISTS idx_rewrite_stage_runs_pipeline_run_id ON rewrite_stage_runs(pipeline_run_id);

CREATE TABLE IF NOT EXISTS prompt_templates (
    key TEXT NOT NULL,
    version TEXT NOT NULL,
    system_template TEXT NOT NULL DEFAULT '',
    user_template TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY(key, version)
);

CREATE INDEX IF NOT EXISTS idx_prompt_templates_key ON prompt_templates(key);

CREATE TABLE IF NOT EXISTS llm_profiles (
    name TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    api_key_ref TEXT NOT NULL DEFAULT '',
    base_url_ref TEXT NOT NULL DEFAULT '',
    temperature REAL NOT NULL DEFAULT 0,
    max_tokens INTEGER NOT NULL DEFAULT 0,
    timeout_sec INTEGER NOT NULL DEFAULT 0,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
