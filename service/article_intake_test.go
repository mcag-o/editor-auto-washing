package service

import (
	"content-hub/domain"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubArticleIntakeWorkspaceRepo struct {
	created []*domain.ArticleWorkspaceRecord
	err     error
}

func (r *stubArticleIntakeWorkspaceRepo) Create(_ context.Context, record *domain.ArticleWorkspaceRecord) error {
	if r.err != nil {
		return r.err
	}
	copyValue := *record
	r.created = append(r.created, &copyValue)
	return nil
}

func (r *stubArticleIntakeWorkspaceRepo) GetByID(context.Context, string) (*domain.ArticleWorkspaceRecord, error) {
	return nil, domain.NewNotFoundErr("workspace", "missing")
}

func (r *stubArticleIntakeWorkspaceRepo) List(context.Context, *string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}

func (r *stubArticleIntakeWorkspaceRepo) ListByIngestionID(context.Context, string) ([]domain.ArticleWorkspaceRecord, error) {
	return nil, nil
}

func (r *stubArticleIntakeWorkspaceRepo) TransitionStatus(context.Context, string, string, string) error {
	return nil
}

func (r *stubArticleIntakeWorkspaceRepo) Delete(context.Context, string) error {
	return nil
}

type stubArticleIntakeRewriteRunner struct {
	called  bool
	lastReq RewriteRunRequest
	err     error
}

type stubArticleIntakeResumeRewriteRunner struct {
	called        bool
	lastRewriteID string
	lastTitle     string
	result        *domain.RewritePipelineRun
	err           error
}

type stubArticleIntakeWorkflowRepo struct {
	workflow *domain.WorkflowDefinition
	err      error
	gotID    string
}

func (r *stubArticleIntakeWorkflowRepo) GetByID(_ context.Context, id string) (*domain.WorkflowDefinition, error) {
	r.gotID = id
	if r.err != nil {
		return nil, r.err
	}
	if r.workflow == nil {
		return nil, domain.NewNotFoundErr("workflow_definition", id)
	}
	return r.workflow, nil
}

func (r *stubArticleIntakeRewriteRunner) Run(_ context.Context, req RewriteRunRequest) (*domain.RewritePipelineRun, error) {
	r.called = true
	r.lastReq = req
	if r.err != nil {
		return nil, r.err
	}
	return &domain.RewritePipelineRun{ID: "run-1"}, nil
}

func (r *stubArticleIntakeResumeRewriteRunner) Run(_ context.Context, _ RewriteRunRequest) (*domain.RewritePipelineRun, error) {
	return nil, errors.New("unexpected run call")
}

func (r *stubArticleIntakeResumeRewriteRunner) Resume(_ context.Context, rewriteRunID string, title string) (*domain.RewritePipelineRun, error) {
	r.called = true
	r.lastRewriteID = rewriteRunID
	r.lastTitle = title
	if r.err != nil {
		return nil, r.err
	}
	if r.result == nil {
		return nil, nil
	}
	copyRun := *r.result
	copyRun.ID = rewriteRunID
	return &copyRun, nil
}

func TestArticleIntakeServiceCreatesWorkspaceArticleAndTriggersRewrite(t *testing.T) {
	workspaceRepo := &stubArticleIntakeWorkspaceRepo{}
	rewrite := &stubArticleIntakeRewriteRunner{}
	svc := NewArticleIntakeService(workspaceRepo, rewrite)
	article := domain.IntakeArticle{
		ExternalID:            "guid-1",
		SourceType:            "rss",
		SubscriptionID:        "sub-1",
		Title:                 "Title",
		Body:                  "Body",
		Summary:               "Summary",
		OriginalURL:           "https://example.com/a",
		TargetType:            "wechat-longform",
		SourceProfile:         "sspai",
		RewriteProfileVersion: "latest",
	}

	workspace, err := svc.Intake(t.Context(), article)

	require.NoError(t, err)
	require.Len(t, workspaceRepo.created, 1)
	created := workspaceRepo.created[0]
	require.Equal(t, created.ID, workspace.ID)
	require.Equal(t, "Title", workspace.Title)
	require.Equal(t, "Summary", workspace.Summary)
	require.Equal(t, "rss", workspace.Source.SourceType)
	require.Equal(t, "https://example.com/a", workspace.Source.URL)
	require.Equal(t, "guid-1", workspace.Metadata["rss_guid"])
	require.Equal(t, "sub-1", workspace.Metadata["rss_subscription_id"])
	require.Equal(t, "Body", workspace.Metadata["source_body"])
	require.True(t, rewrite.called)
	require.Equal(t, RewriteRunRequest{
		WorkspaceArticleID: workspace.ID,
		Title:              "Title",
		TargetType:         "wechat-longform",
		SourceProfile:      "sspai",
		Version:            "latest",
	}, rewrite.lastReq)
}

func TestArticleIntakeServiceReturnsCreatedWorkspaceWhenRewriteFails(t *testing.T) {
	workspaceRepo := &stubArticleIntakeWorkspaceRepo{}
	rewrite := &stubArticleIntakeRewriteRunner{err: errors.New("rewrite failed")}
	svc := NewArticleIntakeService(workspaceRepo, rewrite)
	article := domain.IntakeArticle{
		ExternalID:            "guid-1",
		SourceType:            "rss",
		SubscriptionID:        "sub-1",
		Title:                 "Title",
		Body:                  "Body",
		Summary:               "Summary",
		OriginalURL:           "https://example.com/a",
		TargetType:            "wechat-longform",
		SourceProfile:         "sspai",
		RewriteProfileVersion: "latest",
	}

	workspace, err := svc.Intake(t.Context(), article)

	require.Error(t, err)
	require.ErrorContains(t, err, "run rewrite orchestrator")
	require.Len(t, workspaceRepo.created, 1)
	require.NotNil(t, workspace)
	require.Equal(t, workspaceRepo.created[0].ID, workspace.ID)
	require.True(t, rewrite.called)
}

func TestArticleIntakeServiceReusesExistingWorkspaceID(t *testing.T) {
	workspaceRepo := &stubArticleIntakeWorkspaceRepo{}
	rewrite := &stubArticleIntakeRewriteRunner{}
	svc := NewArticleIntakeService(workspaceRepo, rewrite)
	article := domain.IntakeArticle{
		ExternalID:            "guid-1",
		SourceType:            "rss",
		SubscriptionID:        "sub-1",
		Title:                 "Title",
		Body:                  "Body",
		Summary:               "Summary",
		OriginalURL:           "https://example.com/a",
		TargetType:            "wechat-longform",
		SourceProfile:         "sspai",
		RewriteProfileVersion: "latest",
	}

	workspace, err := svc.IntakeIntoWorkspace(t.Context(), "workspace-existing", article)

	require.NoError(t, err)
	require.Empty(t, workspaceRepo.created)
	require.Equal(t, "workspace-existing", workspace.ID)
	require.True(t, rewrite.called)
	require.Equal(t, "workspace-existing", rewrite.lastReq.WorkspaceArticleID)
}

func TestArticleIntakeServiceReturnsExplicitRewriteResult(t *testing.T) {
	workspaceRepo := &stubArticleIntakeWorkspaceRepo{}
	rewrite := &stubArticleIntakeRewriteRunner{}
	svc := NewArticleIntakeService(workspaceRepo, rewrite)
	article := domain.IntakeArticle{
		ExternalID:            "guid-1",
		SourceType:            "rss",
		SubscriptionID:        "sub-1",
		Title:                 "Title",
		Body:                  "Body",
		Summary:               "Summary",
		OriginalURL:           "https://example.com/a",
		TargetType:            "wechat-longform",
		SourceProfile:         "sspai",
		RewriteProfileVersion: "latest",
	}

	result, err := svc.IntakeResult(t.Context(), article)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.WorkspaceArticle)
	require.NotNil(t, result.RewriteRun)
	require.Equal(t, "run-1", result.RewriteRun.ID)
	require.Equal(t, result.RewriteRun.FinalDraftID, result.DraftID)
}

