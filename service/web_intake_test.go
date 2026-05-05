package service

import (
	"bytes"
	"errors"
	"testing"

	"content-hub/domain"

	"github.com/stretchr/testify/require"
)

func TestWebIntakeServiceCreateFromPastePersistsPendingSourceDocument(t *testing.T) {
	repo := &stubSourceDocumentRepo{}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(repo, audit)

	doc, err := svc.CreateFromPaste(t.Context(), CreatePasteIntakeInput{
		Actor: "local-admin",
		Title: "Title",
		Body:  "Body",
	})

	require.NoError(t, err)
	require.NotNil(t, doc)
	require.Equal(t, "paste", doc.SourceType)
	require.Equal(t, domain.SourceDocumentStatusPending, doc.Status)
	require.Equal(t, "Title", doc.Title)
	require.Equal(t, "Body", doc.Body)
	require.Equal(t, "paste", doc.OriginalFilename)
	require.Equal(t, "paste", doc.OriginalPath)
	require.Equal(t, "txt", doc.FileType)
	require.NotEmpty(t, doc.Hash)
	require.Nil(t, doc.ImportedAt)
	require.Len(t, repo.created, 1)
	require.Equal(t, domain.SourceDocumentStatusPending, repo.created[0].Status)
	require.Len(t, audit.logs, 1)
	require.Equal(t, "local-admin", audit.logs[0].Actor)
	require.Equal(t, "web_intake.create_from_paste", audit.logs[0].Action)
	require.Equal(t, "source_document", audit.logs[0].Resource)
	require.Equal(t, doc.ID, audit.logs[0].ResourceID)
	require.Equal(t, "success", audit.logs[0].Result)
}

func TestWebIntakeServiceCreateFromUploadSupportsMarkdown(t *testing.T) {
	repo := &stubSourceDocumentRepo{}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(repo, audit)

	doc, err := svc.CreateFromUpload(t.Context(), CreateUploadIntakeInput{
		Actor:       "local-admin",
		Filename:    "article.md",
		ContentType: "text/markdown",
		Content:     bytes.NewBufferString("# Title\n\nBody"),
	})

	require.NoError(t, err)
	require.NotNil(t, doc)
	require.Equal(t, "upload", doc.SourceType)
	require.Equal(t, domain.SourceDocumentStatusPending, doc.Status)
	require.Equal(t, "article.md", doc.OriginalFilename)
	require.Equal(t, "article.md", doc.OriginalPath)
	require.Equal(t, "md", doc.FileType)
	require.Equal(t, "Title", doc.Title)
	require.Equal(t, "# Title\n\nBody", doc.Body)
	require.Len(t, repo.created, 1)
	require.Len(t, audit.logs, 1)
	require.Equal(t, "web_intake.create_from_upload", audit.logs[0].Action)
	require.Equal(t, "success", audit.logs[0].Result)
	require.Equal(t, doc.ID, audit.logs[0].ResourceID)
}

func TestWebIntakeServiceCreateFromUploadUsesOriginalFilenameForFallbackTitle(t *testing.T) {
	repo := &stubSourceDocumentRepo{}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(repo, audit)

	doc, err := svc.CreateFromUpload(t.Context(), CreateUploadIntakeInput{
		Actor:       "local-admin",
		Filename:    "notes.txt",
		ContentType: "text/plain",
		Content:     bytes.NewBufferString("Body"),
	})

	require.NoError(t, err)
	require.Equal(t, "notes", doc.Title)
}

func TestWebIntakeServiceCreateFromUploadRejectsUnsupportedExtension(t *testing.T) {
	repo := &stubSourceDocumentRepo{}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(repo, audit)

	doc, err := svc.CreateFromUpload(t.Context(), CreateUploadIntakeInput{
		Actor:       "local-admin",
		Filename:    "article.docx",
		ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		Content:     bytes.NewBufferString("not-used"),
	})

	require.Nil(t, doc)
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported source document type")
	require.Empty(t, repo.created)
	require.Len(t, audit.logs, 1)
	require.Equal(t, "web_intake.create_from_upload", audit.logs[0].Action)
	require.Equal(t, "failure", audit.logs[0].Result)
}

func TestWebIntakeServiceCreateFromPasteRecordsFailedAuditWhenCreateFails(t *testing.T) {
	repo := &stubSourceDocumentRepo{createErr: errors.New("db down")}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(repo, audit)

	doc, err := svc.CreateFromPaste(t.Context(), CreatePasteIntakeInput{
		Actor: "local-admin",
		Title: "Title",
		Body:  "Body",
	})

	require.Nil(t, doc)
	require.Error(t, err)
	require.ErrorContains(t, err, "create source document")
	require.Len(t, audit.logs, 1)
	require.Equal(t, "web_intake.create_from_paste", audit.logs[0].Action)
	require.Equal(t, "failure", audit.logs[0].Result)
	require.Empty(t, audit.logs[0].ResourceID)
	metadata, ok := audit.logs[0].Metadata["title"]
	require.True(t, ok)
	require.Equal(t, "Title", metadata)
}
