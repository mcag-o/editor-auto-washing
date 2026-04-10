package service

import (
	"content-hub/domain"
	"content-hub/infra/filesystem"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/pkg/id"
	"content-hub/pkg/repo"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type IngestionPipelineService struct {
	ingestionRepo repo.IngestionRepo
	workspaceRepo repo.WorkspaceRepo
	loader        *workspaceinfra.Loader
	router        bundleRouter
	beginImportTx func(ctx context.Context) (repo.BundleImportTx, error)
}

type bundleRouter interface {
	RouteToProcessed(sourcePath, processedDir string) (string, error)
	RouteToFailed(sourcePath, failedDir string) (string, error)
}

func NewIngestionPipelineService(ingestionRepo repo.IngestionRepo, workspaceRepo repo.WorkspaceRepo, txStarter repo.BundleImportTxStarter, loader *workspaceinfra.Loader) *IngestionPipelineService {
	svc := &IngestionPipelineService{
		ingestionRepo: ingestionRepo,
		workspaceRepo: workspaceRepo,
		loader:        loader,
		router:        filesystem.NewBundleRouter(),
	}
	if txStarter != nil {
		svc.beginImportTx = txStarter.BeginBundleImport
	}
	return svc
}

func (s *IngestionPipelineService) ImportIncoming(ctx context.Context, workspaceRoot string) (*domain.IngestionRunResult, error) {
	if s.beginImportTx == nil {
		return s.importWithoutTransactions(ctx, workspaceRoot)
	}
	resolved, err := s.loader.Resolve(workspaceRoot)
	if err != nil {
		return nil, err
	}
	return s.importFromDir(ctx, resolved.Paths.AutomationIncomingDir, resolved.Paths.AutomationProcessedDir, resolved.Paths.AutomationFailedDir, "incoming")
}

func (s *IngestionPipelineService) RetryFailed(ctx context.Context, workspaceRoot string) (*domain.IngestionRunResult, error) {
	if s.beginImportTx == nil {
		return nil, domain.NewInternalErr("transactional bundle import support is required", nil)
	}
	resolved, err := s.loader.Resolve(workspaceRoot)
	if err != nil {
		return nil, err
	}
	return s.importFromDir(ctx, resolved.Paths.AutomationFailedDir, resolved.Paths.AutomationProcessedDir, resolved.Paths.AutomationFailedDir, "failed")
}

func (s *IngestionPipelineService) ListRecords(ctx context.Context, status string) ([]domain.IngestionRecord, error) {
	return s.ingestionRepo.List(ctx, status)
}

func (s *IngestionPipelineService) ListWorkspaceItems(ctx context.Context, status string) ([]domain.ArticleWorkspaceRecord, error) {
	if status == "" {
		return s.workspaceRepo.List(ctx, nil)
	}
	return s.workspaceRepo.List(ctx, &status)
}

func (s *IngestionPipelineService) GetStatus(ctx context.Context, ingestionID string) (*domain.IngestionStatusView, error) {
	record, err := s.ingestionRepo.GetByID(ctx, ingestionID)
	if err != nil {
		return nil, err
	}
	articles, err := s.workspaceRepo.ListByIngestionID(ctx, ingestionID)
	if err != nil {
		return nil, err
	}
	return &domain.IngestionStatusView{Record: *record, Articles: articles}, nil
}

func (s *IngestionPipelineService) importFromDir(ctx context.Context, sourceDir, processedDir, failedDir, origin string) (*domain.IngestionRunResult, error) {
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		return nil, fmt.Errorf("prepare source dir: %w", err)
	}
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("read bundle dir: %w", err)
	}
	files := []string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)

	result := &domain.IngestionRunResult{ScannedFiles: len(files), FileResults: []domain.IngestionFileResult{}}
	for _, fileName := range files {
		filePath := filepath.Join(sourceDir, fileName)
		fileResult := s.importFile(ctx, filePath, processedDir, failedDir, origin)
		result.FileResults = append(result.FileResults, fileResult)
		if fileResult.Status == domain.IngestionStatusImported {
			result.ImportedFiles++
			result.TotalImportedItems += fileResult.ImportedItems
			result.TotalCreatedArticles += fileResult.CreatedArticles
		} else if fileResult.Status == domain.IngestionStatusRoutingFailed {
			result.RoutingFailedFiles++
			result.TotalImportedItems += fileResult.ImportedItems
			result.TotalCreatedArticles += fileResult.CreatedArticles
		} else if fileResult.Status == domain.IngestionStatusFailed {
			result.FailedFiles++
		} else {
			result.FailedFiles++
		}
	}
	return result, nil
}