func TestArticleIntakeServiceOmitsCollectorMetadataForBrowserFirstSources(t *testing.T) {
	workspaceRepo := &stubArticleIntakeWorkspaceRepo{}
	rewrite := &stubArticleIntakeRewriteRunner{}
	svc := NewArticleIntakeService(workspaceRepo, rewrite)
	article := domain.IntakeArticle{
		ExternalID:            "doc-1",
		SourceType:            "upload",
		Title:                 "Title",
		Body:                  "Body",
		Summary:               "Summary",
		OriginalURL:           "uploaded://doc-1",
		TargetType:            "wechat-longform",
		SourceProfile:         "web-upload",
		RewriteProfileVersion: "v1",
	}

	workspace, err := svc.Intake(t.Context(), article)

	require.NoError(t, err)
	require.NotNil(t, workspace)
	require.True(t, rewrite.called)
	require.Equal(t, RewriteRunRequest{
		WorkspaceArticleID: workspace.ID,
		Title:              "Title",
		TargetType:         "wechat-longform",
		SourceProfile:      "web-upload",
		Version:            "v1",
	}, rewrite.lastReq)
	require.NotContains(t, workspaceRepo.created[0].Metadata, "collector_article_id")
}

func TestArticleIntakeServiceDoesNotPreserveCollectorMetadataForCompatibilitySources(t *testing.T) {
	workspaceRepo := &stubArticleIntakeWorkspaceRepo{}
	rewrite := &stubArticleIntakeRewriteRunner{}
	svc := NewArticleIntakeService(workspaceRepo, rewrite)
	article := domain.IntakeArticle{
		ExternalID:            "compat-collector-1",
		SourceType:            "rss",
		SubscriptionID:        "sub-1",
		Title:                 "Title",
		Body:                  "Body",
		Summary:               "Summary",
		OriginalURL:           "https://example.com/a",
		TargetType:            "wechat-longform",
		SourceProfile:         "web-upload",
		RewriteProfileVersion: "v1",
	}

	workspace, err := svc.Intake(t.Context(), article)

	require.NoError(t, err)
	require.NotNil(t, workspace)
	require.True(t, rewrite.called)
	require.Equal(t, RewriteRunRequest{
		WorkspaceArticleID: workspace.ID,
		Title:              "Title",
		TargetType:         "wechat-longform",
		SourceProfile:      "web-upload",
		Version:            "v1",
	}, rewrite.lastReq)
	require.NotContains(t, workspaceRepo.created[0].Metadata, "collector_article_id")
}

