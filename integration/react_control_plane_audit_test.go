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

func TestReactAuditPageDataPathRemainsStable(t *testing.T) {
	_, _, _, _, serverURL := newWebControlPlaneIntegrationServer(t)

	rootResp, err := http.Get(serverURL + "/audit")
	require.NoError(t, err)
	defer rootResp.Body.Close()
	require.Equal(t, http.StatusOK, rootResp.StatusCode)
	require.Contains(t, rootResp.Header.Get("Content-Type"), "text/html")
	rootBody, err := io.ReadAll(rootResp.Body)
	require.NoError(t, err)
	require.Contains(t, string(rootBody), "<title>Content Hub Control Plane</title>")
	require.Contains(t, string(rootBody), `<div id="root"></div>`)
	require.Contains(t, string(rootBody), "/ui/assets/")

	assetPath := regexp.MustCompile(`/ui/assets/[^"]+\.js`).FindString(string(rootBody))
	require.NotEmpty(t, assetPath)

	assetResp, err := http.Get(serverURL + assetPath)
	require.NoError(t, err)
	defer assetResp.Body.Close()
	require.Equal(t, http.StatusOK, assetResp.StatusCode)
	require.Contains(t, assetResp.Header.Get("Content-Type"), "javascript")

	pasteResp := postJSON(t, serverURL+"/api/intake/paste", map[string]any{
		"title": "Audit Page Source",
		"body":  "Body pasted to create audit entries for the audit page.",
	})
	defer pasteResp.Body.Close()
	require.Equal(t, http.StatusCreated, pasteResp.StatusCode)

	var createdDoc reactBrowserArticlePayload
	require.NoError(t, json.NewDecoder(pasteResp.Body).Decode(&createdDoc))

	startResp := postJSON(t, serverURL+"/api/system/start", map[string]any{
		"concurrency_limit": 1,
	})
	defer startResp.Body.Close()
	require.Equal(t, http.StatusOK, startResp.StatusCode)

	auditListResp, err := http.Get(serverURL + "/api/audit")
	require.NoError(t, err)
	defer auditListResp.Body.Close()
	require.Equal(t, http.StatusOK, auditListResp.StatusCode)

	var auditList struct {
		Data []domain.AuditLog `json:"data"`
	}
	require.NoError(t, json.NewDecoder(auditListResp.Body).Decode(&auditList))
	require.Len(t, auditList.Data, 2)
	require.Equal(t, "control_plane.started", auditList.Data[0].Action)
	require.Equal(t, "web_intake.create_from_paste", auditList.Data[1].Action)

	auditByAction := map[string]domain.AuditLog{}
	for _, entry := range auditList.Data {
		auditByAction[entry.Action] = entry
	}

	pasteAudit, ok := auditByAction["web_intake.create_from_paste"]
	require.True(t, ok)
	require.Equal(t, createdDoc.ID, pasteAudit.ResourceID)
	require.Equal(t, "success", pasteAudit.Result)

	startAudit, ok := auditByAction["control_plane.started"]
	require.True(t, ok)
	require.Equal(t, "success", startAudit.Result)
	require.Equal(t, auditList.Data[0].ID, startAudit.ID)

	auditDetailResp, err := http.Get(serverURL + "/api/audit/" + startAudit.ID)
	require.NoError(t, err)
	defer auditDetailResp.Body.Close()
	require.Equal(t, http.StatusOK, auditDetailResp.StatusCode)

	var auditDetail domain.AuditLog
	require.NoError(t, json.NewDecoder(auditDetailResp.Body).Decode(&auditDetail))
	require.Equal(t, startAudit.ID, auditDetail.ID)
	require.Equal(t, "control_plane.started", auditDetail.Action)
	require.Equal(t, "success", auditDetail.Result)
	limit, ok := auditDetail.Metadata["concurrency_limit"].(float64)
	require.True(t, ok)
	require.Equal(t, 1.0, limit)
}
