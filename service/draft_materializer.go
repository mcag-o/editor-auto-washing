package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
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

	draft := domain.NewArticleDraft(domain.DraftString(finalOutput["template"]))
	draft.ID = workspaceID
	draft.Meta["title"] = domain.DraftString(finalOutput["title"])
	draft.Headline["title"] = domain.DraftString(finalOutput["title"])
	draft.Headline["body"] = domain.DraftParagraphs(finalOutput["body"])

	if err := m.drafts.Create(ctx, draft); err != nil {
		return nil, err
	}

	if m.workspaces != nil {
		if err := m.workspaces.TransitionStatus(ctx, workspaceID, domain.ArticleWorkspaceStatusDraft, draftMaterializedNote); err != nil {
			return nil, err
		}
	}

	return draft, nil
}
