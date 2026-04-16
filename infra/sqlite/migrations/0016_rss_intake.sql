CREATE TABLE IF NOT EXISTS rss_subscriptions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    feed_url TEXT NOT NULL,
    target_type TEXT NOT NULL,
    source_profile TEXT NOT NULL,
    rewrite_profile_version TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    poll_interval_sec INTEGER NOT NULL DEFAULT 0,
    last_pulled_at TEXT,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_rss_subscriptions_feed_url ON rss_subscriptions(feed_url);
CREATE INDEX IF NOT EXISTS idx_rss_subscriptions_enabled ON rss_subscriptions(enabled);

CREATE TABLE IF NOT EXISTS rss_pull_runs (
    id TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    error_summary TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    FOREIGN KEY(subscription_id) REFERENCES rss_subscriptions(id)
);

CREATE INDEX IF NOT EXISTS idx_rss_pull_runs_subscription_id ON rss_pull_runs(subscription_id);
CREATE INDEX IF NOT EXISTS idx_rss_pull_runs_status ON rss_pull_runs(status);
CREATE INDEX IF NOT EXISTS idx_rss_pull_runs_started_at ON rss_pull_runs(started_at);

CREATE TABLE IF NOT EXISTS rss_items (
    id TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL,
    pull_run_id TEXT NOT NULL,
    guid TEXT NOT NULL DEFAULT '',
    link TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    status TEXT NOT NULL,
    published_at TEXT,
    imported_at TEXT,
    workspace_article_id TEXT,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    raw_payload_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(subscription_id) REFERENCES rss_subscriptions(id),
    FOREIGN KEY(pull_run_id) REFERENCES rss_pull_runs(id)
);

CREATE INDEX IF NOT EXISTS idx_rss_items_subscription_link ON rss_items(subscription_id, link);
CREATE INDEX IF NOT EXISTS idx_rss_items_workspace_article_id ON rss_items(workspace_article_id);
CREATE INDEX IF NOT EXISTS idx_rss_items_status ON rss_items(status);
CREATE INDEX IF NOT EXISTS idx_rss_items_subscription_guid ON rss_items(subscription_id, guid);
CREATE INDEX IF NOT EXISTS idx_rss_items_subscription_content_hash ON rss_items(subscription_id, content_hash);
CREATE INDEX IF NOT EXISTS idx_rss_items_pull_run_id ON rss_items(pull_run_id);
