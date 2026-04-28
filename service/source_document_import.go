package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SourceDocumentImportService struct {
	repo       repo.SourceDocumentRepo
	archiveDir string
}

func NewSourceDocumentImportService(repo repo.SourceDocumentRepo, archiveDir string) *SourceDocumentImportService {
	return &SourceDocumentImportService{repo: repo, archiveDir: strings.TrimSpace(archiveDir)}
}

func (s *SourceDocumentImportService) ImportFile(ctx context.Context, path string) (*domain.SourceDocument, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, domain.NewValidationErr("source document path is required", nil)
	}
	if s.repo == nil {
		return nil, domain.NewInternalErr("source document import service is not configured", nil)
	}
	if strings.TrimSpace(s.archiveDir) == "" {
		return nil, domain.NewInternalErr("source document archive directory is not configured", nil)
	}

	parsed, err := ParseSourceDocument(path)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	doc := domain.NewSourceDocument(
		filepath.Base(path),
		path,
		strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."),
		parsed.Title,
		parsed.Body,
		hashParsedSourceDocument(parsed),
	)
	doc.Summary = parsed.Summary
	doc.Metadata = sourceDocumentMetadata(parsed)
	doc.ImportedAt = &now
	doc.Status = domain.SourceDocumentStatusImported

	if err := s.repo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("create source document: %w", err)
	}

	archivedPath, err := archiveSourceDocument(path, s.archiveDir)
	if err != nil {
		return nil, fmt.Errorf("archive source document: %w", err)
	}

	doc.ArchivedPath = archivedPath
	doc.Status = domain.SourceDocumentStatusPending
	if err := s.repo.Update(ctx, doc); err != nil {
		return nil, s.handleArchiveStateDivergence(ctx, doc, err)
	}

	return doc, nil
}

func (s *SourceDocumentImportService) handleArchiveStateDivergence(ctx context.Context, doc *domain.SourceDocument, updateErr error) error {
	doc.Status = domain.SourceDocumentStatusImportDiverged
	doc.ErrorSummary = fmt.Sprintf("archive succeeded but pending update failed: %v", updateErr)
	if err := s.repo.Update(ctx, doc); err != nil {
		return errors.Join(
			fmt.Errorf("source document archive state diverged after move to %s: pending update failed: %w", doc.ArchivedPath, updateErr),
			fmt.Errorf("failed to persist divergence state: %w", err),
		)
	}
	return fmt.Errorf("source document archive state diverged after move to %s: pending update failed: %w", doc.ArchivedPath, updateErr)
}

func hashParsedSourceDocument(parsed *ParsedSourceDocument) string {
	checksum := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(parsed.Title),
		parsed.Body,
		strings.TrimSpace(parsed.Summary),
		strings.Join(parsed.Tags, "\n"),
	}, "\n")))
	return hex.EncodeToString(checksum[:])
}

func sourceDocumentMetadata(parsed *ParsedSourceDocument) map[string]any {
	metadata := map[string]any{}
	if len(parsed.Tags) > 0 {
		metadata["tags"] = append([]string(nil), parsed.Tags...)
	}
	return metadata
}

func archiveSourceDocument(path, archiveDir string) (string, error) {
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return "", fmt.Errorf("create archive dir: %w", err)
	}
	targetPath := filepath.Join(archiveDir, archivedFilename(path))
	if err := os.Rename(path, targetPath); err != nil {
		return "", err
	}
	return targetPath, nil
}

func archivedFilename(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	checksum := sha256.Sum256([]byte(path))
	shortHash := hex.EncodeToString(checksum[:])[:12]
	if ext == "" {
		return name + "." + shortHash
	}
	return name + "." + shortHash + ext
}
