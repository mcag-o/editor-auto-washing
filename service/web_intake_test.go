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

type stubWebIntakeWorkspaceRepo struct {
	created []*domain.ArticleWorkspaceRecord
	err     error
}

func (r *stubWebIntakeWorkspaceRepo) Create(_ context.Context, record *domain.ArticleWorkspaceRecord) error {
	if r.err != nil {
		return r.err
	}
	copyValue := cloneWorkspaceRecord(record)
	r.created = append(r.created, copyValue)
	return nil
}

func (r *stubWebIntakeWorkspaceRepo) Update(_ context.Context, record *domain.ArticleWorkspaceRecord) error {
	if r.err != nil {
		return r.err
	}
	for i, item := range r.created {
		if item.ID == record.ID {
			r.created[i] = cloneWorkspaceRecord(record)
			return nil
		}
	}
	return domain.NewNotFoundErr("workspace_article", record.ID)
}

func (r *stubWebIntakeWorkspaceRepo) GetByID(_ context.Context, id string) (*domain.ArticleWorkspaceRecord, error) {
	for _, item := range r.created {
		if item.ID == id {
			return cloneWorkspaceRecord(item), nil
		}
	}
	return nil, domain.NewNotFoundErr("workspace_article", id)
}

func (r *stubWebIntakeWorkspaceRepo) List(context.Context, *string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}

func (r *stubWebIntakeWorkspaceRepo) ListByIngestionID(context.Context, string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}

func (r *stubWebIntakeWorkspaceRepo) TransitionStatus(_ context.Context, id string, newStatus, notes string) error {
	for _, item := range r.created {
		if item.ID == id {
			item.Status = newStatus
			item.Notes = notes
			item.StatusHistory = append(item.StatusHistory, newStatus)
			item.LifecycleHistory = append(item.LifecycleHistory, domain.ArticleWorkspaceLifecycleEntry{Status: newStatus, Notes: notes, CreatedAt: item.UpdatedAt})
			return nil
		}
	}
	return domain.NewNotFoundErr("workspace_article", id)
}

func (r *stubWebIntakeWorkspaceRepo) Delete(context.Context, string) error {
	return nil
}

func cloneWorkspaceRecord(record *domain.ArticleWorkspaceRecord) *domain.ArticleWorkspaceRecord {
	if record == nil {
		return nil
	}
	copyValue := *record
	copyValue.StatusHistory = append([]string(nil), record.StatusHistory...)
	copyValue.LifecycleHistory = append([]domain.ArticleWorkspaceLifecycleEntry(nil), record.LifecycleHistory...)
	if record.Metadata != nil {
		copyValue.Metadata = map[string]any{}
		for key, value := range record.Metadata {
			copyValue.Metadata[key] = value
		}
	}
	return &copyValue
}

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

func TestWebIntakeServiceCreateFromPastePersistsWorkspaceIntakeItem(t *testing.T) {
	workspaceRepo := &stubWebIntakeWorkspaceRepo{}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(workspaceRepo, audit)

	workspace, err := svc.CreateFromPaste(t.Context(), CreatePasteIntakeInput{
		Actor: "local-admin",
		Title: "Title",
		Body:  "Body",
	})

	require.NoError(t, err)
	require.NotNil(t, workspace)
	require.Equal(t, domain.ArticleWorkspaceStatusImported, workspace.Status)
	require.Equal(t, "Title", workspace.Title)
	require.Equal(t, "paste", workspace.Source.SourceType)
	require.Equal(t, browserPasteOriginalURL("Title"), workspace.Source.URL)
	require.Equal(t, "Body", workspace.Metadata["source_body"])
	require.Equal(t, defaultWebIntakeTargetType, workspace.Metadata["target_type"])
	require.Equal(t, defaultWebPasteSourceProfile, workspace.Metadata["source_profile"])
	require.Equal(t, defaultWebIntakeRenderPlatform, workspace.Metadata["render_platform"])
	require.Equal(t, defaultWebIntakeRewriteProfileVersion, workspace.Metadata["rewrite_profile_version"])
	require.Len(t, workspaceRepo.created, 1)
	require.Equal(t, workspace.ID, workspaceRepo.created[0].ID)
	require.Len(t, audit.logs, 1)
	require.Equal(t, "local-admin", audit.logs[0].Actor)
	require.Equal(t, "web_intake.create_from_paste", audit.logs[0].Action)
	require.Equal(t, "workspace_article", audit.logs[0].Resource)
	require.Equal(t, workspace.ID, audit.logs[0].ResourceID)
	require.Equal(t, "success", audit.logs[0].Result)
}

