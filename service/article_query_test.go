package service

import (
	"content-hub/domain"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestArticleQueryServiceListsBrowserArticlesByWorkspaceStatus(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)

	imported := domain.NewArticleWorkspaceRecord("workspace-imported", "Imported", "", domain.ArticleWorkspaceSource{
		SourceType: "paste",
		URL:        "browser://paste/imported",
	}, map[string]any{"source_body": "Imported body", "source_profile": "web-paste"})
	rendered := domain.NewArticleWorkspaceRecord("workspace-rendered", "Rendered", "", domain.ArticleWorkspaceSource{
		SourceType: "upload",
		URL:        "browser://upload/rendered.md",
	}, map[string]any{"source_body": "Rendered body", "source_profile": "web-upload"})
	rendered.Status = domain.ArticleWorkspaceStatusRendered
	rendered.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusRewritePending, domain.ArticleWorkspaceStatusRewriting, domain.ArticleWorkspaceStatusDraft, domain.ArticleWorkspaceStatusRendered}
	now := time.Now().UTC()
	rendered.LifecycleHistory = append(rendered.LifecycleHistory,
		domain.ArticleWorkspaceLifecycleEntry{Status: domain.ArticleWorkspaceStatusRewritePending, Notes: "rewrite queued", CreatedAt: now.Add(-3 * time.Minute)},
		domain.ArticleWorkspaceLifecycleEntry{Status: domain.ArticleWorkspaceStatusRewriting, Notes: "rewrite in progress", CreatedAt: now.Add(-2 * time.Minute)},
		domain.ArticleWorkspaceLifecycleEntry{Status: domain.ArticleWorkspaceStatusDraft, Notes: "draft materialized", CreatedAt: now.Add(-time.Minute)},
		domain.ArticleWorkspaceLifecycleEntry{Status: domain.ArticleWorkspaceStatusRendered, Notes: "rendered draft asset", CreatedAt: now},
	)
	rendered.UpdatedAt = now

	require.NoError(t, repos.WorkspaceRepo.Create(t.Context(), imported))
	require.NoError(t, repos.WorkspaceRepo.Create(t.Context(), rendered))

	svc := NewBrowserArticleQueryService(repos.WorkspaceRepo, repos.RewritePipelineRunRepo, repos.WorkflowRunRepo)

	articles, err := svc.ListArticles(t.Context(), ArticleQueryFilter{
		Status: domain.ArticleWorkspaceStatusImported,
		Limit:  1,
	})

	require.NoError(t, err)
	require.Len(t, articles, 1)
	require.Equal(t, imported.ID, articles[0].ID)
	require.Equal(t, domain.ArticleWorkspaceStatusImported, articles[0].Status)
	require.Equal(t, "web-paste", articles[0].Metadata["source_profile"])
}

func TestArticleQueryServiceListsBrowserArticlesWithoutStatusFilter(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)

	imported := domain.NewArticleWorkspaceRecord("workspace-imported", "Imported", "", domain.ArticleWorkspaceSource{
		SourceType: "paste",
		URL:        "browser://paste/imported",
	}, map[string]any{"source_body": "Imported body", "source_profile": "web-paste"})
	failed := domain.NewArticleWorkspaceRecord("workspace-failed", "Failed", "", domain.ArticleWorkspaceSource{
		SourceType: "upload",
		URL:        "browser://upload/failed.md",
	}, map[string]any{"source_body": "Failed body", "source_profile": "web-upload"})
	failed.Status = domain.ArticleWorkspaceStatusRewriteFailed
	failed.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusRewritePending, domain.ArticleWorkspaceStatusRewriting, domain.ArticleWorkspaceStatusRewriteFailed}
	now := time.Now().UTC()
	failed.LifecycleHistory = append(failed.LifecycleHistory,
		domain.ArticleWorkspaceLifecycleEntry{Status: domain.ArticleWorkspaceStatusRewritePending, Notes: "rewrite queued", CreatedAt: now.Add(-3 * time.Minute)},
		domain.ArticleWorkspaceLifecycleEntry{Status: domain.ArticleWorkspaceStatusRewriting, Notes: "rewrite in progress", CreatedAt: now.Add(-2 * time.Minute)},
		domain.ArticleWorkspaceLifecycleEntry{Status: domain.ArticleWorkspaceStatusRewriteFailed, Notes: "rewrite failed", CreatedAt: now.Add(-time.Minute)},
	)
	failed.UpdatedAt = now

	require.NoError(t, repos.WorkspaceRepo.Create(t.Context(), imported))
	require.NoError(t, repos.WorkspaceRepo.Create(t.Context(), failed))

	svc := NewBrowserArticleQueryService(repos.WorkspaceRepo, repos.RewritePipelineRunRepo, repos.WorkflowRunRepo)

	articles, err := svc.ListArticles(t.Context(), ArticleQueryFilter{Limit: 10})

	require.NoError(t, err)
	require.Len(t, articles, 2)
	ids := []string{articles[0].ID, articles[1].ID}
	require.Contains(t, ids, imported.ID)
	require.Contains(t, ids, failed.ID)
}

