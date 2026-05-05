package service

import (
	"content-hub/domain"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubSourceDocumentRepo struct {
	created     []*domain.SourceDocument
	updated     []*domain.SourceDocument
	createErr   error
	updateErr   error
	updateCalls int
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
	r.updateCalls++
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

func (r *stubSourceDocumentRepo) List(context.Context, int) ([]domain.SourceDocument, error) {
	return nil, nil
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
	require.FileExists(t, doc.ArchivedPath)
	require.DirExists(t, archive)
	require.Equal(t, archive, filepath.Dir(doc.ArchivedPath))
	require.NotEqual(t, filepath.Join(archive, "article.json"), doc.ArchivedPath)
	requireArchivedPathSuffixStrategy(t, doc)
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
	_, statErr := os.Stat(doc.ArchivedPath)
	require.NoError(t, statErr)
	_, statErr = os.Stat(path)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestSourceDocumentImportSameNameImportsProduceDifferentArchivedPaths(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "SyncOver")
	require.NoError(t, os.MkdirAll(archive, 0o755))

	firstInbox := filepath.Join(root, "inbox-a")
	secondInbox := filepath.Join(root, "inbox-b")
	require.NoError(t, os.MkdirAll(firstInbox, 0o755))
	require.NoError(t, os.MkdirAll(secondInbox, 0o755))

	firstPath := filepath.Join(firstInbox, "article.md")
	secondPath := filepath.Join(secondInbox, "article.md")
	require.NoError(t, os.WriteFile(firstPath, []byte("# Title A\n\nBody A"), 0o644))
	require.NoError(t, os.WriteFile(secondPath, []byte("# Title B\n\nBody B"), 0o644))

	repo := &stubSourceDocumentRepo{}
	svc := NewSourceDocumentImportService(repo, archive)

	firstDoc, err := svc.ImportFile(t.Context(), firstPath)
	require.NoError(t, err)
	secondDoc, err := svc.ImportFile(t.Context(), secondPath)
	require.NoError(t, err)

	require.NotEqual(t, firstDoc.ArchivedPath, secondDoc.ArchivedPath)
	require.FileExists(t, firstDoc.ArchivedPath)
	require.FileExists(t, secondDoc.ArchivedPath)
	require.Equal(t, archive, filepath.Dir(firstDoc.ArchivedPath))
	require.Equal(t, archive, filepath.Dir(secondDoc.ArchivedPath))
	requireArchivedPathSuffixStrategy(t, firstDoc)
	requireArchivedPathSuffixStrategy(t, secondDoc)
}

func TestSourceDocumentImportSameSourcePathDifferentContentGetsDifferentArchivedPaths(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "SyncOver")
	require.NoError(t, os.MkdirAll(archive, 0o755))

	path := filepath.Join(root, "article.md")
	repo := &stubSourceDocumentRepo{}
	svc := NewSourceDocumentImportService(repo, archive)

	require.NoError(t, os.WriteFile(path, []byte("# Title A\n\nBody A"), 0o644))
	firstDoc, err := svc.ImportFile(t.Context(), path)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path, []byte("# Title B\n\nBody B"), 0o644))
	secondDoc, err := svc.ImportFile(t.Context(), path)
	require.NoError(t, err)

	require.NotEqual(t, firstDoc.ArchivedPath, secondDoc.ArchivedPath)
	require.NotEqual(t, firstDoc.Hash, secondDoc.Hash)
	require.FileExists(t, firstDoc.ArchivedPath)
	require.FileExists(t, secondDoc.ArchivedPath)
	requireArchivedPathSuffixStrategy(t, firstDoc)
	requireArchivedPathSuffixStrategy(t, secondDoc)
}

func TestSourceDocumentImportSameSourcePathSameContentStillGetsUniqueArchivedPath(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "SyncOver")
	require.NoError(t, os.MkdirAll(archive, 0o755))

	path := filepath.Join(root, "article.md")
	repo := &stubSourceDocumentRepo{}
	svc := NewSourceDocumentImportService(repo, archive)

	content := []byte("# Same Title\n\nSame Body")
	require.NoError(t, os.WriteFile(path, content, 0o644))
	firstDoc, err := svc.ImportFile(t.Context(), path)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path, content, 0o644))
	secondDoc, err := svc.ImportFile(t.Context(), path)
	require.NoError(t, err)

	require.Equal(t, firstDoc.Hash, secondDoc.Hash)
	require.NotEqual(t, firstDoc.ID, secondDoc.ID)
	require.NotEqual(t, firstDoc.ArchivedPath, secondDoc.ArchivedPath)
	require.FileExists(t, firstDoc.ArchivedPath)
	require.FileExists(t, secondDoc.ArchivedPath)
	requireArchivedPathSuffixStrategy(t, firstDoc)
	requireArchivedPathSuffixStrategy(t, secondDoc)
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

func TestSourceDocumentImportMarksDivergenceWhenFinalUpdateFails(t *testing.T) {
	inbox := t.TempDir()
	archive := filepath.Join(inbox, "SyncOver")
	require.NoError(t, os.MkdirAll(archive, 0o755))
	path := filepath.Join(inbox, "article.md")
	require.NoError(t, os.WriteFile(path, []byte("# Title\n\nBody"), 0o644))
	repo := &stubSourceDocumentRepo{updateErr: os.ErrPermission}
	svc := NewSourceDocumentImportService(repo, archive)

	doc, err := svc.ImportFile(t.Context(), path)

	require.Nil(t, doc)
	require.Error(t, err)
	require.ErrorContains(t, err, "source document archive state diverged")
	require.Len(t, repo.created, 1)
	require.Len(t, repo.updated, 0)
	require.Equal(t, 2, repo.updateCalls)
	archivedMatches, globErr := filepath.Glob(filepath.Join(archive, "article.*.md"))
	require.NoError(t, globErr)
	require.Len(t, archivedMatches, 1)
	require.FileExists(t, archivedMatches[0])
	require.NoFileExists(t, path)
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

func requireArchivedPathSuffixStrategy(t *testing.T, doc *domain.SourceDocument) {
	t.Helper()
	base := filepath.Base(doc.ArchivedPath)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)
	parts := strings.Split(nameWithoutExt, ".")
	require.GreaterOrEqual(t, len(parts), 3)
	require.Equal(t, trimFilenameExt(doc.OriginalFilename), strings.Join(parts[:len(parts)-2], "."))
	require.Equal(t, doc.Hash[:12], parts[len(parts)-2])
	require.Equal(t, doc.ID, parts[len(parts)-1])
	require.Equal(t, filepath.Ext(doc.OriginalFilename), ext)
}

func trimFilenameExt(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}
