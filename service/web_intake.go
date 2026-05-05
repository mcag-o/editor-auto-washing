package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
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
	repo  repo.SourceDocumentRepo
	audit repo.AuditLogRepo
}

func NewWebIntakeService(repo repo.SourceDocumentRepo, audit repo.AuditLogRepo) *WebIntakeService {
	return &WebIntakeService{repo: repo, audit: audit}
}

func (s *WebIntakeService) CreateFromPaste(ctx context.Context, input CreatePasteIntakeInput) (*domain.SourceDocument, error) {
	if s.repo == nil || s.audit == nil {
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

	doc := domain.NewSourceDocument("paste", "paste", "txt", title, body, hashParsedSourceDocument(&ParsedSourceDocument{
		Title: title,
		Body:  body,
	}))
	doc.SourceType = "paste"
	doc.Status = domain.SourceDocumentStatusPending

	if err := s.repo.Create(ctx, doc); err != nil {
		wrapped := fmt.Errorf("create source document: %w", err)
		s.recordFailureAudit(ctx, input.Actor, "web_intake.create_from_paste", wrapped.Error(), map[string]any{"title": title})
		return nil, wrapped
	}

	if err := s.recordAudit(ctx, AuditLogCreateInput{
		Actor:      input.Actor,
		Action:     "web_intake.create_from_paste",
		Resource:   "source_document",
		ResourceID: doc.ID,
		Result:     "success",
		Message:    "created source document from pasted text",
		Metadata: map[string]any{
			"title": title,
		},
	}); err != nil {
		s.logAuditFailure("web_intake.create_from_paste", doc.ID, err)
	}

	return doc, nil
}

func (s *WebIntakeService) CreateFromUpload(ctx context.Context, input CreateUploadIntakeInput) (*domain.SourceDocument, error) {
	if s.repo == nil || s.audit == nil {
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

	doc := domain.NewSourceDocument(
		filename,
		filename,
		strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), "."),
		parsed.Title,
		parsed.Body,
		hashParsedSourceDocument(parsed),
	)
	doc.SourceType = "upload"
	doc.Summary = parsed.Summary
	doc.Metadata = sourceDocumentMetadata(parsed)
	if doc.Metadata == nil {
		doc.Metadata = map[string]any{}
	}
	if contentType := strings.TrimSpace(input.ContentType); contentType != "" {
		doc.Metadata["content_type"] = contentType
	}
	doc.Status = domain.SourceDocumentStatusPending

	if err := s.repo.Create(ctx, doc); err != nil {
		wrapped := fmt.Errorf("create source document: %w", err)
		s.recordFailureAudit(ctx, input.Actor, "web_intake.create_from_upload", wrapped.Error(), map[string]any{"filename": filename})
		return nil, wrapped
	}

	if err := s.recordAudit(ctx, AuditLogCreateInput{
		Actor:      input.Actor,
		Action:     "web_intake.create_from_upload",
		Resource:   "source_document",
		ResourceID: doc.ID,
		Result:     "success",
		Message:    "created source document from browser upload",
		Metadata: map[string]any{
			"filename":  filename,
			"file_type": doc.FileType,
		},
	}); err != nil {
		s.logAuditFailure("web_intake.create_from_upload", doc.ID, err)
	}

	return doc, nil
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
		Resource: "source_document",
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
