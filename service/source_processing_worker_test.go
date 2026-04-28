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
	lastDoc   *domain.SourceDocument
	err       error
}

func (r *stubSourceProcessingRenderRunner) Render(_ context.Context, workspaceArticleID, draftID string, doc *domain.SourceDocument) error {
	r.called = true
	r.lastWork = workspaceArticleID
	r.lastDraft = draftID
	r.lastDoc = cloneSourceDocument(doc)
	return r.err
}

type stubSourceProcessingArticleIntake struct {
	called        bool
	lastWorkspace string
	lastArticle   domain.IntakeArticle
	result        *ArticleIntakeResult
	err           error
}

func (s *stubSourceProcessingArticleIntake) IntakeResult(_ context.Context, article domain.IntakeArticle) (*ArticleIntakeResult, error) {
	s.called = true
	s.lastArticle = article
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func (s *stubSourceProcessingArticleIntake) IntakeResultIntoWorkspace(_ context.Context, workspaceArticleID string, article domain.IntakeArticle) (*ArticleIntakeResult, error) {
	s.called = true
	s.lastWorkspace = workspaceArticleID
	s.lastArticle = article
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func TestSourceProcessingWorkerStopsAfterRender(t *testing.T) {
	doc := domain.NewSourceDocument("article.md", "/inbox/article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusClaimed
	now := time.Now().UTC()
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	doc.Metadata = map[string]any{
		"target_type":             "wechat-longform",
		"source_profile":          "sspai",
		"render_platform":         "wechat",
		"rewrite_profile_version": "latest",
	}
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
	require.NotEqual(t, render.lastWork, render.lastDraft)
	require.Equal(t, domain.SourceDocumentStatusProcessing, rewrite.gotDoc.Status)
	require.Equal(t, domain.SourceDocumentStatusProcessing, render.lastDoc.Status)
	require.Len(t, repo.updated, 2)
	require.Equal(t, domain.SourceDocumentStatusProcessing, repo.updated[0].Status)
	require.NotNil(t, repo.updated[0].ProcessingStartedAt)
	require.Equal(t, domain.SourceDocumentStatusCompleted, repo.updated[1].Status)
	require.Equal(t, "workspace-1", repo.updated[1].WorkspaceArticleID)
	require.NotEmpty(t, repo.updated[1].RewriteRunID)
	require.Equal(t, "rewrite-1", repo.updated[1].RewriteRunID)
	require.NotNil(t, repo.updated[1].CompletedAt)
	require.Empty(t, repo.updated[1].ErrorSummary)
}

func TestSourceProcessingWorkerFailsWhenTargetTypeMissing(t *testing.T) {
	doc := validClaimedSourceProcessingDocument()
	delete(doc.Metadata, "target_type")
	rewrite := &stubSourceProcessingRewriteRunner{}
	render := &stubSourceProcessingRenderRunner{}
	repo := &stubSourceProcessingRepo{stored: cloneSourceDocument(doc)}
	worker := NewSourceProcessingWorker(repo, rewrite, render)

	err := worker.Process(t.Context(), doc)

	require.Error(t, err)
	require.ErrorContains(t, err, "target_type")
	require.False(t, rewrite.called)
	require.False(t, render.called)
	require.Empty(t, repo.updated)
}

func TestSourceProcessingWorkerFailsWhenSourceProfileMissing(t *testing.T) {
	doc := validClaimedSourceProcessingDocument()
	delete(doc.Metadata, "source_profile")
	rewrite := &stubSourceProcessingRewriteRunner{}
	render := &stubSourceProcessingRenderRunner{}
	repo := &stubSourceProcessingRepo{stored: cloneSourceDocument(doc)}
	worker := NewSourceProcessingWorker(repo, rewrite, render)

	err := worker.Process(t.Context(), doc)

	require.Error(t, err)
	require.ErrorContains(t, err, "source_profile")
	require.False(t, rewrite.called)
	require.False(t, render.called)
	require.Empty(t, repo.updated)
}

func TestSourceProcessingWorkerFailsWhenRenderPlatformMissing(t *testing.T) {
	doc := validClaimedSourceProcessingDocument()
	delete(doc.Metadata, "render_platform")
	rewrite := &stubSourceProcessingRewriteRunner{}
	render := &stubSourceProcessingRenderRunner{}
	repo := &stubSourceProcessingRepo{stored: cloneSourceDocument(doc)}
	worker := NewSourceProcessingWorker(repo, rewrite, render)

	err := worker.Process(t.Context(), doc)

	require.Error(t, err)
	require.ErrorContains(t, err, "render_platform")
	require.False(t, rewrite.called)
	require.False(t, render.called)
	require.Empty(t, repo.updated)
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
	doc := validClaimedSourceProcessingDocument()
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
	doc := validClaimedSourceProcessingDocument()
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

func TestArticleIntakeSourceProcessingRewriteRunnerReturnsExplicitRewriteOutputs(t *testing.T) {
	doc := validClaimedSourceProcessingDocument()
	doc.WorkspaceArticleID = "workspace-existing"
	intake := &stubSourceProcessingArticleIntake{result: &ArticleIntakeResult{
		WorkspaceArticle: &domain.ArticleWorkspaceRecord{ID: "workspace-existing"},
		RewriteRun:       &domain.RewritePipelineRun{ID: "rewrite-real", FinalDraftID: "draft-real"},
		DraftID:          "draft-real",
	}}
	runner := NewArticleIntakeSourceProcessingRewriteRunner(intake)

	result, err := runner.Run(t.Context(), doc)

	require.NoError(t, err)
	require.True(t, intake.called)
	require.Equal(t, "workspace-existing", intake.lastWorkspace)
	require.Equal(t, "workspace-existing", result.WorkspaceArticleID)
	require.Equal(t, "rewrite-real", result.RewriteRunID)
	require.Equal(t, "draft-real", result.DraftID)
	require.NotEqual(t, result.WorkspaceArticleID, result.DraftID)
}

func TestArticleIntakeSourceProcessingRewriteRunnerRejectsMissingExplicitDraftID(t *testing.T) {
	doc := validClaimedSourceProcessingDocument()
	intake := &stubSourceProcessingArticleIntake{result: &ArticleIntakeResult{
		WorkspaceArticle: &domain.ArticleWorkspaceRecord{ID: "workspace-1"},
		RewriteRun:       &domain.RewritePipelineRun{ID: "rewrite-real", FinalDraftID: ""},
	}}
	runner := NewArticleIntakeSourceProcessingRewriteRunner(intake)

	result, err := runner.Run(t.Context(), doc)

	require.Nil(t, result)
	require.Error(t, err)
	require.ErrorContains(t, err, "draft id")
}

func validClaimedSourceProcessingDocument() *domain.SourceDocument {
	doc := domain.NewSourceDocument("article.md", "/inbox/article.md", "md", "Title", "Body", "hash-1")
	doc.Status = domain.SourceDocumentStatusClaimed
	now := time.Now().UTC()
	doc.ClaimedBy = "worker-1"
	doc.ClaimedAt = &now
	doc.Metadata = map[string]any{
		"target_type":             "wechat-longform",
		"source_profile":          "sspai",
		"render_platform":         "wechat",
		"rewrite_profile_version": "latest",
	}
	return doc
}
