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

var _ repo.ReviewRepo = (*reviewRepo)(nil)

type reviewRepo struct{ db *sql.DB }

func (r *reviewRepo) Create(ctx context.Context, review *domain.ReviewTask) error {
	assetIDs, err := json.Marshal(review.AssetIDs)
	if err != nil {
		return fmt.Errorf("marshal review asset ids: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO review_tasks (id, article_id, asset_ids, status, publish_profile, reviewer, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, review.ID, review.ArticleID, string(assetIDs), review.Status, review.PublishProfile, review.Reviewer, review.Notes, review.CreatedAt.Format(time.RFC3339), review.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert review: %w", err)
	}
	return nil
}

func (r *reviewRepo) GetByID(ctx context.Context, id string) (*domain.ReviewTask, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, article_id, asset_ids, status, publish_profile, reviewer, notes, created_at, updated_at FROM review_tasks WHERE id = ?`, id)
	review, err := scanReview(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("review", id)
		}
		return nil, err
	}
	return review, nil
}

func (r *reviewRepo) ListByArticle(ctx context.Context, articleID string) ([]domain.ReviewTask, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, article_id, asset_ids, status, publish_profile, reviewer, notes, created_at, updated_at FROM review_tasks WHERE article_id = ? ORDER BY created_at DESC`, articleID)
	if err != nil {
		return nil, fmt.Errorf("query reviews: %w", err)
	}
	defer rows.Close()
	var reviews []domain.ReviewTask
	for rows.Next() {
		review, scanErr := scanReview(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		reviews = append(reviews, *review)
	}
	return reviews, rows.Err()
}

func (r *reviewRepo) UpdateStatus(ctx context.Context, id string, status, reviewer, notes string) error {
	review, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	review.Status = status
	review.Reviewer = reviewer
	review.Notes = notes
	review.UpdatedAt = time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `UPDATE review_tasks SET status = ?, reviewer = ?, notes = ?, updated_at = ? WHERE id = ?`, review.Status, review.Reviewer, review.Notes, review.UpdatedAt.Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update review: %w", err)
	}
	return nil
}

func (r *reviewRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM review_tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete review: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete review result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("review", id)
	}
	return nil
}

type reviewScanner interface{ Scan(dest ...any) error }

func scanReview(row reviewScanner) (*domain.ReviewTask, error) {
	var review domain.ReviewTask
	var assetIDs string
	var createdAt string
	var updatedAt string
	if err := row.Scan(&review.ID, &review.ArticleID, &assetIDs, &review.Status, &review.PublishProfile, &review.Reviewer, &review.Notes, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(assetIDs), &review.AssetIDs); err != nil {
		return nil, fmt.Errorf("decode review asset ids: %w", err)
	}
	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode review created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode review updated_at: %w", err)
	}
	review.CreatedAt = parsedCreatedAt
	review.UpdatedAt = parsedUpdatedAt
	return &review, nil
}
