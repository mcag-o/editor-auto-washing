package integration

import (
	"content-hub/domain"
	"content-hub/infra/config"
	llminfra "content-hub/infra/llm"
	workspaceinfra "content-hub/infra/workspace"
	"content-hub/service"
	httpserver "content-hub/transport/http"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWebControlPlaneUploadToRenderedResultWithWorkflowTemplate(t *testing.T) {
	root, repos, serverURL := newWebControlPlaneIntegrationServer(t)
	require.DirExists(t, root)

	templateResp := postJSON(t, serverURL+"/api/templates", map[string]any{
		"id":             "template-definition-mainline",
		"name":           "Generate Draft Mainline",
		"type":           "prompt",
		"version":        "v1",
		"enabled":        true,
		"content":        "Rewrite {{title}} into a polished draft.",
		"variables_json": map[string]any{"title": "string"},
		"updated_by":     "integration-test",
	})
	defer templateResp.Body.Close()
	require.Equal(t, http.StatusCreated, templateResp.StatusCode)

	workflowResp := postJSON(t, serverURL+"/api/workflows", map[string]any{
		"id":            "workflow-template-mainline",
		"name":          "Graph Workflow Mainline",
		"description":   "Mainline integration workflow",
		"version":       "v1",
		"enabled":       true,
		"entry_node_id": "generate_draft",
		"nodes": []map[string]any{{
			"id":          "generate_draft",
			"type":        "rewrite_stage",
			"name":        "generate_draft",
			"config_json": `{"stage_name":"generate_draft","prompt_ref":"generate_draft_alt@v2","vars":{"workflow_marker":"graph-mainline"}}`,
		}},
		"edges":      []map[string]any{},
		"updated_by": "integration-test",
	})
	defer workflowResp.Body.Close()
	require.Equal(t, http.StatusCreated, workflowResp.StatusCode)

	templatesListResp, err := http.Get(serverURL + "/api/templates")
	require.NoError(t, err)
	defer templatesListResp.Body.Close()
	require.Equal(t, http.StatusOK, templatesListResp.StatusCode)

	var templatesList struct {
		Data []domain.TemplateDefinition `json:"data"`
	}
	require.NoError(t, json.NewDecoder(templatesListResp.Body).Decode(&templatesList))
	require.Len(t, templatesList.Data, 1)
	require.Equal(t, "template-definition-mainline", templatesList.Data[0].ID)

	workflowsListResp, err := http.Get(serverURL + "/api/workflows")
	require.NoError(t, err)
	defer workflowsListResp.Body.Close()
	require.Equal(t, http.StatusOK, workflowsListResp.StatusCode)

	var workflowsList struct {
		Data []domain.WorkflowDefinition `json:"data"`
	}
	require.NoError(t, json.NewDecoder(workflowsListResp.Body).Decode(&workflowsList))
	require.Len(t, workflowsList.Data, 1)
	require.Equal(t, "workflow-template-mainline", workflowsList.Data[0].ID)

	pasteResp := postJSON(t, serverURL+"/api/intake/paste", map[string]any{
		"title": "Graph Control Plane Source",
		"body":  "Body pasted for graph workflow mainline.",
	})
	defer pasteResp.Body.Close()
	require.Equal(t, http.StatusCreated, pasteResp.StatusCode)

	var createdDoc domain.SourceDocument
	require.NoError(t, json.NewDecoder(pasteResp.Body).Decode(&createdDoc))

	assignResp := postJSON(t, serverURL+"/api/articles/"+createdDoc.ID+"/workflow-template", map[string]any{
		"workflow_template_id": "workflow-template-mainline",
	})
	defer assignResp.Body.Close()
	require.Equal(t, http.StatusOK, assignResp.StatusCode)

	startResp := postJSON(t, serverURL+"/api/system/start", map[string]any{
		"concurrency_limit": 1,
	})
	defer startResp.Body.Close()
	if startResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(startResp.Body)
		t.Fatalf("unexpected start status %d: %s", startResp.StatusCode, string(body))
	}
	require.Equal(t, http.StatusOK, startResp.StatusCode)

	articleResp, err := http.Get(serverURL + "/api/articles/" + createdDoc.ID)
	require.NoError(t, err)
	defer articleResp.Body.Close()
	require.Equal(t, http.StatusOK, articleResp.StatusCode)

	var article domain.SourceDocument
	require.NoError(t, json.NewDecoder(articleResp.Body).Decode(&article))
	require.Equal(t, domain.SourceDocumentStatusCompleted, article.Status)
	require.Equal(t, "workflow-template-mainline", article.Metadata["workflow_template_id"])
	require.NotEmpty(t, article.WorkspaceArticleID)
	require.NotEmpty(t, article.RewriteRunID)

	stagesResp, err := http.Get(serverURL + "/api/articles/" + createdDoc.ID + "/stages")
	require.NoError(t, err)
	defer stagesResp.Body.Close()
	require.Equal(t, http.StatusOK, stagesResp.StatusCode)

	var stagesPayload struct {
		Article domain.SourceDocument      `json:"article"`
		Run     *domain.RewritePipelineRun `json:"run"`
		Stages  []domain.RewriteStageRun   `json:"stages"`
	}
	require.NoError(t, json.NewDecoder(stagesResp.Body).Decode(&stagesPayload))
	require.NotNil(t, stagesPayload.Run)
	require.Equal(t, domain.RewriteRunSucceeded, stagesPayload.Run.Status)
	require.Equal(t, "workflow-template-mainline", stagesPayload.Run.Metadata["workflow_template_id"])
	require.Equal(t, "generate_draft_alt@v2", stagesPayload.Run.Metadata["workflow_prompt_ref"])
	require.NotContains(t, stagesPayload.Run.Metadata, "active_token_set")
	require.NotContains(t, stagesPayload.Run.Metadata, "rewrite_workflow_checkpoint")
	routeSummary, ok := stagesPayload.Run.Metadata["workflow_route_latest"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "materialize_draft", routeSummary["node_id"])
	require.Equal(t, "no_match", routeSummary["outcome"])
	require.Len(t, stagesPayload.Stages, 1)
	require.Equal(t, domain.RewriteStageSucceeded, stagesPayload.Stages[0].Status)
	require.Equal(t, "generate_draft_alt@v2", stagesPayload.Stages[0].PromptRef)
	require.Contains(t, stagesPayload.Stages[0].InputJSON, "graph-mainline")

	workspace, err := repos.WorkspaceRepo.GetByID(t.Context(), article.WorkspaceArticleID)
	require.NoError(t, err)
	require.Equal(t, domain.ArticleWorkspaceStatusRendered, workspace.Status)
	require.Equal(t, "workflow-template-mainline", workspace.Metadata["workflow_template_id"])

	draft, err := repos.DraftRepo.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, "daily-intelligence-alt", draft.Template)
	require.Equal(t, "Graph Mainline Title", draft.Headline["title"])

	assets, err := repos.AssetRepo.List(t.Context(), draft.ID, "wechat")
	require.NoError(t, err)
	require.Len(t, assets, 1)
	require.Contains(t, assets[0].Content, "Graph Mainline Title")

	auditResp, err := http.Get(serverURL + "/api/audit")
	require.NoError(t, err)
	defer auditResp.Body.Close()
	require.Equal(t, http.StatusOK, auditResp.StatusCode)

	var auditList struct {
		Data []domain.AuditLog `json:"data"`
	}
	require.NoError(t, json.NewDecoder(auditResp.Body).Decode(&auditList))

	actions := map[string]domain.AuditLog{}
	for _, entry := range auditList.Data {
		actions[entry.Action] = entry
	}
	require.Contains(t, actions, "web_intake.create_from_paste")
	require.Contains(t, actions, "control_plane.started")
	require.Contains(t, actions, "web_control.article.workflow_template_assigned")
	require.Equal(t, createdDoc.ID, actions["web_control.article.workflow_template_assigned"].ResourceID)
	require.Equal(t, "workflow-template-mainline", actions["web_control.article.workflow_template_assigned"].Metadata["workflow_template_id"])
}

