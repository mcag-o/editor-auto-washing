package service

import (
	"content-hub/domain"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubSourceDocumentRepo struct {
	created []*domain.SourceDocument
	updated []*domain.SourceDocument
	createErr error
	updateErr error
}

func (r *stubSourceDocumentRepo) Create(_ context.Context, doc *domain.SourceDocument) error {
	if r.createErr != nil {
		return r.createErr
	}
	copyValue := cloneSourceDocument(doc)
	r.created = append(r.created, copyValue)
	return nil
}

func (r *stubSourceDocumentRepo) Update(_ context.Context, doc *domain.SourceDocument) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	copyValue := cloneSourceDocument(doc)
	r.updated = append(r.updated, copyValue)
	return nil
}

func (r *stubSourceDocumentRepo) GetByID(context.Context, string) (*domain.SourceDocument, error) {
	return nil, domain.NewNotFoundErr("source_document", "missing")
}

func (r *stubSourceDocumentRepo) FindByHash(context.Context, string) (*domain.SourceDocument, error) {
	return nil, domain.NewNotFoundErr("source_document_hash", "missing")
}

func (r *stubSourceDocumentRepo) ClaimPending(context.Context, int, string, time.Time) ([]domain.SourceDocument, error) {
	return nil, nil
}

func (r *stubSourceDocumentRepo) ListByStatus(context.Context, string, int) ([]domain.SourceDocument, error) {
	return nil, nil
}

func TestSourceDocumentImportPersistsAndArchivesFile(t *testing.T) {
	inbox := t.TempDir()
	archive := filepath.Join(inbox, "SyncOver")
	require.NoError(t, os.MkdirAll(archive, 0o755))
	path := filepath.Join(inbox, "article.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"title":"Title","content":"Body","summary":"Summary","tags":["news","ops"]}`), 0o644))
	repo := &stubSourceDocumentRepo{}
	svc := NewSourceDocumentImportService(repo, archive)

	doc, err := svc.ImportFile(t.Context(), path)

	require.NoError(t, err)
	require.Len(t, repo.created, 1)
	require.Len(t, repo.updated, 1)
	require.Equal(t, "article.json", doc.OriginalFilename)
	require.Equal(t, path, doc.OriginalPath)
	require.Equal(t, filepath.Join(archive, "article.json"), doc.ArchivedPath)
	require.Equal(t, "json", doc.FileType)
	require.Equal(t, "Title", doc.Title)
	require.Equal(t, "Body", doc.Body)
	require.Equal(t, "Summary", doc.Summary)
	require.Equal(t, domain.SourceDocumentStatusPending, doc.Status)
	require.NotEmpty(t, doc.Hash)
	require.NotNil(t, doc.ImportedAt)
	require.Equal(t, []string{"news", "ops"}, doc.Metadata["tags"])
	require.Equal(t, domain.SourceDocumentStatusImported, repo.created[0].Status)
	require.Empty(t, repo.created[0].ArchivedPath)
	require.Equal(t, doc.Hash, repo.created[0].Hash)
	require.Equal(t, domain.SourceDocumentStatusPending, repo.updated[0].Status)
	require.Equal(t, doc.ArchivedPath, repo.updated[0].ArchivedPath)
	_, statErr := os.Stat(filepath.Join(archive, "article.json"))
	require.NoError(t, statErr)
	_, statErr = os.Stat(path)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestSourceDocumentImportReturnsExplicitErrorWhenArchiveMoveFails(t *testing.T) {
	inbox := t.TempDir()
	path := filepath.Join(inbox, "article.md")
	require.NoError(t, os.WriteFile(path, []byte("# Title\n\nBody"), 0o644))
	repo := &stubSourceDocumentRepo{}
	svc := NewSourceDocumentImportService(repo, filepath.Join(path, "SyncOver"))

	doc, err := svc.ImportFile(t.Context(), path)

	require.Error(t, err)
	require.ErrorContains(t, err, "archive source document")
	require.Len(t, repo.created, 1)
	require.Empty(t, repo.updated)
	require.Nil(t, doc)
	_, statErr := os.Stat(path)
	require.NoError(t, statErr)
	created := repo.created[0]
	require.Equal(t, domain.SourceDocumentStatusImported, created.Status)
	require.Empty(t, created.ArchivedPath)
	require.NotEmpty(t, created.Hash)
	require.NotNil(t, created.ImportedAt)
}

func cloneSourceDocument(doc *domain.SourceDocument) *domain.SourceDocument {
	copyValue := *doc
	if doc.Metadata != nil {
		copyValue.Metadata = make(map[string]any, len(doc.Metadata))
		for key, value := range doc.Metadata {
			copyValue.Metadata[key] = value
		}
	}
	if doc.ImportedAt != nil {
		copiedTime := *doc.ImportedAt
		copyValue.ImportedAt = &copiedTime
	}
	if doc.ClaimedAt != nil {
		copiedTime := *doc.ClaimedAt
		copyValue.ClaimedAt = &copiedTime
	}
	if doc.ProcessingStartedAt != nil {
		copiedTime := *doc.ProcessingStartedAt
		copyValue.ProcessingStartedAt = &copiedTime
	}
	if doc.CompletedAt != nil {
		copiedTime := *doc.CompletedAt
		copyValue.CompletedAt = &copiedTime
	}
	return &copyValue
}
