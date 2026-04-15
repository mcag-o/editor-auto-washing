package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"fmt"
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
	if m.drafts == nil {
		return nil, domain.NewInternalErr("draft materializer is not configured", nil)
	}
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

	draft := domain.NewArticleDraft(template)
	draft.ID = workspaceID
	draft.Meta["title"] = title
	draft.Headline["title"] = title
	draft.Headline["body"] = body

	if err := m.drafts.Create(ctx, draft); err != nil {
		return nil, err
	}

	if m.workspaces != nil {
		if err := m.workspaces.TransitionStatus(ctx, workspaceID, domain.ArticleWorkspaceStatusDraft, draftMaterializedNote); err != nil {
			return nil, fmt.Errorf("draft persisted but workspace transition failed: %w", err)
		}
	}

	return draft, nil
}
