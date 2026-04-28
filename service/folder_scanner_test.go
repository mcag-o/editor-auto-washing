package service

import (
	"content-hub/domain"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubFolderSourceImporter struct {
	importedPaths []string
	errByPath     map[string]error
}

func (s *stubFolderSourceImporter) ImportFile(_ context.Context, path string) (*domain.SourceDocument, error) {
	s.importedPaths = append(s.importedPaths, path)
	if err := s.errByPath[path]; err != nil {
		return nil, err
	}
	return domain.NewSourceDocument(filepath.Base(path), path, "md", "Title", "Body", "hash"), nil
}

func TestFolderScannerImportsSupportedFilesAndSkipsSyncOver(t *testing.T) {
	inbox := t.TempDir()
	syncOver := filepath.Join(inbox, "SyncOver")
	require.NoError(t, os.MkdirAll(syncOver, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(inbox, "article.md"), []byte("# Title\n\nBody"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(syncOver, "old.md"), []byte("ignored"), 0o644))

	importer := &stubFolderSourceImporter{}
	scanner := NewFolderScanner(importer)

	run, err := scanner.ScanOnce(t.Context(), inbox, syncOver)

	require.NoError(t, err)
	require.Equal(t, domain.ImportRunStatusCompleted, run.Status)
	require.Equal(t, 1, run.ImportedCount)
	require.Equal(t, 0, run.FailedCount)
	require.Equal(t, 1, run.Metadata["scanned_files"])
	require.Equal(t, 0, run.Metadata["skipped_files"])
	require.Len(t, importer.importedPaths, 1)
	require.Equal(t, filepath.Join(inbox, "article.md"), importer.importedPaths[0])
}

func TestFolderScannerSkipsSyncOverByDefaultWhenArchiveDirIsEmpty(t *testing.T) {
	inbox := t.TempDir()
	syncOver := filepath.Join(inbox, "SyncOver")
	require.NoError(t, os.MkdirAll(syncOver, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(inbox, "article.md"), []byte("# Title\n\nBody"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(syncOver, "old.md"), []byte("ignored"), 0o644))

	importer := &stubFolderSourceImporter{}
	scanner := NewFolderScanner(importer)

	run, err := scanner.ScanOnce(t.Context(), inbox, "")

	require.NoError(t, err)
	require.Equal(t, domain.ImportRunStatusCompleted, run.Status)
	require.Equal(t, 1, run.ImportedCount)
	require.Equal(t, 0, run.FailedCount)
	require.Equal(t, 1, run.Metadata["scanned_files"])
	require.Equal(t, 0, run.Metadata["skipped_files"])
	require.Len(t, importer.importedPaths, 1)
	require.Equal(t, filepath.Join(inbox, "article.md"), importer.importedPaths[0])
}

func TestFolderScannerTracksSkippedAndFailedFiles(t *testing.T) {
	inbox := t.TempDir()
	syncOver := filepath.Join(inbox, "SyncOver")
	nested := filepath.Join(inbox, "nested")
	require.NoError(t, os.MkdirAll(syncOver, 0o755))
	require.NoError(t, os.MkdirAll(nested, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(inbox, "article.md"), []byte("# Title\n\nBody"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(inbox, "notes.csv"), []byte("ignored"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(nested, "broken.txt"), []byte("broken"), 0o644))

	failingPath := filepath.Join(nested, "broken.txt")
	importer := &stubFolderSourceImporter{errByPath: map[string]error{failingPath: errors.New("boom")}}
	scanner := NewFolderScanner(importer)

	run, err := scanner.ScanOnce(t.Context(), inbox, syncOver)

	require.NoError(t, err)
	require.Equal(t, domain.ImportRunStatusFailed, run.Status)
	require.Equal(t, 1, run.ImportedCount)
	require.Equal(t, 1, run.FailedCount)
	require.Equal(t, 3, run.Metadata["scanned_files"])
	require.Equal(t, 1, run.Metadata["skipped_files"])
	require.Equal(t, 1, run.Metadata["failed_files"])
	require.Equal(t, 1, run.Metadata["imported_files"])
	require.Equal(t, []string{filepath.Join(inbox, "article.md"), failingPath}, importer.importedPaths)
	require.Contains(t, run.ErrorSummary, filepath.Join("nested", "broken.txt"))
}

func TestFolderScannerUnsupportedFileCountsAsScannedAndSkipped(t *testing.T) {
	inbox := t.TempDir()
	syncOver := filepath.Join(inbox, "SyncOver")
	require.NoError(t, os.MkdirAll(syncOver, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(inbox, "notes.csv"), []byte("ignored"), 0o644))

	importer := &stubFolderSourceImporter{}
	scanner := NewFolderScanner(importer)

	run, err := scanner.ScanOnce(t.Context(), inbox, syncOver)

	require.NoError(t, err)
	require.Equal(t, domain.ImportRunStatusCompleted, run.Status)
	require.Equal(t, 1, run.Metadata["scanned_files"])
	require.Equal(t, 1, run.Metadata["skipped_files"])
	require.Equal(t, 0, run.Metadata["imported_files"])
	require.Equal(t, 0, run.Metadata["failed_files"])
	require.Empty(t, importer.importedPaths)
}

func TestFolderScannerReturnsFailedRunWhenWatchDirectoryCannotBeScanned(t *testing.T) {
	watchDir := filepath.Join(t.TempDir(), "missing")
	importer := &stubFolderSourceImporter{}
	scanner := NewFolderScanner(importer)

	run, err := scanner.ScanOnce(t.Context(), watchDir, filepath.Join(watchDir, "SyncOver"))

	require.Error(t, err)
	require.NotNil(t, run)
	require.Equal(t, domain.ImportRunStatusFailed, run.Status)
	require.NotNil(t, run.CompletedAt)
	require.ErrorContains(t, err, "scan watch directory")
	require.Contains(t, run.ErrorSummary, "scan watch directory")
	require.Equal(t, 0, run.Metadata["scanned_files"])
	require.Equal(t, 0, run.Metadata["skipped_files"])
	require.Equal(t, 0, run.Metadata["imported_files"])
	require.Equal(t, 0, run.Metadata["failed_files"])
}
