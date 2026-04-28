package service

import (
	"content-hub/domain"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubSourceProcessingRepo struct {
	stored     *domain.SourceDocument
	updated    []*domain.SourceDocument
	getErr     error
	updateErr  error
	updateCall int
}

func (r *stubSourceProcessingRepo) Create(context.Context, *domain.SourceDocument) error {
	return nil
}

func (r *stubSourceProcessingRepo) Update(_ context.Context, doc *domain.SourceDocument) error {
	r.updateCall++
	if r.updateErr != nil {
		return r.updateErr
	}
	copyValue := cloneSourceDocument(doc)
	r.updated = append(r.updated, copyValue)
	r.stored = copyValue
	return nil
}

func (r *stubSourceProcessingRepo) GetByID(_ context.Context, id string) (*domain.SourceDocument, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.stored == nil || r.stored.ID != id {
		return nil, domain.NewNotFoundErr("source_document", id)
	}
	return cloneSourceDocument(r.stored), nil
}

func (r *stubSourceProcessingRepo) FindByHash(context.Context, string) (*domain.SourceDocument, error) {
	return nil, domain.NewNotFoundErr("source_document_hash", "missing")
}

func (r *stubSourceProcessingRepo) ClaimPending(context.Context, int, string, time.Time) ([]domain.SourceDocument, error) {
	return nil, nil
}

func (r *stubSourceProcessingRepo) ListByStatus(context.Context, string, int) ([]domain.SourceDocument, error) {
	return nil, nil
}

type stubSourceProcessingRewriteRunner struct {
	called bool
	gotDoc *domain.SourceDocument
	result *SourceProcessingRewriteResult
	err    error
}

func (r *stubSourceProcessingRewriteRunner) Run(_ context.Context, doc *domain.SourceDocument) (*SourceProcessingRewriteResult, error) {
	r.called = true
	r.gotDoc = cloneSourceDocument(doc)
	if r.err != nil {
		return nil, r.err
	}
	return r.result, nil
}

type stubSourceProcessingRenderRunner struct {
	called    bool
	lastDraft string
	lastWork  string
	err       error
}

func (r *stubSourceProcessingRenderRunner) Render(_ context.Context, workspaceArticleID, draftID string) error {
	r.called = true
	r.lastWork = workspaceArticleID
	r.lastDraft = draftID
	return r.err
}

func TestSourceProcessingWorkerRunsRewriteAndRender(t *testing.T) {
	doc := domain.NewSourceDocument("article.md", "/inbox/article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusClaimed
	now := time.Now().UTC()
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	rewrite := &stubSourceProcessingRewriteRunner{result: &SourceProcessingRewriteResult{
		WorkspaceArticleID: "workspace-1",
		DraftID:            "draft-1",
		RewriteRunID:       "rewrite-1",
	}}
	render := &stubSourceProcessingRenderRunner{}
	repo := &stubSourceProcessingRepo{stored: cloneSourceDocument(doc)}
	worker := NewSourceProcessingWorker(repo, rewrite, render)

	require.NoError(t, worker.Process(t.Context(), doc))
	require.True(t, rewrite.called)
	require.True(t, render.called)
	require.Equal(t, "workspace-1", render.lastWork)
	require.Equal(t, "draft-1", render.lastDraft)
	require.Len(t, repo.updated, 2)
	require.Equal(t, domain.SourceDocumentStatusProcessing, repo.updated[0].Status)
	require.NotNil(t, repo.updated[0].ProcessingStartedAt)
	require.Equal(t, domain.SourceDocumentStatusCompleted, repo.updated[1].Status)
	require.Equal(t, "workspace-1", repo.updated[1].WorkspaceArticleID)
	require.Equal(t, "rewrite-1", repo.updated[1].RewriteRunID)
	require.NotNil(t, repo.updated[1].CompletedAt)
}

func TestSourceProcessingWorkerRejectsNonClaimedDocument(t *testing.T) {
	doc := domain.NewSourceDocument("article.md", "/inbox/article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusPending
	rewrite := &stubSourceProcessingRewriteRunner{}
	render := &stubSourceProcessingRenderRunner{}
	repo := &stubSourceProcessingRepo{stored: cloneSourceDocument(doc)}
	worker := NewSourceProcessingWorker(repo, rewrite, render)

	err := worker.Process(t.Context(), doc)

	require.Error(t, err)
	require.ErrorContains(t, err, "claimed")
	require.False(t, rewrite.called)
	require.False(t, render.called)
	require.Empty(t, repo.updated)
}

func TestSourceProcessingWorkerMarksFailedWhenRewriteFails(t *testing.T) {
	doc := domain.NewSourceDocument("article.md", "/inbox/article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusClaimed
	now := time.Now().UTC()
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	rewrite := &stubSourceProcessingRewriteRunner{err: errors.New("rewrite exploded")}
	render := &stubSourceProcessingRenderRunner{}
	repo := &stubSourceProcessingRepo{stored: cloneSourceDocument(doc)}
	worker := NewSourceProcessingWorker(repo, rewrite, render)

	err := worker.Process(t.Context(), doc)

	require.Error(t, err)
	require.ErrorContains(t, err, "rewrite exploded")
	require.True(t, rewrite.called)
	require.False(t, render.called)
	require.Len(t, repo.updated, 2)
	require.Equal(t, domain.SourceDocumentStatusProcessing, repo.updated[0].Status)
	require.Equal(t, domain.SourceDocumentStatusFailed, repo.updated[1].Status)
	require.Equal(t, "rewrite exploded", repo.updated[1].ErrorSummary)
}

func TestSourceProcessingWorkerMarksFailedWhenRenderFails(t *testing.T) {
	doc := domain.NewSourceDocument("article.md", "/inbox/article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusClaimed
	now := time.Now().UTC()
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	rewrite := &stubSourceProcessingRewriteRunner{result: &SourceProcessingRewriteResult{
		WorkspaceArticleID: "workspace-1",
		DraftID:            "draft-1",
		RewriteRunID:       "rewrite-1",
	}}
	render := &stubSourceProcessingRenderRunner{err: errors.New("render exploded")}
	repo := &stubSourceProcessingRepo{stored: cloneSourceDocument(doc)}
	worker := NewSourceProcessingWorker(repo, rewrite, render)

	err := worker.Process(t.Context(), doc)

	require.Error(t, err)
	require.ErrorContains(t, err, "render exploded")
	require.True(t, rewrite.called)
	require.True(t, render.called)
	require.Len(t, repo.updated, 2)
	require.Equal(t, domain.SourceDocumentStatusFailed, repo.updated[1].Status)
	require.Equal(t, "workspace-1", repo.updated[1].WorkspaceArticleID)
	require.Equal(t, "rewrite-1", repo.updated[1].RewriteRunID)
	require.Equal(t, "render exploded", repo.updated[1].ErrorSummary)
}
