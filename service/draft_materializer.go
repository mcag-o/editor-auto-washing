package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"fmt"
	"strings"
)

const draftMaterializedNote = "rewrite draft materialized"

type DraftMaterializer struct {
	drafts     repo.DraftRepo
	workspaces repo.WorkspaceRepo
}

func NewDraftMaterializer(drafts repo.DraftRepo, workspaces repo.WorkspaceRepo) *DraftMaterializer {
	return &DraftMaterializer{drafts: drafts, workspaces: workspaces}
}

func (m *DraftMaterializer) Materialize(ctx context.Context, workspaceID string, finalOutput map[string]any) (*domain.ArticleDraft, error) {
	if m.drafts == nil || m.workspaces == nil {
		return nil, domain.NewInternalErr("draft materializer is not configured", nil)
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, domain.NewValidationErr("workspace id is required", nil)
	}

	template := domain.DraftString(finalOutput["template"])
	if template == "" {
		return nil, domain.NewValidationErr("final output template is required", nil)
	}
	title := domain.DraftString(finalOutput["title"])
	if title == "" {
		return nil, domain.NewValidationErr("final output title is required", nil)
	}
	body := domain.DraftParagraphs(finalOutput["body"])
	if len(body) == 0 {
		return nil, domain.NewValidationErr("final output body is required", nil)
	}

	if existing, err := m.drafts.GetByID(ctx, workspaceID); err == nil && existing != nil {
		if err := m.workspaces.TransitionStatus(ctx, workspaceID, domain.ArticleWorkspaceStatusDraft, draftMaterializedNote); err != nil {
			appErr, ok := err.(*domain.AppError)
			if !ok || appErr.Code != domain.ErrConflict || !workspaceAlreadyDraft(ctx, m.workspaces, workspaceID) {
				return nil, fmt.Errorf("draft already persisted but workspace transition failed: %w", err)
			}
		}
		return existing, nil
	} else {
		appErr, ok := err.(*domain.AppError)
		if err != nil && (!ok || appErr.Code != domain.ErrNotFound) {
			return nil, err
		}
	}

	draft := domain.NewArticleDraft(template)
	draft.ID = workspaceID
	draft.Meta["title"] = title
	for key, value := range draftMaterializerMap(finalOutput["meta"]) {
		draft.Meta[key] = value
	}
	draft.Headline["title"] = title
	draft.Headline["body"] = body
	if sections, ok := finalOutput["sections"].([]any); ok {
		draft.Sections = append([]any(nil), sections...)
	}
	draft.Conclusion = domain.DraftString(finalOutput["conclusion"])
	draft.CTA = domain.DraftString(finalOutput["cta"])
	if sourceRefs, ok := finalOutput["source_refs"].([]any); ok {
		draft.SourceRefs = append([]any(nil), sourceRefs...)
	}

	if err := m.drafts.Create(ctx, draft); err != nil {
		return nil, err
	}

	if err := m.workspaces.TransitionStatus(ctx, workspaceID, domain.ArticleWorkspaceStatusDraft, draftMaterializedNote); err != nil {
		return nil, fmt.Errorf("draft persisted but workspace transition failed: %w", err)
	}

	return draft, nil
}

func draftMaterializerMap(value any) map[string]any {
	metadata, ok := value.(map[string]any)
	if !ok || len(metadata) == 0 {
		return nil
	}
	return metadata
}

func workspaceAlreadyDraft(ctx context.Context, workspaces repo.WorkspaceRepo, workspaceID string) bool {
	if workspaces == nil {
		return false
	}
	workspace, err := workspaces.GetByID(ctx, workspaceID)
	if err != nil || workspace == nil {
		return false
	}
	return strings.TrimSpace(workspace.Status) == domain.ArticleWorkspaceStatusDraft
}
