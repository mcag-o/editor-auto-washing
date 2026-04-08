-- Articles
CREATE TABLE IF NOT EXISTS articles (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    format TEXT NOT NULL DEFAULT 'markdown',
    summary TEXT NOT NULL DEFAULT '',
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_articles_title ON articles(title);

-- Templates
CREATE TABLE IF NOT EXISTS templates (
    id TEXT PRIMARY KEY,
    category TEXT NOT NULL,
    name TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_templates_category ON templates(category);
CREATE INDEX IF NOT EXISTS idx_templates_name ON templates(name);

-- Article Drafts
CREATE TABLE IF NOT EXISTS article_drafts (
    id TEXT PRIMARY KEY,
    template TEXT NOT NULL,
    meta TEXT NOT NULL DEFAULT '{}',
    headline TEXT NOT NULL DEFAULT '{}',
    sections TEXT NOT NULL DEFAULT '[]',
    conclusion TEXT NOT NULL DEFAULT '',
    cta TEXT NOT NULL DEFAULT '',
    source_refs TEXT NOT NULL DEFAULT '[]',
    target_platforms TEXT NOT NULL DEFAULT '[]',
    provider_profile TEXT NOT NULL DEFAULT '',
    article_profile TEXT NOT NULL DEFAULT '',
    publish_profile TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_drafts_status ON article_drafts(status);

-- Rendered Assets
CREATE TABLE IF NOT EXISTS rendered_assets (
    id TEXT PRIMARY KEY,
    article_id TEXT NOT NULL,
    platform TEXT NOT NULL,
    asset_type TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_assets_article_id ON rendered_assets(article_id);
CREATE INDEX IF NOT EXISTS idx_assets_platform ON rendered_assets(platform);

-- Review Tasks
CREATE TABLE IF NOT EXISTS review_tasks (
    id TEXT PRIMARY KEY,
    article_id TEXT NOT NULL,
    asset_ids TEXT NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'review_pending',
    publish_profile TEXT NOT NULL DEFAULT '',
    reviewer TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_reviews_article_id ON review_tasks(article_id);
CREATE INDEX IF NOT EXISTS idx_reviews_status ON review_tasks(status);

-- Publish Records
CREATE TABLE IF NOT EXISTS publish_records (
    id TEXT PRIMARY KEY,
    article_title TEXT NOT NULL,
    platform TEXT NOT NULL,
    success INTEGER NOT NULL DEFAULT 0,
    message TEXT NOT NULL DEFAULT '',
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_publish_article_title ON publish_records(article_title);
CREATE INDEX IF NOT EXISTS idx_publish_platform ON publish_records(platform);

-- Jobs
CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    topic TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    artifact_path TEXT,
    result TEXT,
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_topic ON jobs(topic);

-- Job Events
CREATE TABLE IF NOT EXISTS job_events (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL,
    status TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    detail TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_job_events_job_id ON job_events(job_id);

-- Ingestions
CREATE TABLE IF NOT EXISTS ingestions (
    id TEXT PRIMARY KEY,
    source_type TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ingestions_source_type ON ingestions(source_type);
CREATE INDEX IF NOT EXISTS idx_ingestions_status ON ingestions(status);

-- Workspace Articles
CREATE TABLE IF NOT EXISTS workspace_articles (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    status_history TEXT NOT NULL DEFAULT '[]',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_workspace_status ON workspace_articles(status);