func TestArticleQueryServiceGetsBrowserArticleDetailByID(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)

	workspace := domain.NewArticleWorkspaceRecord("workspace-detail", "Title", "Summary", domain.ArticleWorkspaceSource{
		SourceType: "upload",
		URL:        "browser://upload/article.json",
	}, map[string]any{
		"source_body":      "Body",
		"source_profile":   "web-upload",
		"workflow_template_id": "workflow-a",
	})
	workspace.Status = domain.ArticleWorkspaceStatusRewriteFailed
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusRewritePending, domain.ArticleWorkspaceStatusRewriting, domain.ArticleWorkspaceStatusRewriteFailed}
	now := time.Now().UTC()
	workspace.LifecycleHistory = append(workspace.LifecycleHistory,
		domain.ArticleWorkspaceLifecycleEntry{Status: domain.ArticleWorkspaceStatusRewritePending, Notes: "rewrite queued", CreatedAt: now.Add(-3 * time.Minute)},
		domain.ArticleWorkspaceLifecycleEntry{Status: domain.ArticleWorkspaceStatusRewriting, Notes: "rewrite in progress", CreatedAt: now.Add(-2 * time.Minute)},
		domain.ArticleWorkspaceLifecycleEntry{Status: domain.ArticleWorkspaceStatusRewriteFailed, Notes: "rewrite failed", CreatedAt: now.Add(-time.Minute)},
	)
	workspace.UpdatedAt = now
	require.NoError(t, repos.WorkspaceRepo.Create(t.Context(), workspace))

	run := domain.NewRewritePipelineRun("profile-1", "v1", workspace.ID, "wechat-longform", "web-upload")
	run.ID = "run-detail"
	run.Status = domain.RewriteRunFailed
	run.ErrorSummary = "stage failed"
	run.CompletedAt = &now
	require.NoError(t, repos.RewritePipelineRunRepo.Create(t.Context(), run))

	svc := NewBrowserArticleQueryService(repos.WorkspaceRepo, repos.RewritePipelineRunRepo, repos.WorkflowRunRepo)

	loaded, err := svc.GetArticle(t.Context(), workspace.ID)

	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, workspace.ID, loaded.ID)
	require.Equal(t, "Title", loaded.Title)
	require.Equal(t, "Body", loaded.Body)
	require.Equal(t, "Summary", loaded.Summary)
	require.Equal(t, domain.ArticleWorkspaceStatusRewriteFailed, loaded.Status)
	require.Equal(t, "web-upload", loaded.Metadata["source_profile"])
	require.Equal(t, run.ID, loaded.RewriteRunID)
}

