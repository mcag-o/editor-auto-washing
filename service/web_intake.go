package service

import (
	"content-hub/domain"
	"content-hub/pkg/id"
	"content-hub/pkg/repo"
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
)

const (
	defaultWebIntakeTargetType            = "wechat-longform"
	defaultWebIntakeRenderPlatform        = "wechat"
	defaultWebIntakeRewriteProfileVersion = "v1"
	defaultWebPasteSourceProfile          = "web-paste"
	defaultWebUploadSourceProfile         = "web-upload"
)

type CreatePasteIntakeInput struct {
	Actor string
	Title string
	Body  string
}

type CreateUploadIntakeInput struct {
	Actor       string
	Filename    string
	ContentType string
	Content     io.Reader
}

type WebIntakeService struct {
	workspaces repo.WorkspaceRepo
	audit      repo.AuditLogRepo
}

func NewWebIntakeService(workspaces repo.WorkspaceRepo, audit repo.AuditLogRepo) *WebIntakeService {
	return &WebIntakeService{workspaces: workspaces, audit: audit}
}

func (s *WebIntakeService) CreateFromPaste(ctx context.Context, input CreatePasteIntakeInput) (*domain.ArticleWorkspaceRecord, error) {
	if s.workspaces == nil || s.audit == nil {
		return nil, domain.NewInternalErr("web intake service is not configured", nil)
	}

	title := strings.TrimSpace(input.Title)
	body := input.Body
	if title == "" {
		err := domain.NewValidationErr("title is required", nil)
		s.recordFailureAudit(ctx, input.Actor, "web_intake.create_from_paste", err.Error(), map[string]any{"title": title})
		return nil, err
	}
	if strings.TrimSpace(body) == "" {
		err := domain.NewValidationErr("body is required", nil)
		s.recordFailureAudit(ctx, input.Actor, "web_intake.create_from_paste", err.Error(), map[string]any{"title": title})
		return nil, err
	}

	workspace := domain.NewArticleWorkspaceRecord(id.New(), title, "", domain.ArticleWorkspaceSource{
		SourceType: "paste",
		URL:        browserPasteOriginalURL(title),
	}, webIntakeWorkspaceMetadata(body, webIntakeMetadata(defaultWebPasteSourceProfile)))
	workspace.LifecycleHistory = []domain.ArticleWorkspaceLifecycleEntry{{
		Status:    domain.ArticleWorkspaceStatusImported,
		Notes:     "created from browser paste intake",
		CreatedAt: workspace.CreatedAt,
	}}

	if err := s.workspaces.Create(ctx, workspace); err != nil {
		wrapped := fmt.Errorf("create workspace article: %w", err)
		s.recordFailureAudit(ctx, input.Actor, "web_intake.create_from_paste", wrapped.Error(), map[string]any{"title": title})
		return nil, wrapped
	}

	if err := s.recordAudit(ctx, AuditLogCreateInput{
		Actor:      input.Actor,
		Action:     "web_intake.create_from_paste",
		Resource:   "workspace_article",
		ResourceID: workspace.ID,
		Result:     "success",
		Message:    "created workspace article from pasted text",
		Metadata: map[string]any{
			"title": title,
		},
	}); err != nil {
		s.logAuditFailure("web_intake.create_from_paste", workspace.ID, err)
	}

	return workspace, nil
}