func TestWebControlPlaneGraphWorkflowCanExposeMultipleActiveBranches(t *testing.T) {
	_, repos, serverURL := newWebControlPlaneIntegrationServer(t)

	article := domain.NewSourceDocument("paste", "paste", "txt", "Graph Branch Visibility", "Body pasted for branch visibility.", "hash-branch-visibility")
	article.SourceType = "web-paste"
	article.Status = domain.SourceDocumentStatusPending
	require.NoError(t, repos.SourceDocumentRepo.Create(t.Context(), article))

	startedAt := time.Now().UTC()
	article.Status = domain.SourceDocumentStatusProcessing
	article.WorkspaceArticleID = "workspace-branch-1"
	article.RewriteRunID = "run-graph-branches"
	article.ClaimedBy = "worker-branches"
	article.ClaimedAt = &startedAt
	article.ProcessingStartedAt = &startedAt
	require.NoError(t, repos.SourceDocumentRepo.Update(t.Context(), article))

	run := domain.NewRewritePipelineRun("profile-web-mainline", "v1", article.WorkspaceArticleID, article.ID, "wechat-longform", "web-paste")
	run.ID = article.RewriteRunID
	run.Status = domain.RewriteRunRunning
	run.CurrentStage = "review_draft"
	run.Metadata["workflow_template_id"] = "workflow-template-branch-visibility"
	run.Metadata["workflow_prompt_ref"] = "generate_draft_alt@v2"
	run.Metadata["active_token_set"] = []map[string]any{
		{
			"token_id":                            "token-right",
			"token_parent_id":                     "token-root",
			"token_origin_id":                     "token-root",
			"token_origin_route_node_id":          "generate_draft",
			"token_origin_route_edge_id":          "2:generate_draft->review_right@1[payload.route == approved]",
			"token_origin_route_selected_node_id": "review_right",
			"node_id":                             "review_right",
			"token_branch_vars":                   map[string]any{"branch": "right"},
		},
		{
			"token_id":                            "token-left",
			"token_parent_id":                     "token-root",
			"token_origin_id":                     "token-root",
			"token_origin_route_node_id":          "generate_draft",
			"token_origin_route_edge_id":          "1:generate_draft->review_left@1[payload.route == approved]",
			"token_origin_route_selected_node_id": "review_left",
			"node_id":                             "review_left",
			"token_branch_vars":                   map[string]any{"branch": "left"},
		},
	}
	run.Metadata["workflow_route_latest"] = map[string]any{
		"node_id":          "generate_draft",
		"selected_edge_id": "1:generate_draft->review_left@1[payload.route == approved]",
		"selected_node_id": "review_left",
		"outcome":          "matched",
		"evaluation_trace": []string{"payload.route == approved => true", "payload.route == approved => true"},
	}
	run.Metadata["rewrite_workflow_checkpoint"] = map[string]any{
		"node_id": "review_left",
		"payload": map[string]any{
			"title": "Graph Branch Visibility",
			"route": "approved",
		},
		"token_id":               "token-left",
		"token_parent_id":        "token-root",
		"token_origin_id":        "token-root",
		"token_origin_route_node_id":          "generate_draft",
		"token_origin_route_edge_id":          "1:generate_draft->review_left@1[payload.route == approved]",
		"token_origin_route_selected_node_id": "review_left",
		"active_token_set": []map[string]any{
			{
				"token_id":                            "token-right",
				"token_parent_id":                     "token-root",
				"token_origin_id":                     "token-root",
				"token_origin_route_node_id":          "generate_draft",
				"token_origin_route_edge_id":          "2:generate_draft->review_right@1[payload.route == approved]",
				"token_origin_route_selected_node_id": "review_right",
				"node_id":                             "review_right",
				"token_branch_vars":                   map[string]any{"branch": "right"},
			},
			{
				"token_id":                            "token-left",
				"token_parent_id":                     "token-root",
				"token_origin_id":                     "token-root",
				"token_origin_route_node_id":          "generate_draft",
				"token_origin_route_edge_id":          "1:generate_draft->review_left@1[payload.route == approved]",
				"token_origin_route_selected_node_id": "review_left",
				"node_id":                             "review_left",
				"token_branch_vars":                   map[string]any{"branch": "left"},
			},
		},
	}
	require.NoError(t, repos.RewritePipelineRunRepo.Create(t.Context(), run))

	stagesResp, err := http.Get(serverURL + "/api/articles/" + article.ID + "/stages")
	require.NoError(t, err)
	defer stagesResp.Body.Close()
	require.Equal(t, http.StatusOK, stagesResp.StatusCode)

	var stagesPayload struct {
		Article domain.SourceDocument      `json:"article"`
		Run     *domain.RewritePipelineRun `json:"run"`
		Stages  []domain.RewriteStageRun   `json:"stages"`
	}
	require.NoError(t, json.NewDecoder(stagesResp.Body).Decode(&stagesPayload))
	require.NotNil(t, stagesPayload.Run)
	require.Equal(t, domain.RewriteRunRunning, stagesPayload.Run.Status)
	require.Contains(t, stagesPayload.Run.Metadata, "rewrite_workflow_checkpoint")
	activeSet, ok := stagesPayload.Run.Metadata["active_token_set"].([]any)
	if !ok {
		mapped, mappedOK := stagesPayload.Run.Metadata["active_token_set"].([]map[string]any)
		require.True(t, mappedOK)
		require.Len(t, mapped, 2)
		require.Equal(t, "review_right", mapped[0]["node_id"])
		require.Equal(t, "token-right", mapped[0]["token_id"])
		require.Equal(t, "review_left", mapped[1]["node_id"])
		require.Equal(t, "token-left", mapped[1]["token_id"])
		routeSummary, ok := stagesPayload.Run.Metadata["workflow_route_latest"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "generate_draft", routeSummary["node_id"])
		require.Equal(t, "matched", routeSummary["outcome"])
		return
	}
	require.Len(t, activeSet, 2)
	firstBranch, ok := activeSet[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "review_right", firstBranch["node_id"])
	require.Equal(t, "token-right", firstBranch["token_id"])
	secondBranch, ok := activeSet[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "review_left", secondBranch["node_id"])
	require.Equal(t, "token-left", secondBranch["token_id"])
	routeSummary, ok := stagesPayload.Run.Metadata["workflow_route_latest"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "generate_draft", routeSummary["node_id"])
	require.Equal(t, "matched", routeSummary["outcome"])
}