func TestWebIntakeServiceCreateFromPastePreservesOriginalBodyText(t *testing.T) {
	originalBody := "  Body with preserved edges\n"
	workspaceRepo := &stubWebIntakeWorkspaceRepo{}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(workspaceRepo, audit)

	workspace, err := svc.CreateFromPaste(t.Context(), CreatePasteIntakeInput{
		Actor: "local-admin",
		Title: "Title",
		Body:  originalBody,
	})

	require.NoError(t, err)
	require.Equal(t, originalBody, workspace.Metadata["source_body"])
}

func TestWebIntakeServiceCreateFromPasteRejectsEffectivelyEmptyBody(t *testing.T) {
	workspaceRepo := &stubWebIntakeWorkspaceRepo{}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(workspaceRepo, audit)

	workspace, err := svc.CreateFromPaste(t.Context(), CreatePasteIntakeInput{
		Actor: "local-admin",
		Title: "Title",
		Body:  "   \n\t  ",
	})

	require.Nil(t, workspace)
	require.Error(t, err)
	require.ErrorContains(t, err, "body is required")
	require.Empty(t, workspaceRepo.created)
}

func TestWebIntakeServiceCreateFromUploadPersistsWorkspaceIntakeItem(t *testing.T) {
	workspaceRepo := &stubWebIntakeWorkspaceRepo{}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(workspaceRepo, audit)

	workspace, err := svc.CreateFromUpload(t.Context(), CreateUploadIntakeInput{
		Actor:       "local-admin",
		Filename:    "article.md",
		ContentType: "text/markdown",
		Content:     bytes.NewBufferString("# Title\n\nBody"),
	})

	require.NoError(t, err)
	require.NotNil(t, workspace)
	require.Equal(t, domain.ArticleWorkspaceStatusImported, workspace.Status)
	require.Equal(t, "Title", workspace.Title)
	require.Equal(t, "upload", workspace.Source.SourceType)
	require.Equal(t, browserUploadOriginalURL("article.md"), workspace.Source.URL)
	require.Equal(t, "# Title\n\nBody", workspace.Metadata["source_body"])
	require.Equal(t, defaultWebIntakeTargetType, workspace.Metadata["target_type"])
	require.Equal(t, defaultWebUploadSourceProfile, workspace.Metadata["source_profile"])
	require.Equal(t, defaultWebIntakeRenderPlatform, workspace.Metadata["render_platform"])
	require.Equal(t, defaultWebIntakeRewriteProfileVersion, workspace.Metadata["rewrite_profile_version"])
	require.Equal(t, "text/markdown", workspace.Metadata["content_type"])
	require.Len(t, workspaceRepo.created, 1)
	require.Len(t, audit.logs, 1)
	require.Equal(t, "web_intake.create_from_upload", audit.logs[0].Action)
	require.Equal(t, "success", audit.logs[0].Result)
	require.Equal(t, workspace.ID, audit.logs[0].ResourceID)
}

func TestWebIntakeServiceCreateFromUploadSucceedsWhenSuccessAuditWriteFails(t *testing.T) {
	workspaceRepo := &stubWebIntakeWorkspaceRepo{}
	audit := &stubAuditLogRepoWithCreateError{err: fmt.Errorf("audit down")}
	svc := NewWebIntakeService(workspaceRepo, audit)

	workspace, err := svc.CreateFromUpload(t.Context(), CreateUploadIntakeInput{
		Actor:       "local-admin",
		Filename:    "article.md",
		ContentType: "text/markdown",
		Content:     bytes.NewBufferString("# Title\n\nBody"),
	})

	require.NoError(t, err)
	require.NotNil(t, workspace)
	require.Len(t, audit.logs, 1)
	require.Equal(t, "success", audit.logs[0].Result)
}