func (s *IngestionPipelineService) importFile(ctx context.Context, filePath, processedDir, failedDir, origin string) domain.IngestionFileResult {
	now := time.Now().UTC()
	record := &domain.IngestionRecord{
		ID:               id.New(),
		SourceType:       "bundle",
		BundleFile:       filepath.Base(filePath),
		OriginalLocation: origin,
		Status:           domain.IngestionStatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
		Retried:          origin == "failed",
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		record.Status = domain.IngestionStatusFailed
		record.ErrorMessage = err.Error()
		record.Payload = []byte("{}")
		record.RoutedPath, _ = s.router.RouteToFailed(filePath, failedDir)
		_ = s.ingestionRepo.Record(ctx, record)
		return domain.IngestionFileResult{RecordID: record.ID, FileName: record.BundleFile, Status: record.Status, ErrorMessage: record.ErrorMessage, OriginalLocation: record.OriginalLocation, RoutedPath: record.RoutedPath}
	}
	record.Payload = data

	bundle, err := parseBundle(data)
	if err != nil {
		record.Status = domain.IngestionStatusFailed
		record.ErrorMessage = err.Error()
		record.RoutedPath, _ = s.router.RouteToFailed(filePath, failedDir)
		_ = s.ingestionRepo.Record(ctx, record)
		return domain.IngestionFileResult{RecordID: record.ID, FileName: record.BundleFile, Status: record.Status, ErrorMessage: record.ErrorMessage, OriginalLocation: record.OriginalLocation, RoutedPath: record.RoutedPath}
	}

	createdArticles := 0
	var stagedArticles []*domain.ArticleWorkspaceRecord
	for idx, item := range bundle.Items {
		itemID := item.URL
		if itemID == "" {
			itemID = fmt.Sprintf("%s-item-%d", record.ID, idx)
		}
		article := domain.NewArticleWorkspaceRecord(itemID, fallbackString(item.Title, item.URL), item.Summary, domain.ArticleWorkspaceSource{IngestionID: record.ID, BundleFile: record.BundleFile, SourceType: item.SourceType, Platform: fallbackString(item.Platform, item.CanonicalPlatform), URL: item.URL}, map[string]any{"author": item.Author, "publish_time": item.PublishTime, "category": item.Category, "tags": item.Tags})
		stagedArticles = append(stagedArticles, article)
	}
	record.ImportedItems = len(bundle.Items)
	record.CreatedArticles = len(stagedArticles)
	record.Status = domain.IngestionStatusImported

	createdArticles, err = s.persistImportedBundle(ctx, record, stagedArticles, filePath, processedDir, failedDir)
	if err != nil {
		if routingErr, ok := err.(routingFailureErr); ok {
			record.Status = domain.IngestionStatusRoutingFailed
			record.ErrorMessage = routingErr.Error()
			record.CreatedArticles = record.ImportedItems
			return domain.IngestionFileResult{RecordID: record.ID, FileName: record.BundleFile, Status: record.Status, ImportedItems: record.ImportedItems, CreatedArticles: record.CreatedArticles, ErrorMessage: record.ErrorMessage, OriginalLocation: record.OriginalLocation, RoutedPath: record.RoutedPath}
		}
		record.Status = domain.IngestionStatusFailed
		record.ErrorMessage = err.Error()
		record.CreatedArticles = 0
		failedPath, routeErr := s.router.RouteToFailed(filePath, failedDir)
		if routeErr != nil {
			record.ErrorMessage = fmt.Sprintf("%s; failed to route bundle to failed dir: %v", record.ErrorMessage, routeErr)
			record.RoutedPath = filePath
		} else {
			record.RoutedPath = failedPath
		}
		_ = s.ingestionRepo.Record(ctx, record)
		return domain.IngestionFileResult{RecordID: record.ID, FileName: record.BundleFile, Status: record.Status, ImportedItems: record.ImportedItems, CreatedArticles: 0, ErrorMessage: record.ErrorMessage, OriginalLocation: record.OriginalLocation, RoutedPath: record.RoutedPath}
	}
	record.Status = domain.IngestionStatusImported
	record.CreatedArticles = createdArticles
	return domain.IngestionFileResult{RecordID: record.ID, FileName: record.BundleFile, Status: record.Status, ImportedItems: record.ImportedItems, CreatedArticles: record.CreatedArticles, ErrorMessage: record.ErrorMessage, OriginalLocation: record.OriginalLocation, RoutedPath: record.RoutedPath}
}

func (s *IngestionPipelineService) importWithoutTransactions(ctx context.Context, workspaceRoot string) (*domain.IngestionRunResult, error) {
	resolved, err := s.loader.Resolve(workspaceRoot)
	if err != nil {
		return nil, err
	}
	return s.importFromDirWithoutTransactions(ctx, resolved.Paths.AutomationIncomingDir, resolved.Paths.AutomationFailedDir, "incoming")
}

