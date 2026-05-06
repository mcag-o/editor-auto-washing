package service

import (
	"content-hub/domain"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubSourceProcessingSchedulerRepo struct {
	claimedDocs    []domain.SourceDocument
	claimErr       error
	claimLimit     int
	claimWorker    string
	claimTime      time.Time
	claimCallCount int
	updated        []*domain.SourceDocument
	storedByID     map[string]*domain.SourceDocument
	updateErr      error
	getErr         error
}

func (r *stubSourceProcessingSchedulerRepo) Create(context.Context, *domain.SourceDocument) error {
	return nil
}

func (r *stubSourceProcessingSchedulerRepo) Update(_ context.Context, doc *domain.SourceDocument) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	copyValue := cloneSourceDocument(doc)
	r.updated = append(r.updated, copyValue)
	if r.storedByID == nil {
		r.storedByID = map[string]*domain.SourceDocument{}
	}
	r.storedByID[copyValue.ID] = copyValue
	return nil
}

func (r *stubSourceProcessingSchedulerRepo) Delete(context.Context, string) error {
	return nil
}

func (r *stubSourceProcessingSchedulerRepo) GetByID(_ context.Context, id string) (*domain.SourceDocument, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.storedByID != nil {
		if doc, ok := r.storedByID[id]; ok {
			return cloneSourceDocument(doc), nil
		}
	}
	for _, doc := range r.claimedDocs {
		if doc.ID == id {
			copyValue := cloneSourceDocument(&doc)
			if r.storedByID == nil {
				r.storedByID = map[string]*domain.SourceDocument{}
			}
			r.storedByID[id] = copyValue
			return cloneSourceDocument(copyValue), nil
		}
	}
	return nil, domain.NewNotFoundErr("source_document", id)
}

func (r *stubSourceProcessingSchedulerRepo) List(context.Context, int) ([]domain.SourceDocument, error) {
	return nil, nil
}

func (r *stubSourceProcessingSchedulerRepo) FindByHash(context.Context, string) (*domain.SourceDocument, error) {
	return nil, domain.NewNotFoundErr("source_document_hash", "missing")
}

func (r *stubSourceProcessingSchedulerRepo) ClaimPending(_ context.Context, limit int, claimedBy string, now time.Time) ([]domain.SourceDocument, error) {
	r.claimCallCount++
	r.claimLimit = limit
	r.claimWorker = claimedBy
	r.claimTime = now
	if r.claimErr != nil {
		return nil, r.claimErr
	}
	if limit > len(r.claimedDocs) {
		limit = len(r.claimedDocs)
	}
	claimed := make([]domain.SourceDocument, 0, limit)
	for i := 0; i < limit; i++ {
		doc := r.claimedDocs[i]
		claimed = append(claimed, doc)
		copyValue := cloneSourceDocument(&doc)
		if r.storedByID == nil {
			r.storedByID = map[string]*domain.SourceDocument{}
		}
		r.storedByID[doc.ID] = copyValue
	}
	return claimed, nil
}

func (r *stubSourceProcessingSchedulerRepo) ListByStatus(context.Context, string, int) ([]domain.SourceDocument, error) {
	return nil, nil
}

type stubSourceProcessingScheduleRewriteRunner struct{}

func (stubSourceProcessingScheduleRewriteRunner) Run(context.Context, *domain.SourceDocument) (*SourceProcessingRewriteResult, error) {
	return &SourceProcessingRewriteResult{
		WorkspaceArticleID: "workspace-1",
		DraftID:            "draft-1",
		RewriteRunID:       "rewrite-1",
	}, nil
}

type stubSourceProcessingScheduleRenderRunner struct{}

func (stubSourceProcessingScheduleRenderRunner) Render(context.Context, string, string, *domain.SourceDocument) error {
	return nil
}

type stubSourceProcessingSchedulerWorker struct {
	processed []*domain.SourceDocument
	errByID   map[string]error
}

func (w *stubSourceProcessingSchedulerWorker) Process(_ context.Context, doc *domain.SourceDocument) error {
	w.processed = append(w.processed, cloneSourceDocument(doc))
	if w.errByID != nil {
		if err, ok := w.errByID[doc.ID]; ok {
			return err
		}
	}
	return nil
}

func TestProcessingSchedulerClaimsUpToConcurrencyLimit(t *testing.T) {
	repo := &stubSourceProcessingSchedulerRepo{claimedDocs: []domain.SourceDocument{
		*validClaimedSourceProcessingDocument(),
		*validClaimedSourceProcessingDocument(),
		*validClaimedSourceProcessingDocument(),
	}}
	repo.claimedDocs[0].ID = "doc-1"
	repo.claimedDocs[1].ID = "doc-2"
	repo.claimedDocs[2].ID = "doc-3"
	worker := &stubSourceProcessingSchedulerWorker{}
	scheduler := NewSourceProcessingScheduler(repo, worker, 2, "scheduler-1")

	processed, err := scheduler.ProcessPending(t.Context())

	require.NoError(t, err)
	require.Len(t, processed, 2)
	require.Equal(t, 1, repo.claimCallCount)
	require.Equal(t, 2, repo.claimLimit)
	require.Equal(t, "scheduler-1", repo.claimWorker)
	require.Len(t, worker.processed, 2)
	require.Equal(t, "doc-1", worker.processed[0].ID)
	require.Equal(t, "doc-2", worker.processed[1].ID)
}