func TestArticleIntakeServiceCarriesWorkflowSelectionMetadataIntoRewriteRequest(t *testing.T) {
	workspaceRepo := &stubArticleIntakeWorkspaceRepo{}
	rewrite := &stubArticleIntakeRewriteRunner{}
	workflows := &stubArticleIntakeWorkflowRepo{workflow: &domain.WorkflowDefinition{
		ID:          "workflow-a",
		Name:        "Workflow A",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "node-generate-draft",
		Nodes: []domain.WorkflowNode{{
			ID:         "node-generate-draft",
			Type:       "rewrite_stage",
			Name:       "generate_draft",
			ConfigJSON: `{"stage_name":"generate_draft","prompt_ref":"generate_draft_alt@v2","vars":{"workflow_marker":"workflow-a"}}`,
		}},
	}}
	svc := NewArticleIntakeServiceWithWorkflows(workspaceRepo, rewrite, workflows)
	article := domain.IntakeArticle{
		ExternalID:            "guid-1",
		SourceType:            "rss",
		SubscriptionID:        "sub-1",
		Title:                 "Title",
		Body:                  "Body",
		Summary:               "Summary",
		OriginalURL:           "https://example.com/a",
		TargetType:            "wechat-longform",
		SourceProfile:         "sspai",
		RewriteProfileVersion: "latest",
		Metadata: map[string]any{
			"workflow_template_id":      "workflow-a",
			"workflow_prompt_ref":       "generate_draft_alt@v2",
			"workflow_node_generate_draft": "node-generate-draft",
		},
	}

	_, err := svc.Intake(t.Context(), article)

	require.NoError(t, err)
	require.True(t, rewrite.called)
	require.Equal(t, "workflow-a", workflows.gotID)
	require.Equal(t, "workflow-a", rewrite.lastReq.Metadata["workflow_template_id"])
	require.Equal(t, "generate_draft_alt@v2", rewrite.lastReq.Metadata["workflow_prompt_ref"])
	require.Equal(t, "node-generate-draft", rewrite.lastReq.Metadata["workflow_node_generate_draft"])
	require.Contains(t, rewrite.lastReq.Metadata, workflowStageOverridesMetadataKey)
}

