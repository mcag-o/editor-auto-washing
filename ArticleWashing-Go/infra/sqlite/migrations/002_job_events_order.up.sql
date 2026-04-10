ALTER TABLE job_events ADD COLUMN event_order INTEGER NOT NULL DEFAULT 0;

UPDATE job_events
SET event_order = (
    SELECT COUNT(*)
    FROM job_events AS earlier
    WHERE earlier.job_id = job_events.job_id
      AND (
        earlier.created_at < job_events.created_at
        OR (earlier.created_at = job_events.created_at AND earlier.id <= job_events.id)
      )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_job_events_job_id_event_order ON job_events(job_id, event_order);
