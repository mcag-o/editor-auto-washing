package domain

import (
	"content-hub/pkg/id"
	"strings"
	"time"
)

const (
	SourceDocumentStatusDiscovered     = "discovered"
	SourceDocumentStatusImported       = "imported"
	SourceDocumentStatusImportDiverged = "import_diverged"
	SourceDocumentStatusPending        = "pending"
	SourceDocumentStatusClaimed        = "claimed"
	SourceDocumentStatusProcessing     = "processing"
	SourceDocumentStatusPaused         = "paused"
	SourceDocumentStatusCompleted      = "completed"
	SourceDocumentStatusFailed         = "failed"
)

var validSourceDocumentStatuses = map[string]struct{}{
	SourceDocumentStatusDiscovered:     {},
	SourceDocumentStatusImported:       {},
	SourceDocumentStatusImportDiverged: {},
	SourceDocumentStatusPending:        {},
	SourceDocumentStatusClaimed:        {},
	SourceDocumentStatusProcessing:     {},
	SourceDocumentStatusPaused:         {},
	SourceDocumentStatusCompleted:      {},
	SourceDocumentStatusFailed:         {},
}

type SourceDocument struct {
	ID                  string         `json:"id"`
	SourceType          string         `json:"source_type"`
	OriginalFilename    string         `json:"original_filename"`
	OriginalPath        string         `json:"original_path"`
	ArchivedPath        string         `json:"archived_path"`
	FileType            string         `json:"file_type"`
	Title               string         `json:"title"`
	Body                string         `json:"body"`
	Summary             string         `json:"summary"`
	Metadata            map[string]any `json:"metadata"`
	Hash                string         `json:"hash"`
	ImportedAt          *time.Time     `json:"imported_at"`
	Status              string         `json:"status"`
	WorkspaceArticleID  string         `json:"workspace_article_id"`
	RewriteRunID        string         `json:"rewrite_run_id"`
	ClaimedBy           string         `json:"claimed_by"`
	ClaimedAt           *time.Time     `json:"claimed_at"`
	ProcessingStartedAt *time.Time     `json:"processing_started_at"`
	CompletedAt         *time.Time     `json:"completed_at"`
	ErrorSummary        string         `json:"error_summary"`
}

func NewSourceDocument(filename, originalPath, fileType, title, body, hash string) *SourceDocument {
	return &SourceDocument{
		ID:               id.New(),
		SourceType:       "folder",
		OriginalFilename: strings.TrimSpace(filename),
		OriginalPath:     strings.TrimSpace(originalPath),
		FileType:         strings.TrimSpace(fileType),
		Title:            strings.TrimSpace(title),
		Body:             body,
		Metadata:         map[string]any{},
		Hash:             strings.TrimSpace(hash),
		Status:           SourceDocumentStatusDiscovered,
	}
}

func (d SourceDocument) Validate() error {
	if strings.TrimSpace(d.SourceType) == "" {
		return NewValidationErr("source type is required", nil)
	}
	if strings.TrimSpace(d.OriginalFilename) == "" {
		return NewValidationErr("original filename is required", nil)
	}
	if strings.TrimSpace(d.OriginalPath) == "" {
		return NewValidationErr("original path is required", nil)
	}
	if strings.TrimSpace(d.FileType) == "" {
		return NewValidationErr("file type is required", nil)
	}
	status := strings.TrimSpace(d.Status)
	if status == "" {
		return NewValidationErr("status is required", nil)
	}
	if _, ok := validSourceDocumentStatuses[status]; !ok {
		return NewValidationErr("unsupported source document status", nil)
	}
	if status != SourceDocumentStatusDiscovered {
		if strings.TrimSpace(d.Title) == "" {
			return NewValidationErr("title is required", nil)
		}
		if strings.TrimSpace(d.Body) == "" {
			return NewValidationErr("body is required", nil)
		}
		if strings.TrimSpace(d.Hash) == "" {
			return NewValidationErr("hash is required", nil)
		}
	}
	switch status {
	case SourceDocumentStatusClaimed:
		if strings.TrimSpace(d.ClaimedBy) == "" {
			return NewValidationErr("claimed by is required", nil)
		}
		if d.ClaimedAt == nil {
			return NewValidationErr("claimed at is required", nil)
		}
	case SourceDocumentStatusProcessing:
		if d.ProcessingStartedAt == nil {
			return NewValidationErr("processing started at is required", nil)
		}
	case SourceDocumentStatusPaused:
		if d.ProcessingStartedAt == nil {
			return NewValidationErr("processing started at is required", nil)
		}
	case SourceDocumentStatusCompleted:
		if d.CompletedAt == nil {
			return NewValidationErr("completed at is required", nil)
		}
	case SourceDocumentStatusFailed:
		if strings.TrimSpace(d.ErrorSummary) == "" {
			return NewValidationErr("error summary is required", nil)
		}
	case SourceDocumentStatusImportDiverged:
		if strings.TrimSpace(d.ErrorSummary) == "" {
			return NewValidationErr("error summary is required", nil)
		}
	}
	return nil
}
