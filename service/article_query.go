package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"sort"
	"strings"
	"time"
)

type ArticleQueryFilter struct {
	Status string
	Limit  int
}

type BrowserArticle struct {
	ID                  string         `json:"id"`
	WorkspaceArticleID  string         `json:"workspace_article_id"`
	Title               string         `json:"title"`
	Summary             string         `json:"summary"`
	Body                string         `json:"body"`
	Status              string         `json:"status"`
	SourceType          string         `json:"source_type"`
	OriginalFilename    string         `json:"original_filename"`
	OriginalPath        string         `json:"original_path"`
	FileType            string         `json:"file_type"`
	RewriteRunID        string         `json:"rewrite_run_id"`
	WorkflowRunID       string         `json:"workflow_run_id"`
	ImportedAt          *time.Time     `json:"imported_at"`
	ProcessingStartedAt *time.Time     `json:"processing_started_at"`
	CompletedAt         *time.Time     `json:"completed_at"`
	ErrorSummary        string         `json:"error_summary"`
	Metadata            map[string]any `json:"metadata"`
}

type ArticleQueryService struct {
	workspaces repo.WorkspaceRepo
	runs       repo.RewritePipelineRunRepo
	workflows  repo.WorkflowRunRepo
}

func NewBrowserArticleQueryService(workspaces repo.WorkspaceRepo, runs repo.RewritePipelineRunRepo, workflows repo.WorkflowRunRepo) *ArticleQueryService {
	return &ArticleQueryService{workspaces: workspaces, runs: runs, workflows: workflows}
}

