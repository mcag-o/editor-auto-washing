package sqlite

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

var _ repo.PublishRepo = (*publishRepo)(nil)
var _ repo.JobRepo = (*jobRepo)(nil)
var _ repo.JobEventRepo = (*jobEventRepo)(nil)

type publishRepo struct{ db *sql.DB }
type jobRepo struct{ db *sql.DB }
type jobEventRepo struct{ db *sql.DB }

func (r *publishRepo) Record(ctx context.Context, record *domain.PublishRecord) error {
	metadata, err := json.Marshal(record.Metadata)
	if err != nil {
		return fmt.Errorf("marshal publish metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO publish_records (id, article_id, article_title, review_id, asset_id, platform, success, message, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, record.ID, record.ArticleID, record.ArticleTitle, record.ReviewID, record.AssetID, record.Platform, boolToInt(record.Success), record.Message, string(metadata), record.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert publish record: %w", err)
	}
	return nil
}

func (r *publishRepo) ListByArticle(ctx context.Context, articleID string) ([]domain.PublishRecord, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, article_id, article_title, review_id, asset_id, platform, success, message, metadata, created_at FROM publish_records WHERE article_id = ? ORDER BY created_at DESC`, articleID)
	if err != nil {
		return nil, fmt.Errorf("query publish records: %w", err)
	}
	defer rows.Close()
	var records []domain.PublishRecord
	for rows.Next() {
		record, scanErr := scanPublishRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, *record)
	}
	return records, rows.Err()
}

func (r *publishRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM publish_records WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete publish record: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete publish result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("publish_record", id)
	}
	return nil
}

func (r *jobRepo) Create(ctx context.Context, job *domain.JobRun) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO jobs (id, topic, status, artifact_path, result, started_at, completed_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, job.ID, job.Topic, job.Status, nullableString(job.ArtifactPath), nullableString(job.Result), nullableTime(job.StartedAt), nullableTime(job.CompletedAt), job.CreatedAt.Format(time.RFC3339), job.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert job: %w", err)
	}
	return nil
}

func (r *jobRepo) GetByID(ctx context.Context, id string) (*domain.JobRun, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, topic, status, artifact_path, result, started_at, completed_at, created_at, updated_at FROM jobs WHERE id = ?`, id)
	job, err := scanJob(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("job", id)
		}
		return nil, err
	}
	return job, nil
}

func (r *jobRepo) List(ctx context.Context, status *string) ([]domain.JobRun, error) {
	query := `SELECT id, topic, status, artifact_path, result, started_at, completed_at, created_at, updated_at FROM jobs`
	args := []any{}
	if status != nil && *status != "" {
		query += ` WHERE status = ?`
		args = append(args, *status)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()
	var jobs []domain.JobRun
	for rows.Next() {
		job, scanErr := scanJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		jobs = append(jobs, *job)
	}
	return jobs, rows.Err()
}

func (r *jobRepo) Update(ctx context.Context, id string, fn func(*domain.JobRun)) error {
	job, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	fn(job)
	job.UpdatedAt = time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `UPDATE jobs SET topic = ?, status = ?, artifact_path = ?, result = ?, started_at = ?, completed_at = ?, updated_at = ? WHERE id = ?`, job.Topic, job.Status, nullableString(job.ArtifactPath), nullableString(job.Result), nullableTime(job.StartedAt), nullableTime(job.CompletedAt), job.UpdatedAt.Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	return nil
}

func (r *jobRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM job_events WHERE job_id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete job events: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete job result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("job", id)
	}
	return nil
}

func (r *jobEventRepo) Add(ctx context.Context, evt *domain.JobEvent) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO job_events (id, job_id, event_order, status, message, detail, created_at) VALUES (?, ?, COALESCE((SELECT MAX(event_order) + 1 FROM job_events WHERE job_id = ?), 1), ?, ?, ?, ?)`, evt.ID, evt.JobID, evt.JobID, evt.Status, evt.Message, evt.Detail, evt.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("insert job event: %w", err)
	}
	return nil
}

func (r *jobEventRepo) ListByJob(ctx context.Context, jobID string) ([]domain.JobEvent, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, job_id, status, message, detail, created_at FROM job_events WHERE job_id = ? ORDER BY event_order ASC`, jobID)
	if err != nil {
		return nil, fmt.Errorf("query job events: %w", err)
	}
	defer rows.Close()
	var events []domain.JobEvent
	for rows.Next() {
		evt, scanErr := scanJobEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		events = append(events, *evt)
	}
	return events, rows.Err()
}

type runtimeScanner interface{ Scan(dest ...any) error }

func scanPublishRecord(row runtimeScanner) (*domain.PublishRecord, error) {
	var record domain.PublishRecord
	var metadata string
	var createdAt string
	var success int
	if err := row.Scan(&record.ID, &record.ArticleID, &record.ArticleTitle, &record.ReviewID, &record.AssetID, &record.Platform, &success, &record.Message, &metadata, &createdAt); err != nil {
		return nil, err
	}
	record.Success = success == 1
	if err := json.Unmarshal([]byte(metadata), &record.Metadata); err != nil {
		return nil, fmt.Errorf("decode publish metadata: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode publish created_at: %w", err)
	}
	record.CreatedAt = parsed
	return &record, nil
}

func scanJob(row runtimeScanner) (*domain.JobRun, error) {
	var job domain.JobRun
	var artifactPath, result, startedAt, completedAt sql.NullString
	var createdAt, updatedAt string
	if err := row.Scan(&job.ID, &job.Topic, &job.Status, &artifactPath, &result, &startedAt, &completedAt, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if artifactPath.Valid {
		job.ArtifactPath = &artifactPath.String
	}
	if result.Valid {
		job.Result = &result.String
	}
	if startedAt.Valid {
		parsed, err := time.Parse(time.RFC3339, startedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode job started_at: %w", err)
		}
		job.StartedAt = &parsed
	}
	if completedAt.Valid {
		parsed, err := time.Parse(time.RFC3339, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("decode job completed_at: %w", err)
		}
		job.CompletedAt = &parsed
	}
	created, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode job created_at: %w", err)
	}
	updated, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode job updated_at: %w", err)
	}
	job.CreatedAt = created
	job.UpdatedAt = updated
	return &job, nil
}

func scanJobEvent(row runtimeScanner) (*domain.JobEvent, error) {
	var evt domain.JobEvent
	var createdAt string
	if err := row.Scan(&evt.ID, &evt.JobID, &evt.Status, &evt.Message, &evt.Detail, &createdAt); err != nil {
		return nil, err
	}
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("decode job event created_at: %w", err)
		}
	}
	evt.CreatedAt = parsed
	return &evt, nil
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(time.RFC3339)
}