func TestWebIntakeServiceCreateFromUploadUsesOriginalFilenameForFallbackTitle(t *testing.T) {
	workspaceRepo := &stubWebIntakeWorkspaceRepo{}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(workspaceRepo, audit)

	workspace, err := svc.CreateFromUpload(t.Context(), CreateUploadIntakeInput{
		Actor:       "local-admin",
		Filename:    "notes.txt",
		ContentType: "text/plain",
		Content:     bytes.NewBufferString("Body"),
	})

	require.NoError(t, err)
	require.Equal(t, "notes", workspace.Title)
}

func TestWebIntakeServiceCreateFromUploadRejectsUnsupportedExtension(t *testing.T) {
	workspaceRepo := &stubWebIntakeWorkspaceRepo{}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(workspaceRepo, audit)

	workspace, err := svc.CreateFromUpload(t.Context(), CreateUploadIntakeInput{
		Actor:       "local-admin",
		Filename:    "article.docx",
		ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		Content:     bytes.NewBufferString("not-used"),
	})

	require.Nil(t, workspace)
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported input document type")
	require.Empty(t, workspaceRepo.created)
	require.Len(t, audit.logs, 1)
	require.Equal(t, "web_intake.create_from_upload", audit.logs[0].Action)
	require.Equal(t, "failure", audit.logs[0].Result)
}

func TestBuildBrowserIntakeResponseProjectsWorkspaceRecordWithoutLegacyShape(t *testing.T) {
	workspace := domain.NewArticleWorkspaceRecord("workspace-1", "Title", "Summary", domain.ArticleWorkspaceSource{
		SourceType: "upload",
		URL:        browserUploadOriginalURL("article.md"),
	}, map[string]any{
		"source_body":             "Body",
		"target_type":             defaultWebIntakeTargetType,
		"source_profile":          defaultWebUploadSourceProfile,
		"render_platform":         defaultWebIntakeRenderPlatform,
		"rewrite_profile_version": defaultWebIntakeRewriteProfileVersion,
	})

	resp := BuildBrowserIntakeResponse(workspace)

	require.NotNil(t, resp)
	require.Equal(t, workspace.ID, resp.ID)
	require.Equal(t, workspace.Title, resp.Title)
	require.Equal(t, workspace.Summary, resp.Summary)
	require.Equal(t, workspace.Status, resp.Status)
	require.Equal(t, "Body", resp.Body)
	require.Equal(t, "upload", resp.Metadata.SourceType)
	require.Equal(t, browserUploadOriginalURL("article.md"), resp.Metadata.OriginalURL)
	require.Equal(t, defaultWebIntakeTargetType, resp.Metadata.TargetType)
	require.Equal(t, defaultWebUploadSourceProfile, resp.Metadata.SourceProfile)
	require.Equal(t, defaultWebIntakeRenderPlatform, resp.Metadata.RenderPlatform)
	require.Equal(t, defaultWebIntakeRewriteProfileVersion, resp.Metadata.RewriteProfileVersion)
}

func TestBuildBrowserIntakeResponseProjectsPersistedValuesAsStored(t *testing.T) {
	workspace := domain.NewArticleWorkspaceRecord(" workspace-1 ", " Title ", " Summary ", domain.ArticleWorkspaceSource{
		SourceType: " upload ",
		URL:        " browser://upload/article.md ",
	}, map[string]any{
		"source_body":             " Body ",
		"target_type":             " wechat-longform ",
		"source_profile":          " web-upload ",
		"render_platform":         " wechat ",
		"rewrite_profile_version": " v1 ",
	})

	resp := BuildBrowserIntakeResponse(workspace)

	require.NotNil(t, resp)
	require.Equal(t, " workspace-1 ", resp.ID)
	require.Equal(t, " Title ", resp.Title)
	require.Equal(t, " Summary ", resp.Summary)
	require.Equal(t, " Body ", resp.Body)
	require.Equal(t, " upload ", resp.Metadata.SourceType)
	require.Equal(t, " browser://upload/article.md ", resp.Metadata.OriginalURL)
	require.Equal(t, " wechat-longform ", resp.Metadata.TargetType)
	require.Equal(t, " web-upload ", resp.Metadata.SourceProfile)
	require.Equal(t, " wechat ", resp.Metadata.RenderPlatform)
	require.Equal(t, " v1 ", resp.Metadata.RewriteProfileVersion)
}

