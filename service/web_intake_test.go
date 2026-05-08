package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"content-hub/domain"
	"content-hub/pkg/repo"

	"github.com/stretchr/testify/require"
)

type stubAuditLogRepoWithCreateError struct {
	err  error
	logs []*domain.AuditLog
}

func (r *stubAuditLogRepoWithCreateError) Create(_ context.Context, log *domain.AuditLog) error {
	if err := log.Validate(); err != nil {
		return err
	}
	copyLog := *log
	r.logs = append(r.logs, &copyLog)
	return r.err
}

func (r *stubAuditLogRepoWithCreateError) GetByID(_ context.Context, id string) (*domain.AuditLog, error) {
	for _, log := range r.logs {
		if log.ID == id {
			copyLog := *log
			return &copyLog, nil
		}
	}
	return nil, domain.NewNotFoundErr("audit_log", id)
}

func (r *stubAuditLogRepoWithCreateError) List(_ context.Context, limit int) ([]domain.AuditLog, error) {
	if limit <= 0 || limit > len(r.logs) {
		limit = len(r.logs)
	}
	out := make([]domain.AuditLog, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, *r.logs[i])
	}
	return out, nil
}

func (r *stubAuditLogRepoWithCreateError) ListByQuery(_ context.Context, query repo.AuditLogQuery) ([]domain.AuditLog, error) {
	out := make([]domain.AuditLog, 0, len(r.logs))
	for _, log := range r.logs {
		if query.WorkflowRunID != "" {
			workflowRunID, _ := log.Metadata["workflow_run_id"].(string)
			if workflowRunID != query.WorkflowRunID {
				continue
			}
		}
		if query.ActionPrefix != "" && !strings.HasPrefix(log.Action, query.ActionPrefix) {
			continue
		}
		if query.ResourceID != "" && log.ResourceID != query.ResourceID {
			continue
		}
		out = append(out, *log)
	}
	return out, nil
}

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
	require.Equal(t, defaultWebIntakeTargetType, doc.Metadata["target_type"])
	require.Equal(t, defaultWebPasteSourceProfile, doc.Metadata["source_profile"])
	require.Equal(t, defaultWebIntakeRenderPlatform, doc.Metadata["render_platform"])
	require.Equal(t, defaultWebIntakeRewriteProfileVersion, doc.Metadata["rewrite_profile_version"])
	require.Len(t, audit.logs, 1)
	require.Equal(t, "local-admin", audit.logs[0].Actor)
	require.Equal(t, "web_intake.create_from_paste", audit.logs[0].Action)
	require.Equal(t, "source_document", audit.logs[0].Resource)
	require.Equal(t, doc.ID, audit.logs[0].ResourceID)
	require.Equal(t, "success", audit.logs[0].Result)
}

func TestWebIntakeServiceCreateFromPastePreservesOriginalBodyText(t *testing.T) {
	repo := &stubSourceDocumentRepo{}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(repo, audit)

	originalBody := "  Body with preserved edges\n"
	doc, err := svc.CreateFromPaste(t.Context(), CreatePasteIntakeInput{
		Actor: "local-admin",
		Title: "Title",
		Body:  originalBody,
	})

	require.NoError(t, err)
	require.Equal(t, originalBody, doc.Body)
	require.Equal(t, originalBody, repo.created[0].Body)
}

func TestWebIntakeServiceCreateFromPasteRejectsEffectivelyEmptyBody(t *testing.T) {
	repo := &stubSourceDocumentRepo{}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(repo, audit)

	doc, err := svc.CreateFromPaste(t.Context(), CreatePasteIntakeInput{
		Actor: "local-admin",
		Title: "Title",
		Body:  "   \n\t  ",
	})

	require.Nil(t, doc)
	require.Error(t, err)
	require.ErrorContains(t, err, "body is required")
	require.Empty(t, repo.created)
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
	require.Equal(t, defaultWebIntakeTargetType, doc.Metadata["target_type"])
	require.Equal(t, defaultWebUploadSourceProfile, doc.Metadata["source_profile"])
	require.Equal(t, defaultWebIntakeRenderPlatform, doc.Metadata["render_platform"])
	require.Equal(t, defaultWebIntakeRewriteProfileVersion, doc.Metadata["rewrite_profile_version"])
	require.Len(t, repo.created, 1)
	require.Len(t, audit.logs, 1)
	require.Equal(t, "web_intake.create_from_upload", audit.logs[0].Action)
	require.Equal(t, "success", audit.logs[0].Result)
	require.Equal(t, doc.ID, audit.logs[0].ResourceID)
}

func TestWebIntakeServiceCreateFromUploadSucceedsWhenSuccessAuditWriteFails(t *testing.T) {
	repo := &stubSourceDocumentRepo{}
	audit := &stubAuditLogRepoWithCreateError{err: fmt.Errorf("audit down")}
	svc := NewWebIntakeService(repo, audit)

	doc, err := svc.CreateFromUpload(t.Context(), CreateUploadIntakeInput{
		Actor:       "local-admin",
		Filename:    "article.md",
		ContentType: "text/markdown",
		Content:     bytes.NewBufferString("# Title\n\nBody"),
	})

	require.NoError(t, err)
	require.NotNil(t, doc)
	require.Len(t, repo.created, 1)
	require.Equal(t, doc.ID, repo.created[0].ID)
	require.Len(t, audit.logs, 1)
	require.Equal(t, "success", audit.logs[0].Result)
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