func TestWebControlPlaneArticleOperationsAuditAndWorkflowSemantics(t *testing.T) {
	_, repos, serverURL := newWebControlPlaneIntegrationServer(t)

	workflowV1Resp := postJSON(t, serverURL+"/api/workflows", map[string]any{
		"id":            "workflow-ops",
		"name":          "Ops Workflow",
		"description":   "Operations workflow",
		"version":       "v1",
		"enabled":       true,
		"entry_node_id": "generate_draft",
		"nodes": []map[string]any{{
			"id":          "generate_draft",
			"type":        "rewrite_stage",
			"name":        "generate_draft",
			"config_json": `{"stage_name":"generate_draft","prompt_ref":"generate_draft@v1"}`,
		}},
		"edges":      []map[string]any{},
		"updated_by": "integration-test",
	})
	defer workflowV1Resp.Body.Close()
	require.Equal(t, http.StatusCreated, workflowV1Resp.StatusCode)

	pauseResp := postJSON(t, serverURL+"/api/intake/paste", map[string]any{
		"title": "Pause Target",
		"body":  "Pause body.",
	})
	defer pauseResp.Body.Close()
	require.Equal(t, http.StatusCreated, pauseResp.StatusCode)
	var pauseDoc domain.SourceDocument
	require.NoError(t, json.NewDecoder(pauseResp.Body).Decode(&pauseDoc))
	storedPauseDoc, err := repos.SourceDocumentRepo.GetByID(t.Context(), pauseDoc.ID)
	require.NoError(t, err)
	started := time.Now().UTC()
	storedPauseDoc.Status = domain.SourceDocumentStatusProcessing
	storedPauseDoc.RewriteRunID = "run-pause"
	storedPauseDoc.ClaimedBy = "worker-1"
	storedPauseDoc.ClaimedAt = &started
	storedPauseDoc.ProcessingStartedAt = &started
	require.NoError(t, repos.SourceDocumentRepo.Update(t.Context(), storedPauseDoc))
	pauseRun := domain.NewRewritePipelineRun("profile-1", "v1", "workspace-1", storedPauseDoc.ID, "wechat-longform", "web-paste")
	pauseRun.ID = "run-pause"
	pauseRun.Status = domain.RewriteRunRunning
	pauseRun.CurrentStage = "generate_draft"
	pauseRun.Metadata["workflow_template_id"] = "workflow-ops"
	pauseRun.Metadata["workflow_template_version"] = "v1"
	require.NoError(t, repos.RewritePipelineRunRepo.Create(t.Context(), pauseRun))

	stopReq, err := http.NewRequest(http.MethodPost, serverURL+"/api/articles/"+storedPauseDoc.ID+"/stop", nil)
	require.NoError(t, err)
	stopResp, err := http.DefaultClient.Do(stopReq)
	require.NoError(t, err)
	defer stopResp.Body.Close()
	require.Equal(t, http.StatusAccepted, stopResp.StatusCode)
	stoppedDoc, err := repos.SourceDocumentRepo.GetByID(t.Context(), storedPauseDoc.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SourceDocumentStatusPaused, stoppedDoc.Status)
	require.Equal(t, "run-pause", stoppedDoc.RewriteRunID)
	require.Empty(t, stoppedDoc.ClaimedBy)
	require.Nil(t, stoppedDoc.ClaimedAt)
	require.NotNil(t, stoppedDoc.ProcessingStartedAt)

	require.NoError(t, repos.SourceDocumentRepo.Update(t.Context(), stoppedDoc))
	resumeReq, err := http.NewRequest(http.MethodPost, serverURL+"/api/articles/"+stoppedDoc.ID+"/resume", nil)
	require.NoError(t, err)
	resumeResp, err := http.DefaultClient.Do(resumeReq)
	require.NoError(t, err)
	defer resumeResp.Body.Close()
	require.Equal(t, http.StatusAccepted, resumeResp.StatusCode)
	resumedDoc, err := repos.SourceDocumentRepo.GetByID(t.Context(), stoppedDoc.ID)
	require.NoError(t, err)
	require.Equal(t, domain.SourceDocumentStatusPending, resumedDoc.Status)
	require.Equal(t, "run-pause", resumedDoc.RewriteRunID)

	deleteResp := postJSON(t, serverURL+"/api/intake/paste", map[string]any{
		"title": "Delete Target",
		"body":  "Delete body.",
	})
	defer deleteResp.Body.Close()
	require.Equal(t, http.StatusCreated, deleteResp.StatusCode)
	var deleteDoc domain.SourceDocument
	require.NoError(t, json.NewDecoder(deleteResp.Body).Decode(&deleteDoc))
	storedDeleteDoc, err := repos.SourceDocumentRepo.GetByID(t.Context(), deleteDoc.ID)
	require.NoError(t, err)
	storedDeleteDoc.Status = domain.SourceDocumentStatusPaused
	storedDeleteDoc.RewriteRunID = "run-delete"
	storedDeleteDoc.ClaimedBy = "worker-2"
	storedDeleteDoc.ClaimedAt = &started
	storedDeleteDoc.ProcessingStartedAt = &started
	require.NoError(t, repos.SourceDocumentRepo.Update(t.Context(), storedDeleteDoc))
	deleteRun := domain.NewRewritePipelineRun("profile-1", "v1", "workspace-1", storedDeleteDoc.ID, "wechat-longform", "web-paste")
	deleteRun.ID = "run-delete"
	deleteRun.Metadata["workflow_template_id"] = "workflow-ops"
	deleteRun.Metadata["workflow_template_version"] = "v1"
	require.NoError(t, repos.RewritePipelineRunRepo.Create(t.Context(), deleteRun))
	require.NoError(t, repos.RewriteStageRunRepo.Create(t.Context(), &domain.RewriteStageRun{
		ID:            "stage-delete",
		PipelineRunID: deleteRun.ID,
		StageName:     "generate_draft",
		StageType:     "generate_draft",
		Status:        domain.RewriteStageRunning,
		Attempt:       1,
		StartedAt:     started,
	}))
	deleteReq, err := http.NewRequest(http.MethodDelete, serverURL+"/api/articles/"+storedDeleteDoc.ID, nil)
	require.NoError(t, err)
	deleteOpResp, err := http.DefaultClient.Do(deleteReq)
	require.NoError(t, err)
	defer deleteOpResp.Body.Close()
	require.Equal(t, http.StatusNoContent, deleteOpResp.StatusCode)
	_, err = repos.SourceDocumentRepo.GetByID(t.Context(), storedDeleteDoc.ID)
	require.Error(t, err)
	_, err = repos.RewritePipelineRunRepo.GetByID(t.Context(), deleteRun.ID)
	require.Error(t, err)
	deleteStages, err := repos.RewriteStageRunRepo.ListByPipelineRunID(t.Context(), deleteRun.ID)
	require.NoError(t, err)
	require.Len(t, deleteStages, 0)

	retryResp := postJSON(t, serverURL+"/api/intake/paste", map[string]any{
		"title": "Retry Target",
		"body":  "Retry body.",
	})
	defer retryResp.Body.Close()
	require.Equal(t, http.StatusCreated, retryResp.StatusCode)
	var retryDoc domain.SourceDocument
	require.NoError(t, json.NewDecoder(retryResp.Body).Decode(&retryDoc))
	storedRetryDoc, err := repos.SourceDocumentRepo.GetByID(t.Context(), retryDoc.ID)
	require.NoError(t, err)
	storedRetryDoc.Status = domain.SourceDocumentStatusFailed
	storedRetryDoc.ErrorSummary = "broken"
	storedRetryDoc.RewriteRunID = "run-retry"
	storedRetryDoc.Metadata["workflow_template_id"] = "workflow-ops"
	storedRetryDoc.Metadata["workflow_template_version"] = "v2"
	require.NoError(t, repos.SourceDocumentRepo.Update(t.Context(), storedRetryDoc))
	retryRun := domain.NewRewritePipelineRun("profile-1", "v1", "workspace-1", storedRetryDoc.ID, "wechat-longform", "web-paste")
	retryRun.ID = "run-retry"
	retryRun.Status = domain.RewriteRunFailed
	retryRun.CurrentStage = "generate_draft"
	retryRun.CompletedAt = &started
	retryRun.ErrorSummary = "broken"
	retryRun.Metadata["workflow_template_id"] = "workflow-ops"
	retryRun.Metadata["workflow_template_version"] = "v1"
	require.NoError(t, repos.RewritePipelineRunRepo.Create(t.Context(), retryRun))
	require.NoError(t, repos.RewriteStageRunRepo.Create(t.Context(), &domain.RewriteStageRun{
		ID:            "stage-retry",
		PipelineRunID: retryRun.ID,
		StageName:     "generate_draft",
		StageType:     "generate_draft",
		Status:        domain.RewriteStageFailed,
		Attempt:       2,
		StartedAt:     started,
		CompletedAt:   &started,
		ErrorSummary:  "bad output",
	}))
	retryReq, err := http.NewRequest(http.MethodPost, serverURL+"/api/articles/"+storedRetryDoc.ID+"/retry", nil)
	require.NoError(t, err)
	retryOpResp, err := http.DefaultClient.Do(retryReq)
	require.NoError(t, err)
	defer retryOpResp.Body.Close()
	require.Equal(t, http.StatusOK, retryOpResp.StatusCode)
	_, err = repos.RewritePipelineRunRepo.GetByID(t.Context(), retryRun.ID)
	require.Error(t, err)
	retryStages, err := repos.RewriteStageRunRepo.ListByPipelineRunID(t.Context(), retryRun.ID)
	require.NoError(t, err)
	require.Len(t, retryStages, 0)

	auditResp, err := http.Get(serverURL + "/api/audit")
	require.NoError(t, err)
	defer auditResp.Body.Close()
	require.Equal(t, http.StatusOK, auditResp.StatusCode)
	var auditList struct {
		Data []domain.AuditLog `json:"data"`
	}
	require.NoError(t, json.NewDecoder(auditResp.Body).Decode(&auditList))

	var stopAudit, resumeAudit, deleteAudit, retryAudit *domain.AuditLog
	for i := range auditList.Data {
		entry := auditList.Data[i]
		switch entry.Action {
		case "web_control.article.stop":
			if entry.ResourceID == storedPauseDoc.ID {
				stopAudit = &entry
			}
		case "web_control.article.resume":
			if entry.ResourceID == storedPauseDoc.ID {
				resumeAudit = &entry
			}
		case "web_control.article.delete":
			if entry.ResourceID == storedDeleteDoc.ID {
				deleteAudit = &entry
			}
		case "web_control.article.retry":
			if entry.ResourceID == storedRetryDoc.ID {
				retryAudit = &entry
			}
		}
	}
	require.NotNil(t, stopAudit)
	require.Equal(t, "success", stopAudit.Result)
	require.NotNil(t, resumeAudit)
	require.Equal(t, "success", resumeAudit.Result)
	require.NotNil(t, deleteAudit)
	require.Equal(t, "success", deleteAudit.Result)
	require.Equal(t, true, deleteAudit.Metadata["workflow_records_deleted"])
	require.NotNil(t, retryAudit)
	require.Equal(t, "success", retryAudit.Result)
	require.Equal(t, true, retryAudit.Metadata["workflow_state_reset"])
}