func TestWebIntakeServiceCreateFromPasteRecordsFailedAuditWhenWorkspaceCreateFails(t *testing.T) {
	workspaceRepo := &stubWebIntakeWorkspaceRepo{err: errors.New("db down")}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(workspaceRepo, audit)

	workspace, err := svc.CreateFromPaste(t.Context(), CreatePasteIntakeInput{
		Actor: "local-admin",
		Title: "Title",
		Body:  "Body",
	})

	require.Nil(t, workspace)
	require.Error(t, err)
	require.ErrorContains(t, err, "create workspace article")
	require.Len(t, audit.logs, 1)
	require.Equal(t, "web_intake.create_from_paste", audit.logs[0].Action)
	require.Equal(t, "failure", audit.logs[0].Result)
	require.Empty(t, audit.logs[0].ResourceID)
	metadata, ok := audit.logs[0].Metadata["title"]
	require.True(t, ok)
	require.Equal(t, "Title", metadata)
}

func TestWebIntakeServiceCreateFromExternalArticlePersistsQueuedWorkspaceItem(t *testing.T) {
	workspaceRepo := &stubWebIntakeWorkspaceRepo{}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(workspaceRepo, audit)

	workspace, err := svc.CreateFromExternalArticle(t.Context(), CreateExternalArticleIntakeInput{
		Actor: "crawler",
		Article: domain.IntakeArticle{
			SourceType:            "xiaohongshu",
			Title:                 "Title",
			Body:                  "Body",
			Summary:               "Summary",
			OriginalURL:           "https://example.com/source/1",
			TargetType:            "wechat-longform",
			SourceProfile:         "crawler-xhs",
			RewriteProfileVersion: "v2",
			Metadata: map[string]any{
				"external_id": "note-1",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, workspace)
	require.Equal(t, domain.ArticleWorkspaceStatusImported, workspace.Status)
	require.Equal(t, "xiaohongshu", workspace.Source.SourceType)
	require.Equal(t, "https://example.com/source/1", workspace.Source.URL)
	require.Equal(t, "Body", workspace.Metadata["source_body"])
	require.Equal(t, "wechat-longform", workspace.Metadata["target_type"])
	require.Equal(t, "crawler-xhs", workspace.Metadata["source_profile"])
	require.Equal(t, "v2", workspace.Metadata["rewrite_profile_version"])
	require.Equal(t, defaultWebIntakeRenderPlatform, workspace.Metadata["render_platform"])
	require.Equal(t, "external_api", workspace.Metadata["intake_origin"])
	require.Equal(t, "note-1", workspace.Metadata["external_id"])
	require.Len(t, audit.logs, 1)
	require.Equal(t, "api_intake.create_external_article", audit.logs[0].Action)
}

func TestWebIntakeServiceCreateFromExternalFileParsesAndPersistsWorkspaceItem(t *testing.T) {
	workspaceRepo := &stubWebIntakeWorkspaceRepo{}
	audit := &stubAuditLogRepo{}
	svc := NewWebIntakeService(workspaceRepo, audit)

	workspace, err := svc.CreateFromExternalFile(t.Context(), CreateExternalFileIntakeInput{
		Actor:       "crawler",
		Filename:    "article.md",
		ContentType: "text/markdown",
		Content:     bytes.NewBufferString("# Title\n\nBody"),
		Metadata: map[string]any{
			"external_id": "file-1",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, workspace)
	require.Equal(t, "Title", workspace.Title)
	require.Equal(t, "external-file", workspace.Source.SourceType)
	require.Equal(t, "external://file/article.md", workspace.Source.URL)
	require.Equal(t, "# Title\n\nBody", workspace.Metadata["source_body"])
	require.Equal(t, "text/markdown", workspace.Metadata["content_type"])
	require.Equal(t, "external_api", workspace.Metadata["intake_origin"])
	require.Equal(t, "file-1", workspace.Metadata["external_id"])
}