func TestProcessingSchedulerProcessesClaimedDocuments(t *testing.T) {
	repo := &stubSourceProcessingSchedulerRepo{claimedDocs: []domain.SourceDocument{
		*validClaimedSourceProcessingDocument(),
		*validClaimedSourceProcessingDocument(),
	}}
	repo.claimedDocs[0].ID = "doc-a"
	repo.claimedDocs[1].ID = "doc-b"
	worker := &stubSourceProcessingSchedulerWorker{}
	scheduler := NewSourceProcessingScheduler(repo, worker, 4, "scheduler-1")

	processed, err := scheduler.ProcessPending(t.Context())

	require.NoError(t, err)
	require.Len(t, processed, 2)
	require.Len(t, worker.processed, 2)
	require.Equal(t, []string{"doc-a", "doc-b"}, []string{worker.processed[0].ID, worker.processed[1].ID})
}

func TestProcessingSchedulerReturnsClaimError(t *testing.T) {
	repo := &stubSourceProcessingSchedulerRepo{claimErr: errors.New("claim exploded")}
	worker := &stubSourceProcessingSchedulerWorker{}
	scheduler := NewSourceProcessingScheduler(repo, worker, 1, "scheduler-1")

	processed, err := scheduler.ProcessPending(t.Context())

	require.Error(t, err)
	require.ErrorContains(t, err, "claim pending source documents")
	require.Empty(t, processed)
	require.Empty(t, worker.processed)
}

func TestProcessingSchedulerContinuesAfterEarlierWorkerFailure(t *testing.T) {
	repo := &stubSourceProcessingSchedulerRepo{claimedDocs: []domain.SourceDocument{
		*validClaimedSourceProcessingDocument(),
		*validClaimedSourceProcessingDocument(),
		*validClaimedSourceProcessingDocument(),
	}}
	repo.claimedDocs[0].ID = "doc-a"
	repo.claimedDocs[1].ID = "doc-b"
	repo.claimedDocs[2].ID = "doc-c"
	worker := &stubSourceProcessingSchedulerWorker{errByID: map[string]error{"doc-a": errors.New("process exploded")}}
	scheduler := NewSourceProcessingScheduler(repo, worker, 2, "scheduler-1")

	processed, err := scheduler.ProcessPending(t.Context())

	require.Error(t, err)
	require.ErrorContains(t, err, "process source document doc-a")
	require.Len(t, processed, 2)
	require.Len(t, worker.processed, 2)
	require.Equal(t, []string{"doc-a", "doc-b"}, []string{worker.processed[0].ID, worker.processed[1].ID})
}

func TestProcessingSchedulerReturnsErrorIfAnyClaimedDocumentFails(t *testing.T) {
	repo := &stubSourceProcessingSchedulerRepo{claimedDocs: []domain.SourceDocument{
		*validClaimedSourceProcessingDocument(),
		*validClaimedSourceProcessingDocument(),
	}}
	repo.claimedDocs[0].ID = "doc-a"
	repo.claimedDocs[1].ID = "doc-b"
	worker := &stubSourceProcessingSchedulerWorker{errByID: map[string]error{"doc-b": errors.New("render exploded")}}
	scheduler := NewSourceProcessingScheduler(repo, worker, 2, "scheduler-1")

	processed, err := scheduler.ProcessPending(t.Context())

	require.Error(t, err)
	require.ErrorContains(t, err, "process source document doc-b")
	require.Len(t, processed, 2)
	require.Len(t, worker.processed, 2)
}

func TestProcessingSchedulerDoesNotLeaveClaimedBatchUnprocessedAfterFailure(t *testing.T) {
	repo := &stubSourceProcessingSchedulerRepo{claimedDocs: []domain.SourceDocument{
		*validClaimedSourceProcessingDocument(),
		*validClaimedSourceProcessingDocument(),
		*validClaimedSourceProcessingDocument(),
	}}
	repo.claimedDocs[0].ID = "doc-a"
	repo.claimedDocs[1].ID = "doc-b"
	repo.claimedDocs[2].ID = "doc-c"
	worker := &stubSourceProcessingSchedulerWorker{errByID: map[string]error{"doc-b": errors.New("rewrite exploded")}}
	scheduler := NewSourceProcessingScheduler(repo, worker, 3, "scheduler-1")

	processed, err := scheduler.ProcessPending(t.Context())

	require.Error(t, err)
	require.Len(t, processed, 3)
	require.Len(t, worker.processed, 3)
	require.Equal(t, []string{"doc-a", "doc-b", "doc-c"}, []string{worker.processed[0].ID, worker.processed[1].ID, worker.processed[2].ID})
}
