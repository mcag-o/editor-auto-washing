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

var _ repo.DraftRepo = (*draftRepo)(nil)
var _ repo.AssetRepo = (*assetRepo)(nil)

type draftRepo struct {
	db *sql.DB
}

type assetRepo struct {
	db *sql.DB
}

func (r *draftRepo) Create(ctx context.Context, draft *domain.ArticleDraft) error {
	meta, err := json.Marshal(draft.Meta)
	if err != nil {
		return fmt.Errorf("marshal draft meta: %w", err)
	}
	headline, err := json.Marshal(draft.Headline)
	if err != nil {
		return fmt.Errorf("marshal draft headline: %w", err)
	}
	sections, err := json.Marshal(draft.Sections)
	if err != nil {
		return fmt.Errorf("marshal draft sections: %w", err)
	}
	sourceRefs, err := json.Marshal(draft.SourceRefs)
	if err != nil {
		return fmt.Errorf("marshal draft source refs: %w", err)
	}
	targetPlatforms, err := json.Marshal(draft.TargetPlatforms)
	if err != nil {
		return fmt.Errorf("marshal draft target platforms: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO article_drafts (id, template, meta, headline, sections, conclusion, cta, source_refs, target_platforms, provider_profile, article_profile, publish_profile, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		draft.ID, draft.Template, string(meta), string(headline), string(sections), draft.Conclusion, draft.CTA, string(sourceRefs), string(targetPlatforms), draft.ProviderProfile, draft.ArticleProfile, draft.PublishProfile, draft.Status, draft.CreatedAt.Format(time.RFC3339), draft.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert draft: %w", err)
	}
	return nil
}

func (r *draftRepo) GetByID(ctx context.Context, id string) (*domain.ArticleDraft, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, template, meta, headline, sections, conclusion, cta, source_refs, target_platforms, provider_profile, article_profile, publish_profile, status, created_at, updated_at FROM article_drafts WHERE id = ?`, id)
	draft, err := scanDraft(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("draft", id)
		}
		return nil, err
	}
	return draft, nil
}

func (r *draftRepo) List(ctx context.Context, status *string) ([]domain.ArticleDraft, error) {
	query := `SELECT id, template, meta, headline, sections, conclusion, cta, source_refs, target_platforms, provider_profile, article_profile, publish_profile, status, created_at, updated_at FROM article_drafts`
	args := []any{}
	if status != nil && *status != "" {
		query += ` WHERE status = ?`
		args = append(args, *status)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query drafts: %w", err)
	}
	defer rows.Close()
	var drafts []domain.ArticleDraft
	for rows.Next() {
		draft, err := scanDraft(rows)
		if err != nil {
			return nil, err
		}
		drafts = append(drafts, *draft)
	}
	return drafts, rows.Err()
}

func (r *draftRepo) Update(ctx context.Context, id string, fn func(*domain.ArticleDraft)) error {
	draft, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	fn(draft)
	draft.UpdatedAt = time.Now().UTC()
	meta, err := json.Marshal(draft.Meta)
	if err != nil {
		return fmt.Errorf("marshal draft meta: %w", err)
	}
	headline, err := json.Marshal(draft.Headline)
	if err != nil {
		return fmt.Errorf("marshal draft headline: %w", err)
	}
	sections, err := json.Marshal(draft.Sections)
	if err != nil {
		return fmt.Errorf("marshal draft sections: %w", err)
	}
	sourceRefs, err := json.Marshal(draft.SourceRefs)
	if err != nil {
		return fmt.Errorf("marshal draft source refs: %w", err)
	}
	targetPlatforms, err := json.Marshal(draft.TargetPlatforms)
	if err != nil {
		return fmt.Errorf("marshal draft target platforms: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `UPDATE article_drafts SET template = ?, meta = ?, headline = ?, sections = ?, conclusion = ?, cta = ?, source_refs = ?, target_platforms = ?, provider_profile = ?, article_profile = ?, publish_profile = ?, status = ?, updated_at = ? WHERE id = ?`,
		draft.Template, string(meta), string(headline), string(sections), draft.Conclusion, draft.CTA, string(sourceRefs), string(targetPlatforms), draft.ProviderProfile, draft.ArticleProfile, draft.PublishProfile, draft.Status, draft.UpdatedAt.Format(time.RFC3339), id,
	)
	if err != nil {
		return fmt.Errorf("update draft: %w", err)
	}
	return nil
}

func (r *assetRepo) Create(ctx context.Context, asset *domain.RenderedAssetRecord) error {
	metadata, err := json.Marshal(asset.Metadata)
	if err != nil {
		return fmt.Errorf("marshal asset metadata: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO rendered_assets (id, article_id, platform, status, asset_type, content, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		asset.AssetID, asset.ArticleID, asset.Platform, asset.Status, asset.OutputFormat, asset.Content, string(metadata), asset.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert asset: %w", err)
	}
	return nil
}

func (r *assetRepo) GetByID(ctx context.Context, id string) (*domain.RenderedAssetRecord, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, article_id, platform, status, asset_type, content, metadata, created_at FROM rendered_assets WHERE id = ?`, id)
	asset, err := scanAsset(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("asset", id)
		}
		return nil, err
	}
	return asset, nil
}