func (s *ArticleQueryService) ListArticles(ctx context.Context, filter ArticleQueryFilter) ([]BrowserArticle, error) {
	if s == nil || s.workspaces == nil {
		return nil, domain.NewInternalErr("article query service is not configured", nil)
	}
	status := strings.TrimSpace(filter.Status)
	var statusFilter *string
	if status != "" && status != domain.ArticleWorkspaceStatusPaused {
		statusFilter = &status
	}
	items, err := s.workspaces.List(ctx, statusFilter)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return strings.TrimSpace(items[i].ID) > strings.TrimSpace(items[j].ID)
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	out := make([]BrowserArticle, 0, len(items))
	for _, item := range items {
		article, err := s.browserArticleFromWorkspace(ctx, item)
		if err != nil {
			return nil, err
		}
		if status != "" && strings.TrimSpace(article.Status) != status {
			continue
		}
		out = append(out, *article)
	}
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

func (s *ArticleQueryService) GetArticle(ctx context.Context, id string) (*BrowserArticle, error) {
	if s == nil || s.workspaces == nil {
		return nil, domain.NewInternalErr("article query service is not configured", nil)
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, domain.NewValidationErr("article id is required", nil)
	}
	workspace, err := s.workspaces.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.browserArticleFromWorkspace(ctx, *workspace)
}

func (s *ArticleQueryService) WorkspaceRepo() repo.WorkspaceRepo {
	if s == nil {
		return nil
	}
	return s.workspaces
}

func (s *ArticleQueryService) WorkflowRunRepo() repo.WorkflowRunRepo {
	if s == nil {
		return nil
	}
	return s.workflows
}

func (s *ArticleQueryService) browserArticleFromWorkspace(ctx context.Context, workspace domain.ArticleWorkspaceRecord) (*BrowserArticle, error) {
	article := &BrowserArticle{
		ID:                 strings.TrimSpace(workspace.ID),
		WorkspaceArticleID: strings.TrimSpace(workspace.ID),
		Title:              strings.TrimSpace(workspace.Title),
		Summary:            strings.TrimSpace(workspace.Summary),
		Body:               browserWorkspaceBody(workspace.Metadata),
		Status:             strings.TrimSpace(workspace.Status),
		SourceType:         browserWorkspaceSourceType(workspace),
		OriginalFilename:   browserWorkspaceOriginalFilename(workspace),
		OriginalPath:       browserWorkspaceOriginalPath(workspace),
		FileType:           browserWorkspaceFileType(workspace),
		Metadata:           cloneBrowserArticleMetadata(workspace.Metadata),
	}
	importedAt := workspace.CreatedAt.UTC()
	article.ImportedAt = &importedAt
	article.CompletedAt = browserWorkspaceCompletedAt(workspace)

	if run := s.latestRewriteRunForWorkspace(ctx, workspace.ID); run != nil {
		article.RewriteRunID = strings.TrimSpace(run.ID)
		article.ErrorSummary = strings.TrimSpace(run.ErrorSummary)
		if !run.StartedAt.IsZero() {
			startedAt := run.StartedAt.UTC()
			article.ProcessingStartedAt = &startedAt
		}
		if run.CompletedAt != nil {
			completed := run.CompletedAt.UTC()
			article.CompletedAt = &completed
		}
	}
	if workflow := s.latestWorkflowRunForWorkspace(ctx, workspace.ID); workflow != nil {
		mergeWorkflowRunStatus(article, workflow)
	}
	return article, nil
}

func (s *ArticleQueryService) latestRewriteRunForWorkspace(ctx context.Context, workspaceID string) *domain.RewritePipelineRun {
	if s == nil || s.runs == nil || strings.TrimSpace(workspaceID) == "" {
		return nil
	}
	runs, err := s.runs.List(ctx, 0)
	if err != nil {
		return nil
	}
	var selected *domain.RewritePipelineRun
	for i := range runs {
		run := runs[i]
		if strings.TrimSpace(run.WorkspaceArticleID) != strings.TrimSpace(workspaceID) {
			continue
		}
		if selected == nil || rewriteRunSortTime(run).After(rewriteRunSortTime(*selected)) || (rewriteRunSortTime(run).Equal(rewriteRunSortTime(*selected)) && strings.TrimSpace(run.ID) > strings.TrimSpace(selected.ID)) {
			copyRun := run
			selected = &copyRun
		}
	}
	return selected
	}

func (s *ArticleQueryService) latestWorkflowRunForWorkspace(ctx context.Context, workspaceID string) *domain.WorkflowRun {
	if s == nil || s.workflows == nil || strings.TrimSpace(workspaceID) == "" {
		return nil
	}
	runs, err := s.workflows.List(ctx, 0)
	if err != nil {
		return nil
	}
	var selected *domain.WorkflowRun
	for i := range runs {
		run := runs[i]
		if strings.TrimSpace(run.WorkspaceArticleID) != strings.TrimSpace(workspaceID) {
			continue
		}
		if selected == nil || workflowRunSortTime(run).After(workflowRunSortTime(*selected)) || (workflowRunSortTime(run).Equal(workflowRunSortTime(*selected)) && strings.TrimSpace(run.ID) > strings.TrimSpace(selected.ID)) {
			copyRun := run
			selected = &copyRun
		}
	}
	return selected
}

func rewriteRunSortTime(run domain.RewritePipelineRun) time.Time {
	if run.CompletedAt != nil {
		return run.CompletedAt.UTC()
	}
	if !run.StartedAt.IsZero() {
		return run.StartedAt.UTC()
	}
	return time.Time{}
}

func workflowRunSortTime(run domain.WorkflowRun) time.Time {
	if run.CompletedAt != nil {
		return run.CompletedAt.UTC()
	}
	if !run.StartedAt.IsZero() {
		return run.StartedAt.UTC()
	}
	return time.Time{}
}

func browserWorkspaceBody(metadata map[string]any) string {
	value, _ := metadata["source_body"].(string)
	return value
}

func browserWorkspaceOriginalFilename(workspace domain.ArticleWorkspaceRecord) string {
	if value, ok := workspace.Metadata["browser_projection_filename"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if strings.TrimSpace(workspace.Source.SourceType) == "upload" {
		url := strings.TrimSpace(workspace.Source.URL)
		if idx := strings.LastIndex(url, "/"); idx >= 0 && idx+1 < len(url) {
			return url[idx+1:]
		}
	}
	return "browser-article.txt"
}

func browserWorkspaceFileType(workspace domain.ArticleWorkspaceRecord) string {
	name := browserWorkspaceOriginalFilename(workspace)
	if idx := strings.LastIndex(name, "."); idx >= 0 && idx+1 < len(name) {
		return strings.ToLower(name[idx+1:])
	}
	return "txt"
}

func browserWorkspaceSourceType(workspace domain.ArticleWorkspaceRecord) string {
	if sourceType := strings.TrimSpace(workspace.Source.SourceType); sourceType != "" {
		return sourceType
	}
	return "browser"
}

func browserWorkspaceOriginalPath(workspace domain.ArticleWorkspaceRecord) string {
	if url := strings.TrimSpace(workspace.Source.URL); url != "" {
		return url
	}
	return "browser://article/" + strings.TrimSpace(workspace.ID)
}

func browserWorkspaceCompletedAt(workspace domain.ArticleWorkspaceRecord) *time.Time {
	if len(workspace.LifecycleHistory) == 0 {
		return nil
	}
	status := strings.TrimSpace(workspace.Status)
	switch status {
	case domain.ArticleWorkspaceStatusDraft, domain.ArticleWorkspaceStatusRendered, domain.ArticleWorkspaceStatusReviewPending, domain.ArticleWorkspaceStatusApproved, domain.ArticleWorkspaceStatusReviewRejected, domain.ArticleWorkspaceStatusPublished, domain.ArticleWorkspaceStatusRewriteFailed, domain.ArticleWorkspaceStatusFailed:
		completed := workspace.UpdatedAt.UTC()
		return &completed
	default:
		return nil
	}
}

func mergeWorkflowRunStatus(article *BrowserArticle, run *domain.WorkflowRun) {
	if article == nil || run == nil {
		return
	}
	if run.Status == domain.WorkflowRunPaused {
		article.Status = domain.ArticleWorkspaceStatusPaused
	}
	article.WorkflowRunID = strings.TrimSpace(run.ID)
	if run.CompletedAt != nil {
		completed := run.CompletedAt.UTC()
		article.CompletedAt = &completed
	}
	if strings.TrimSpace(run.ErrorSummary) != "" {
		article.ErrorSummary = strings.TrimSpace(run.ErrorSummary)
	}
	if article.Metadata == nil {
		article.Metadata = map[string]any{}
	}
	article.Metadata["workflow_run_id"] = strings.TrimSpace(run.ID)
	article.Metadata["workflow_run_status"] = strings.TrimSpace(run.Status)
}

func cloneBrowserArticleMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}