func newWebControlPlaneIntegrationServer(t *testing.T) (string, *service.RuntimeRepos, string) {
	t.Helper()

	root := t.TempDir()
	loader := workspaceinfra.NewLoader()
	require.NoError(t, loader.Save(root, domain.DefaultWorkspaceSettings()))
	require.NoError(t, os.WriteFile(filepath.Join(root, workspaceinfra.WorkspaceSecretsFileName), []byte("env:\n  LLM_API_KEY: test\nwechat:\n  main: token\n"), 0o600))

	templateDir := filepath.Join(root, "templates")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "daily-intelligence.html"), []byte(`<html><body><h1>{{TITLE}}</h1><div>{{BODY_SECTIONS}}</div><footer>{{CTA}}</footer></body></html>`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "daily-intelligence-alt.html"), []byte(`<html><body><article><header>{{TITLE}}</header><section>{{BODY_SECTIONS}}</section><aside>{{CTA}}</aside></article></body></html>`), 0o644))

	repos, cleanup, err := service.BuildRuntimeRepos(root)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, cleanup())
	})

	repos.LLMClient = llminfra.StaticClient{Response: domain.LLMResponse{
		Content:      `{"title":"Graph Mainline Title","body":"Rendered graph workflow body.","template":"daily-intelligence-alt","meta":{"digest":"Graph digest","author":"Integration Bot"},"sections":[{"cn":"Main Section","blocks":[{"type":"card","title":"Key Point","body":["Workflow-selected control plane detail."],"source":"Web Control"}]}],"conclusion":"Graph end note.","cta":"Read more."}`,
		Model:        "static-integration-model",
		FinishReason: "stop",
	}}

	require.NoError(t, repos.RewritePipelineProfileRepo.Upsert(t.Context(), &domain.RewritePipelineProfile{
		ID:                    "profile-web-mainline",
		Name:                  "Web Control Mainline",
		TargetType:            "wechat-longform",
		SourceProfile:         "web-paste",
		Version:               "v1",
		Description:           "Mainline web control rewrite profile",
		DefaultLLMProfile:     "rewrite-default",
		MaterializationPolicy: "workspace-draft",
		Enabled:               true,
		Stages: []domain.RewriteStageDefinition{{
			Name:      "generate_draft",
			Type:      "generate_draft",
			PromptRef: "generate_draft@v1",
			Enabled:   true,
		}},
	}))

	require.NoError(t, repos.PromptTemplateRepo.Upsert(t.Context(), &domain.PromptTemplate{
		Key:            "generate_draft",
		Version:        "v1",
		SystemTemplate: "You rewrite imported articles into drafts.",
		UserTemplate:   "Rewrite {{title}} into a polished draft.",
		Description:    "Integration rewrite prompt",
	}))

	require.NoError(t, repos.PromptTemplateRepo.Upsert(t.Context(), &domain.PromptTemplate{
		Key:            "generate_draft_alt",
		Version:        "v2",
		SystemTemplate: "You rewrite imported articles into drafts using {{workflow_marker}}.",
		UserTemplate:   "Rewrite {{title}} into a polished draft with {{workflow_marker}}.",
		Description:    "Workflow-selected integration rewrite prompt",
	}))

	require.NoError(t, repos.LLMProfileRepo.Upsert(t.Context(), &domain.LLMProfile{
		Name:        "rewrite-default",
		Provider:    "openai",
		Model:       "static-integration-model",
		Temperature: 0.2,
		MaxTokens:   512,
		TimeoutSec:  30,
	}))

	webControlRuntime, err := service.BuildWebControlRuntime(repos)
	require.NoError(t, err)
	rewriteRuntime, err := service.BuildRewriteRuntime(repos)
	require.NoError(t, err)
	formattingSvc := service.NewFormattingPipelineService(repos.DraftRepo, repos.AssetRepo, repos.WorkspaceRepo, repos.Formatter).WithRenderedDir(repos.RenderedDir)

	cfg := config.DefaultConfig()
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 0
	configLoader := config.NewLoader("")
	configLoader.SetCurrent(cfg)

	provider := &httpserver.Provider{
		ContentSvc:         service.NewContentService(repos.ArticleRepo, repos.PublishRepo),
		TemplateSvc:        service.NewTemplateService(repos.TemplateRepo),
		DraftSvc:           service.NewDraftService(repos.DraftRepo),
		FormattingSvc:      formattingSvc,
		AutomationSvc:      service.NewAutomationService(service.NewWorkspaceConfigService(workspaceinfra.NewLoader(), workspaceinfra.NewValidator()), service.NewIngestionPipelineService(repos.IngestionRepo, repos.WorkspaceRepo, repos.BundleImportTxStarter, workspaceinfra.NewLoader()), nil, nil),
		WorkspaceSvc:       service.NewWorkspaceArticleService(repos.WorkspaceRepo),
		JobSvc:             service.NewJobService(repos.JobRepo, repos.JobEventRepo, noopJobExecutor{}),
		ReviewSvc:          service.NewReviewService(repos.ReviewRepo, repos.WorkspaceRepo),
		PublishSvc:         service.NewPublishGateService(repos.ReviewRepo, repos.AssetRepo, repos.DraftRepo, repos.PublishRepo, repos.WorkspaceRepo, map[string]service.PublisherProvider{"wechat": integrationPublishProviderStub{}}),
		RewriteRuntime:     rewriteRuntime,
		WebControlRuntime:  webControlRuntime,
		WorkflowEngine:     service.NewWorkflowEngine(),
		ConfigLoader:       configLoader,
		SourceDocumentRepo: repos.SourceDocumentRepo,
		RewriteRunRepo:     repos.RewritePipelineRunRepo,
		RewriteStageRepo:   repos.RewriteStageRunRepo,
		AuditLogRepo:       repos.AuditLogRepo,
		WorkspaceRoot:      root,
	}

	server := httpserver.NewServer(cfg, provider)
	httpTestServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpTestServer.Close)

	return root, repos, httpTestServer.URL
}