func TestArticleIntakeServiceRejectsDisabledWorkflowReference(t *testing.T) {
	workspaceRepo := &stubArticleIntakeWorkspaceRepo{}
	rewrite := &stubArticleIntakeRewriteRunner{}
	workflows := &stubArticleIntakeWorkflowRepo{workflow: &domain.WorkflowDefinition{
		ID:          "workflow-disabled",
		Name:        "Disabled workflow",
		Version:     "v1",
		Enabled:     false,
		EntryNodeID: "node-generate-draft",
		Nodes:       []domain.WorkflowNode{{ID: "node-generate-draft", Type: "rewrite_stage", Name: "generate_draft"}},
	}}
	svc := NewArticleIntakeServiceWithWorkflows(workspaceRepo, rewrite, workflows)
	article := domain.IntakeArticle{
		ExternalID:            "guid-1",
		SourceType:            "rss",
		SubscriptionID:        "sub-1",
		Title:                 "Title",
		Body:                  "Body",
		Summary:               "Summary",
		OriginalURL:           "https://example.com/a",
		TargetType:            "wechat-longform",
		SourceProfile:         "sspai",
		RewriteProfileVersion: "latest",
		Metadata: map[string]any{
			"workflow_template_id": "workflow-disabled",
		},
	}

	_, err := svc.Intake(t.Context(), article)

	require.Error(t, err)
	require.ErrorContains(t, err, "disabled")
	require.False(t, rewrite.called)
}

func TestArticleIntakeServiceResumeRejectsDisabledWorkflowReference(t *testing.T) {
	rewrite := &stubArticleIntakeRewriteRunner{}
	workflows := &stubArticleIntakeWorkflowRepo{workflow: &domain.WorkflowDefinition{
		ID:          "workflow-disabled",
		Name:        "Disabled workflow",
		Version:     "v1",
		Enabled:     false,
		EntryNodeID: "node-generate-draft",
		Nodes:       []domain.WorkflowNode{{ID: "node-generate-draft", Type: "rewrite_stage", Name: "generate_draft"}},
	}}
	svc := NewArticleIntakeServiceWithWorkflows(&stubArticleIntakeWorkspaceRepo{}, rewrite, workflows)
	resumeRunner := &stubArticleIntakeResumeRewriteRunner{}
	svc.rewrite = resumeRunner
	article := domain.IntakeArticle{
		ExternalID:            "guid-1",
		SourceType:            "rss",
		SubscriptionID:        "sub-1",
		Title:                 "Title",
		Body:                  "Body",
		Summary:               "Summary",
		OriginalURL:           "https://example.com/a",
		TargetType:            "wechat-longform",
		SourceProfile:         "sspai",
		RewriteProfileVersion: "latest",
		Metadata: map[string]any{
			"workflow_template_id": "workflow-disabled",
		},
	}

	result, err := svc.ResumeResult(t.Context(), "rewrite-1", article)

	require.Nil(t, result)
	require.Error(t, err)
	require.ErrorContains(t, err, "disabled")
	require.False(t, resumeRunner.called)
}

func TestArticleIntakeServiceResumeAllowsEnabledWorkflowReference(t *testing.T) {
	workflows := &stubArticleIntakeWorkflowRepo{workflow: &domain.WorkflowDefinition{
		ID:          "workflow-enabled",
		Name:        "Enabled workflow",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "node-generate-draft",
		Nodes:       []domain.WorkflowNode{{ID: "node-generate-draft", Type: "rewrite_stage", Name: "generate_draft"}},
	}}
	resumeRunner := &stubArticleIntakeResumeRewriteRunner{result: &domain.RewritePipelineRun{ID: "rewrite-1", WorkspaceArticleID: "workspace-1", FinalDraftID: "draft-1"}}
	svc := NewArticleIntakeServiceWithWorkflows(&stubArticleIntakeWorkspaceRepo{}, resumeRunner, workflows)
	article := domain.IntakeArticle{
		ExternalID:            "guid-1",
		SourceType:            "rss",
		SubscriptionID:        "sub-1",
		Title:                 "Title",
		Body:                  "Body",
		Summary:               "Summary",
		OriginalURL:           "https://example.com/a",
		TargetType:            "wechat-longform",
		SourceProfile:         "sspai",
		RewriteProfileVersion: "latest",
		Metadata: map[string]any{
			"workflow_template_id": "workflow-enabled",
		},
	}

	result, err := svc.ResumeResult(t.Context(), "rewrite-1", article)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, resumeRunner.called)
	require.Equal(t, "rewrite-1", result.RewriteRunID)
	require.Equal(t, "workspace-1", result.WorkspaceArticleID)
	require.Equal(t, "draft-1", result.DraftID)
}