func (r *assetRepo) List(ctx context.Context, articleID, platform string) ([]domain.RenderedAssetRecord, error) {
	query := `SELECT id, article_id, platform, status, asset_type, content, metadata, created_at FROM rendered_assets WHERE 1=1`
	args := []any{}
	if articleID != "" {
		query += ` AND article_id = ?`
		args = append(args, articleID)
	}
	if platform != "" {
		query += ` AND platform = ?`
		args = append(args, platform)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query assets: %w", err)
	}
	defer rows.Close()
	var assets []domain.RenderedAssetRecord
	for rows.Next() {
		asset, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, *asset)
	}
	return assets, rows.Err()
}

func (r *assetRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM rendered_assets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete asset: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete asset result: %w", err)
	}
	if rows == 0 {
		return domain.NewNotFoundErr("asset", id)
	}
	return nil
}

type formattingScanner interface {
	Scan(dest ...any) error
}

func scanDraft(row formattingScanner) (*domain.ArticleDraft, error) {
	var draft domain.ArticleDraft
	var meta string
	var headline string
	var sections string
	var sourceRefs string
	var targetPlatforms string
	var createdAt string
	var updatedAt string
	if err := row.Scan(&draft.ID, &draft.Template, &meta, &headline, &sections, &draft.Conclusion, &draft.CTA, &sourceRefs, &targetPlatforms, &draft.ProviderProfile, &draft.ArticleProfile, &draft.PublishProfile, &draft.Status, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(meta), &draft.Meta); err != nil {
		return nil, fmt.Errorf("decode draft meta: %w", err)
	}
	if err := json.Unmarshal([]byte(headline), &draft.Headline); err != nil {
		return nil, fmt.Errorf("decode draft headline: %w", err)
	}
	if err := json.Unmarshal([]byte(sections), &draft.Sections); err != nil {
		return nil, fmt.Errorf("decode draft sections: %w", err)
	}
	if err := json.Unmarshal([]byte(sourceRefs), &draft.SourceRefs); err != nil {
		return nil, fmt.Errorf("decode draft source refs: %w", err)
	}
	if err := json.Unmarshal([]byte(targetPlatforms), &draft.TargetPlatforms); err != nil {
		return nil, fmt.Errorf("decode draft target platforms: %w", err)
	}
	var err error
	draft.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode draft created_at: %w", err)
	}
	draft.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode draft updated_at: %w", err)
	}
	if draft.Meta == nil {
		draft.Meta = map[string]any{}
	}
	if draft.Headline == nil {
		draft.Headline = map[string]any{}
	}
	return &draft, nil
}

func scanAsset(row formattingScanner) (*domain.RenderedAssetRecord, error) {
	var asset domain.RenderedAssetRecord
	var metadata string
	var createdAt string
	if err := row.Scan(&asset.AssetID, &asset.ArticleID, &asset.Platform, &asset.Status, &asset.OutputFormat, &asset.Content, &metadata, &createdAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(metadata), &asset.Metadata); err != nil {
		return nil, fmt.Errorf("decode asset metadata: %w", err)
	}
	if asset.Metadata == nil {
		asset.Metadata = map[string]any{}
	}
	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("decode asset created_at: %w", err)
	}
	asset.CreatedAt = parsedCreatedAt
	if artifactPath, ok := asset.Metadata["artifact_path"].(string); ok {
		asset.ArtifactPath = artifactPath
	}
	if templateName, ok := asset.Metadata["template"].(string); ok {
		asset.Template = templateName
	}
	return &asset, nil
}
