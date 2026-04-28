package service

import (
	"content-hub/domain"
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

type folderSourceImporter interface {
	ImportFile(ctx context.Context, path string) (*domain.SourceDocument, error)
}

type FolderScanner struct {
	importer folderSourceImporter
}

func NewFolderScanner(importer folderSourceImporter) *FolderScanner {
	return &FolderScanner{importer: importer}
}

func (s *FolderScanner) ScanOnce(ctx context.Context, watchDir, archiveDir string) (*domain.ImportRun, error) {
	watchDir = strings.TrimSpace(watchDir)
	archiveDir = strings.TrimSpace(archiveDir)
	if watchDir == "" {
		return nil, domain.NewValidationErr("watch directory is required", nil)
	}
	if s.importer == nil {
		return nil, domain.NewInternalErr("folder scanner is not configured", nil)
	}

	run := domain.NewImportRun("folder")
	run.Status = domain.ImportRunStatusRunning

	var scannedFiles int
	var skippedFiles int
	failedPaths := []string{}

	cleanWatchDir := filepath.Clean(watchDir)
	cleanArchiveDir := ""
	if archiveDir != "" {
		cleanArchiveDir = filepath.Clean(archiveDir)
	}

	err := filepath.WalkDir(cleanWatchDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			if cleanArchiveDir != "" && sameOrUnderPath(path, cleanArchiveDir) {
				return filepath.SkipDir
			}
			return nil
		}

		if cleanArchiveDir != "" && sameOrUnderPath(path, cleanArchiveDir) {
			return nil
		}

		if !isSupportedFolderImportFile(path) {
			skippedFiles++
			return nil
		}

		scannedFiles++
		if _, err := s.importer.ImportFile(ctx, path); err != nil {
			run.FailedCount++
			failedPaths = append(failedPaths, filepath.Base(path))
			return nil
		}

		run.ImportedCount++
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan watch directory: %w", err)
	}

	completedAt := time.Now().UTC()
	run.CompletedAt = &completedAt
	run.Metadata = map[string]any{
		"scanned_files":  scannedFiles,
		"imported_files": run.ImportedCount,
		"skipped_files":  skippedFiles,
		"failed_files":   run.FailedCount,
	}
	if len(failedPaths) > 0 {
		run.Status = domain.ImportRunStatusFailed
		run.ErrorSummary = "failed imports: " + strings.Join(failedPaths, ", ")
	} else {
		run.Status = domain.ImportRunStatusCompleted
	}

	return run, nil
}

func isSupportedFolderImportFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".txt", ".json", ".docx":
		return true
	default:
		return false
	}
}

func sameOrUnderPath(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