func (s *WebIntakeService) CreateFromUpload(ctx context.Context, input CreateUploadIntakeInput) (*domain.ArticleWorkspaceRecord, error) {
	if s.workspaces == nil || s.audit == nil {
		return nil, domain.NewInternalErr("web intake service is not configured", nil)
	}

	filename := strings.TrimSpace(input.Filename)
	if filename == "" {
		err := domain.NewValidationErr("filename is required", nil)
		s.recordFailureAudit(ctx, input.Actor, "web_intake.create_from_upload", err.Error(), map[string]any{"filename": filename})
		return nil, err
	}
	if input.Content == nil {
		err := domain.NewValidationErr("content is required", nil)
		s.recordFailureAudit(ctx, input.Actor, "web_intake.create_from_upload", err.Error(), map[string]any{"filename": filename})
		return nil, err
	}
	if !isSupportedWebUploadExtension(filename) {
		err := domain.NewValidationErr("unsupported source document type", nil)
		s.recordFailureAudit(ctx, input.Actor, "web_intake.create_from_upload", err.Error(), map[string]any{"filename": filename})
		return nil, err
	}

	parsed, err := s.parseUploadedDocument(filename, input.Content)
	if err != nil {
		s.recordFailureAudit(ctx, input.Actor, "web_intake.create_from_upload", err.Error(), map[string]any{"filename": filename})
		return nil, err
	}

	metadata := webIntakeMetadata(defaultWebUploadSourceProfile)
	for key, value := range sourceDocumentMetadata(parsed) {
		metadata[key] = value
	}
	if contentType := strings.TrimSpace(input.ContentType); contentType != "" {
		metadata["content_type"] = contentType
	}

	workspace := domain.NewArticleWorkspaceRecord(id.New(), parsed.Title, parsed.Summary, domain.ArticleWorkspaceSource{
		SourceType: "upload",
		URL:        browserUploadOriginalURL(filename),
	}, webIntakeWorkspaceMetadata(parsed.Body, metadata))
	workspace.LifecycleHistory = []domain.ArticleWorkspaceLifecycleEntry{{
		Status:    domain.ArticleWorkspaceStatusImported,
		Notes:     "created from browser upload intake",
		CreatedAt: workspace.CreatedAt,
	}}

	if err := s.workspaces.Create(ctx, workspace); err != nil {
		wrapped := fmt.Errorf("create workspace article: %w", err)
		s.recordFailureAudit(ctx, input.Actor, "web_intake.create_from_upload", wrapped.Error(), map[string]any{"filename": filename})
		return nil, wrapped
	}

	if err := s.recordAudit(ctx, AuditLogCreateInput{
		Actor:      input.Actor,
		Action:     "web_intake.create_from_upload",
		Resource:   "workspace_article",
		ResourceID: workspace.ID,
		Result:     "success",
		Message:    "created workspace article from browser upload",
		Metadata: map[string]any{
			"filename":  filename,
			"file_type": strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), "."),
		},
	}); err != nil {
		s.logAuditFailure("web_intake.create_from_upload", workspace.ID, err)
	}

	return workspace, nil
}

func (s *WebIntakeService) parseUploadedDocument(filename string, content io.Reader) (*ParsedSourceDocument, error) {
	body, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("read upload content: %w", err)
	}

	return ParseSourceDocumentBytes(filename, body)
}

func isSupportedWebUploadExtension(filename string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(filename))) {
	case ".txt", ".md", ".json":
		return true
	default:
		return false
	}
}

func (s *WebIntakeService) recordFailureAudit(ctx context.Context, actor, action, message string, metadata map[string]any) {
	_ = s.recordAudit(ctx, AuditLogCreateInput{
		Actor:    actor,
		Action:   action,
		Resource: "workspace_article",
		Result:   "failure",
		Message:  message,
		Metadata: metadata,
	})
}

func (s *WebIntakeService) recordAudit(ctx context.Context, input AuditLogCreateInput) error {
	service := NewAuditLogService(s.audit)
	if _, err := service.Create(ctx, input); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}

func (s *WebIntakeService) logAuditFailure(action, resourceID string, err error) {
	log.Printf("web intake audit log failure action=%s resource_id=%s err=%v", action, resourceID, err)
}

func webIntakeMetadata(sourceProfile string) map[string]any {
	return map[string]any{
		"target_type":             defaultWebIntakeTargetType,
		"source_profile":          strings.TrimSpace(sourceProfile),
		"render_platform":         defaultWebIntakeRenderPlatform,
		"rewrite_profile_version": defaultWebIntakeRewriteProfileVersion,
	}
}

func webIntakeWorkspaceMetadata(body string, metadata map[string]any) map[string]any {
	workspaceMetadata := map[string]any{}
	for key, value := range metadata {
		workspaceMetadata[key] = value
	}
	workspaceMetadata["source_body"] = body
	return workspaceMetadata
}

func browserPasteOriginalURL(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "browser://paste"
	}
	return "browser://paste/" + title
}

func browserUploadOriginalURL(filename string) string {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return "browser://upload"
	}
	return "browser://upload/" + filename
}