func TestArticleQueryServiceListsPausedBrowserArticlesFromWorkflowRunState(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)

	workspace := domain.NewArticleWorkspaceRecord("workspace-paused", "Paused", "", domain.ArticleWorkspaceSource{
		SourceType: "paste",
		URL:        "browser://paste/paused",
	}, map[string]any{"source_body": "Paused body", "source_profile": "web-paste"})
	workspace.Status = domain.ArticleWorkspaceStatusFailed
	workspace.StatusHistory = []string{domain.ArticleWorkspaceStatusImported, domain.ArticleWorkspaceStatusRewriting, domain.ArticleWorkspaceStatusFailed}
	require.NoError(t, repos.WorkspaceRepo.Create(t.Context(), workspace))

	run := domain.NewRewritePipelineRun("profile-1", "v1", workspace.ID, "wechat-longform", "web-paste")
	run.ID = "rewrite-paused"
	run.Status = domain.RewriteRunRunning
	require.NoError(t, repos.RewritePipelineRunRepo.Create(t.Context(), run))

	workflowRun, newErr := domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "wf-1", WorkflowVersion: "v1", EntryNodeID: "review", WorkspaceArticleID: workspace.ID})
	require.NoError(t, newErr)
	workflowRun.ID = "workflow-run-paused"
	workflowRun.Status = domain.WorkflowRunPaused
	require.NoError(t, repos.WorkflowRunRepo.Create(t.Context(), workflowRun))

	svc := NewBrowserArticleQueryService(repos.WorkspaceRepo, repos.RewritePipelineRunRepo, repos.WorkflowRunRepo)

	articles, err := svc.ListArticles(t.Context(), ArticleQueryFilter{Status: domain.ArticleWorkspaceStatusPaused, Limit: 10})

	require.NoError(t, err)
	require.Len(t, articles, 1)
	require.Equal(t, workspace.ID, articles[0].ID)
	require.Equal(t, domain.ArticleWorkspaceStatusPaused, articles[0].Status)
	require.Equal(t, workflowRun.ID, articles[0].Metadata["workflow_run_id"])
}

func TestArticleQueryServiceListsPausedBrowserArticlesBeforeLimitSlicing(t *testing.T) {
	repos, cleanup, err := BuildRuntimeRepos(t.TempDir())
	if cleanup != nil {
		defer func() {
			require.NoError(t, cleanup())
		}()
	}
	require.NoError(t, err)

	newestNonPaused := domain.NewArticleWorkspaceRecord("workspace-newest", "Newest", "", domain.ArticleWorkspaceSource{SourceType: "paste", URL: "browser://paste/newest"}, map[string]any{"source_body": "Newest body", "source_profile": "web-paste"})
	newestNonPaused.UpdatedAt = time.Now().UTC()
	require.NoError(t, repos.WorkspaceRepo.Create(t.Context(), newestNonPaused))

	paused := domain.NewArticleWorkspaceRecord("workspace-paused", "Paused", "", domain.ArticleWorkspaceSource{SourceType: "paste", URL: "browser://paste/paused"}, map[string]any{"source_body": "Paused body", "source_profile": "web-paste"})
	paused.Status = domain.ArticleWorkspaceStatusRewriting
	paused.UpdatedAt = time.Now().UTC().Add(-time.Minute)
	require.NoError(t, repos.WorkspaceRepo.Create(t.Context(), paused))

	workflowRun, newErr := domain.NewWorkflowRun(domain.WorkflowRunSpec{WorkflowID: "wf-1", WorkflowVersion: "v1", EntryNodeID: "review", WorkspaceArticleID: paused.ID})
	require.NoError(t, newErr)
	workflowRun.ID = "workflow-run-paused"
	workflowRun.Status = domain.WorkflowRunPaused
	require.NoError(t, repos.WorkflowRunRepo.Create(t.Context(), workflowRun))

	svc := NewBrowserArticleQueryService(repos.WorkspaceRepo, repos.RewritePipelineRunRepo, repos.WorkflowRunRepo)

	articles, err := svc.ListArticles(t.Context(), ArticleQueryFilter{Status: domain.ArticleWorkspaceStatusPaused, Limit: 1})

	require.NoError(t, err)
	require.Len(t, articles, 1)
	require.Equal(t, paused.ID, articles[0].ID)
	require.Equal(t, domain.ArticleWorkspaceStatusPaused, articles[0].Status)
}