func (s *IngestionPipelineService) importFromDirWithoutTransactions(ctx context.Context, sourceDir, failedDir, origin string) (*domain.IngestionRunResult, error) {
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		return nil, fmt.Errorf("prepare source dir: %w", err)
	}
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("read bundle dir: %w", err)
	}
	result := &domain.IngestionRunResult{FileResults: []domain.IngestionFileResult{}}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		result.ScannedFiles++
		filePath := filepath.Join(sourceDir, entry.Name())
		now := time.Now().UTC()
		record := &domain.IngestionRecord{
			ID:               id.New(),
			SourceType:       "bundle",
			BundleFile:       entry.Name(),
			OriginalLocation: origin,
			Status:           domain.IngestionStatusFailed,
			ErrorMessage:     "transactional bundle import support is required",
			Payload:          []byte("{}"),
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		failedPath, routeErr := s.router.RouteToFailed(filePath, failedDir)
		if routeErr != nil {
			record.ErrorMessage = fmt.Sprintf("%s; failed to route bundle to failed dir: %v", record.ErrorMessage, routeErr)
			record.RoutedPath = filePath
		} else {
			record.RoutedPath = failedPath
		}
		_ = s.ingestionRepo.Record(ctx, record)
		result.FailedFiles++
		result.FileResults = append(result.FileResults, domain.IngestionFileResult{RecordID: record.ID, FileName: record.BundleFile, Status: record.Status, ErrorMessage: record.ErrorMessage, OriginalLocation: record.OriginalLocation, RoutedPath: record.RoutedPath})
	}
	return result, nil
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func parseBundle(data []byte) (domain.IngestionBundle, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return domain.IngestionBundle{}, err
	}
	items, ok := raw["items"]
	if !ok {
		return domain.IngestionBundle{}, fmt.Errorf("bundle must include a list field: items")
	}
	if _, ok := items.([]any); !ok {
		return domain.IngestionBundle{}, fmt.Errorf("bundle must include a list field: items")
	}
	var bundle domain.IngestionBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return domain.IngestionBundle{}, err
	}
	return bundle, nil
}

func (s *IngestionPipelineService) persistImportedBundle(ctx context.Context, record *domain.IngestionRecord, articles []*domain.ArticleWorkspaceRecord, filePath, processedDir, failedDir string) (int, error) {
	if s.beginImportTx == nil {
		return 0, domain.NewInternalErr("transactional bundle import support is required", nil)
	}
	tx, err := s.beginImportTx(ctx)
	if err != nil {
		return 0, err
	}
	return s.persistWithTx(ctx, tx, record, articles, filePath, processedDir, failedDir)
}

func (s *IngestionPipelineService) persistWithTx(ctx context.Context, tx repo.BundleImportTx, record *domain.IngestionRecord, articles []*domain.ArticleWorkspaceRecord, filePath, processedDir, failedDir string) (int, error) {
	finalized := false
	rollback := func() {
		if !finalized {
			_ = tx.Rollback()
			finalized = true
		}
	}
	defer func() {
		if !finalized {
			_ = tx.Rollback()
		}
	}()
	for _, article := range articles {
		if err := tx.CreateWorkspaceArticle(ctx, article); err != nil {
			return 0, err
		}
	}
	processedPath, err := s.router.RouteToProcessed(filePath, processedDir)
	if err != nil {
		routingFailedDir := filepath.Join(processedDir, "routing_failed")
		failedPath, routeErr := s.router.RouteToFailed(filePath, routingFailedDir)
		if routeErr != nil {
			return 0, fmt.Errorf("%s; failed to route bundle to routing-failed dir before durable record: %v", err.Error(), routeErr)
		}
		rollback()
		record.Status = domain.IngestionStatusRoutingFailed
		record.RoutedPath = failedPath
		record.ErrorMessage = err.Error()
		if recordErr := s.ingestionRepo.Record(ctx, record); recordErr != nil {
			return 0, fmt.Errorf("record routing failure after processed routing error: %w", recordErr)
		}
		return 0, routingFailureErr{message: record.ErrorMessage}
	}
	record.RoutedPath = processedPath
	record.Status = domain.IngestionStatusImported
	if err := tx.RecordIngestion(ctx, record); err != nil {
		rollback()
		failedPath, routeErr := s.router.RouteToFailed(processedPath, failedDir)
		if routeErr != nil {
			return 0, fmt.Errorf("record imported bundle after processed routing: %v; failed to route processed bundle to failed dir: %v", err, routeErr)
		}
		record.Status = domain.IngestionStatusFailed
		record.RoutedPath = failedPath
		record.ErrorMessage = fmt.Sprintf("record imported bundle after processed routing: %v", err)
		_ = s.ingestionRepo.Record(ctx, record)
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		finalized = true
		failedPath, routeErr := s.router.RouteToFailed(processedPath, failedDir)
		if routeErr != nil {
			return 0, fmt.Errorf("commit imported bundle after processed routing: %v; failed to route processed bundle to failed dir: %v", err, routeErr)
		}
		record.Status = domain.IngestionStatusFailed
		record.RoutedPath = failedPath
		record.ErrorMessage = fmt.Sprintf("commit imported bundle after processed routing: %v", err)
		_ = s.ingestionRepo.Record(ctx, record)
		return 0, err
	}
	finalized = true
	return len(articles), nil
}

type routingFailureErr struct{ message string }

func (e routingFailureErr) Error() string { return e.message }
