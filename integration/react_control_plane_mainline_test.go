package integration

import (
	"content-hub/domain"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReactControlPlanePasteToRenderedResultWithWorkflowTemplate(t *testing.T) {
	_, repos, serverURL := newWebControlPlaneIntegrationServer(t)

	rootResp, err := http.Get(serverURL + "/")
	require.NoError(t, err)
	defer rootResp.Body.Close()
	require.Equal(t, http.StatusOK, rootResp.StatusCode)
	require.Contains(t, rootResp.Header.Get("Content-Type"), "text/html")
	rootBody, err := io.ReadAll(rootResp.Body)
	require.NoError(t, err)
	require.Contains(t, string(rootBody), "<title>Content Hub Control Plane</title>")
	require.Contains(t, string(rootBody), `<div id="root"></div>`)
	require.Contains(t, string(rootBody), "/ui/assets/")
	require.NotContains(t, string(rootBody), "图工作流控制台")

	assetPath := regexp.MustCompile(`/ui/assets/[^"]+\.js`).FindString(string(rootBody))
	require.NotEmpty(t, assetPath)

	assetResp, err := http.Get(serverURL + assetPath)
	require.NoError(t, err)
	defer assetResp.Body.Close()
	require.Equal(t, http.StatusOK, assetResp.StatusCode)
	require.Contains(t, assetResp.Header.Get("Content-Type"), "javascript")

	templateResp := postJSON(t, serverURL+"/api/templates", map[string]any{
		"id":             "react-mainline-template-definition",
		"name":           "React Mainline Prompt",
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
		"id":            "react-mainline-workflow-template",
		"name":          "React Mainline Workflow",
		"description":   "React control plane mainline workflow",
		"version":       "v1",
		"enabled":       true,
		"entry_node_id": "generate_draft",
		"nodes": []map[string]any{{
			"id":          "generate_draft",
			"type":        "rewrite_stage",
			"name":        "generate_draft",
			"config_json": `{"stage_name":"generate_draft","prompt_ref":"generate_draft_alt@v2","vars":{"workflow_marker":"react-mainline"}}`,
		}},
		"edges":      []map[string]any{},
		"updated_by": "integration-test",
	})
	defer workflowResp.Body.Close()
	require.Equal(t, http.StatusCreated, workflowResp.StatusCode)

	pasteResp := postJSON(t, serverURL+"/api/intake/paste", map[string]any{
		"title": "React Control Plane Source",
		"body":  "Body pasted through the React control plane mainline.",
	})
	defer pasteResp.Body.Close()
	require.Equal(t, http.StatusCreated, pasteResp.StatusCode)

	var createdDoc domain.SourceDocument
	require.NoError(t, json.NewDecoder(pasteResp.Body).Decode(&createdDoc))
	require.Equal(t, domain.SourceDocumentStatusPending, createdDoc.Status)

	assignResp := postJSON(t, serverURL+"/api/articles/"+createdDoc.ID+"/workflow-template", map[string]any{
		"workflow_template_id": "react-mainline-workflow-template",
	})
	defer assignResp.Body.Close()
	require.Equal(t, http.StatusOK, assignResp.StatusCode)

	startResp := postJSON(t, serverURL+"/api/system/start", map[string]any{
		"concurrency_limit": 1,
	})
	defer startResp.Body.Close()
	require.Equal(t, http.StatusOK, startResp.StatusCode)

	statusResp, err := http.Get(serverURL + "/api/system/status")
	require.NoError(t, err)
	defer statusResp.Body.Close()
	require.Equal(t, http.StatusOK, statusResp.StatusCode)

	var systemStatus domain.SystemControlState
	require.NoError(t, json.NewDecoder(statusResp.Body).Decode(&systemStatus))
	require.Equal(t, domain.SystemStateStopped, systemStatus.State)

	articleResp, err := http.Get(serverURL + "/api/articles/" + createdDoc.ID)
	require.NoError(t, err)
	defer articleResp.Body.Close()
	require.Equal(t, http.StatusOK, articleResp.StatusCode)

	var article domain.SourceDocument
	require.NoError(t, json.NewDecoder(articleResp.Body).Decode(&article))
	require.Equal(t, domain.SourceDocumentStatusCompleted, article.Status)
	require.Equal(t, "react-mainline-workflow-template", article.Metadata["workflow_template_id"])
	require.NotEmpty(t, article.WorkspaceArticleID)
	require.NotEmpty(t, article.RewriteRunID)
	require.NotNil(t, article.CompletedAt)

	articleListResp, err := http.Get(serverURL + "/api/articles")
	require.NoError(t, err)
	defer articleListResp.Body.Close()
	require.Equal(t, http.StatusOK, articleListResp.StatusCode)

	var articleList struct {
		Data []domain.SourceDocument `json:"data"`
	}
	require.NoError(t, json.NewDecoder(articleListResp.Body).Decode(&articleList))
	require.Len(t, articleList.Data, 1)
	require.Equal(t, createdDoc.ID, articleList.Data[0].ID)
	require.Equal(t, domain.SourceDocumentStatusCompleted, articleList.Data[0].Status)

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
	require.Equal(t, domain.SourceDocumentStatusCompleted, stagesPayload.Article.Status)
	require.NotNil(t, stagesPayload.Run)
	require.Equal(t, domain.RewriteRunSucceeded, stagesPayload.Run.Status)
	require.Equal(t, "react-mainline-workflow-template", stagesPayload.Run.Metadata["workflow_template_id"])
	require.Equal(t, "generate_draft_alt@v2", stagesPayload.Run.Metadata["workflow_prompt_ref"])
	require.NotContains(t, stagesPayload.Run.Metadata, "active_token_set")
	require.NotContains(t, stagesPayload.Run.Metadata, "rewrite_workflow_checkpoint")
	routeSummary, ok := stagesPayload.Run.Metadata["workflow_route_latest"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "materialize_draft", routeSummary["node_id"])
	require.Equal(t, "no_match", routeSummary["outcome"])
	require.Empty(t, routeSummary["evaluation_trace"])
	require.Len(t, stagesPayload.Stages, 1)
	require.Equal(t, domain.RewriteStageSucceeded, stagesPayload.Stages[0].Status)

	workspace, err := repos.WorkspaceRepo.GetByID(t.Context(), article.WorkspaceArticleID)
	require.NoError(t, err)
	require.Equal(t, domain.ArticleWorkspaceStatusRendered, workspace.Status)

	draft, err := repos.DraftRepo.GetByID(t.Context(), workspace.ID)
	require.NoError(t, err)
	require.Equal(t, "daily-intelligence-alt", draft.Template)
	require.Equal(t, "Graph Mainline Title", draft.Headline["title"])

	assets, err := repos.AssetRepo.List(t.Context(), draft.ID, "wechat")
	require.NoError(t, err)
	require.Len(t, assets, 1)
	require.FileExists(t, assets[0].ArtifactPath)
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
	require.Contains(t, actions, "web_control.article.workflow_template_assigned")
	require.Contains(t, actions, "control_plane.started")

	require.Equal(t, createdDoc.ID, actions["web_intake.create_from_paste"].ResourceID)
	require.Equal(t, createdDoc.ID, actions["web_control.article.workflow_template_assigned"].ResourceID)
	require.Equal(t, "react-mainline-workflow-template", actions["web_control.article.workflow_template_assigned"].Metadata["workflow_template_id"])
}
