package sqlite

import (
	"content-hub/domain"
	"content-hub/pkg/id"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderExposesWorkflowTemplateRepos(t *testing.T) {
	provider := newRuntimeProvider(t)

	require.NotNil(t, provider.WorkflowDefinitionRepo())
	require.NotNil(t, provider.TemplateDefinitionRepo())
}

func TestWorkflowDefinitionRepoCreateAndGet(t *testing.T) {
	provider := newRuntimeProvider(t)
	wf := &domain.WorkflowDefinition{
		ID:          id.New(),
		Name:        "默认流程",
		Description: "main workflow",
		Version:     "v1",
		Enabled:     true,
		EntryNodeID: "start-1",
		Nodes: []domain.WorkflowNode{{
			ID:         "start-1",
			Type:       "start",
			Name:       "开始",
			ConfigJSON: `{"mode":"auto"}`,
		}},
		Edges: []domain.WorkflowEdge{{
			FromNodeID: "start-1",
			ToNodeID:   "start-1",
			Condition:  "always",
			Priority:   1,
		}},
		UpdatedBy: "tester",
	}

	require.NoError(t, provider.WorkflowDefinitionRepo().Upsert(t.Context(), wf))

	stored, err := provider.WorkflowDefinitionRepo().GetByID(t.Context(), wf.ID)
	require.NoError(t, err)
	require.Equal(t, wf.ID, stored.ID)
	require.Equal(t, wf.Name, stored.Name)
	require.Equal(t, wf.EntryNodeID, stored.EntryNodeID)
	require.Len(t, stored.Nodes, 1)
	require.Equal(t, wf.Nodes[0].ConfigJSON, stored.Nodes[0].ConfigJSON)
	require.Len(t, stored.Edges, 1)
	require.Equal(t, wf.Edges[0].Condition, stored.Edges[0].Condition)
	require.Equal(t, wf.UpdatedBy, stored.UpdatedBy)
	require.False(t, stored.UpdatedAt.IsZero())
}

func TestWorkflowDefinitionRepoListOrdersLatestUpdateFirst(t *testing.T) {
	provider := newRuntimeProvider(t)
	older := &domain.WorkflowDefinition{
		ID:          id.New(),
		Name:        "旧流程",
		Version:     "v1",
		EntryNodeID: "start-old",
		Nodes:       []domain.WorkflowNode{{ID: "start-old", Type: "start", Name: "开始"}},
	}
	newer := &domain.WorkflowDefinition{
		ID:          id.New(),
		Name:        "新流程",
		Version:     "v2",
		EntryNodeID: "start-new",
		Nodes:       []domain.WorkflowNode{{ID: "start-new", Type: "start", Name: "开始"}},
	}

	require.NoError(t, provider.WorkflowDefinitionRepo().Upsert(t.Context(), older))
	require.NoError(t, provider.WorkflowDefinitionRepo().Upsert(t.Context(), newer))

	list, err := provider.WorkflowDefinitionRepo().List(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, newer.ID, list[0].ID)
	require.Equal(t, older.ID, list[1].ID)
}

func TestWorkflowDefinitionRepoDeleteRemovesStoredDefinition(t *testing.T) {
	provider := newRuntimeProvider(t)
	wf := &domain.WorkflowDefinition{
		ID:          id.New(),
		Name:        "待删除流程",
		Version:     "v1",
		EntryNodeID: "start-1",
		Nodes:       []domain.WorkflowNode{{ID: "start-1", Type: "start", Name: "开始"}},
	}

	require.NoError(t, provider.WorkflowDefinitionRepo().Upsert(t.Context(), wf))
	require.NoError(t, provider.WorkflowDefinitionRepo().Delete(t.Context(), wf.ID))

	_, err := provider.WorkflowDefinitionRepo().GetByID(t.Context(), wf.ID)
	require.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrNotFound, appErr.Code)

	list, err := provider.WorkflowDefinitionRepo().List(t.Context(), 10)
	require.NoError(t, err)
	require.Empty(t, list)
}

func TestTemplateDefinitionRepoUpsertRejectsInvalidVariablesJSON(t *testing.T) {
	provider := newRuntimeProvider(t)
	tpl := &domain.TemplateDefinition{
		ID:            id.New(),
		Name:          "无效变量模板",
		Type:          "prompt",
		Version:       "v1",
		Enabled:       true,
		Content:       "标题：{{title}}",
		VariablesJSON: []byte(`{"required":[}`),
		UpdatedBy:     "tester",
	}

	err := provider.TemplateDefinitionRepo().Upsert(t.Context(), tpl)
	require.Error(t, err)
}

func TestTemplateDefinitionRepoRoundTripsValidVariablesJSON(t *testing.T) {
	provider := newRuntimeProvider(t)
	tpl := &domain.TemplateDefinition{
		ID:            id.New(),
		Name:          "标题模板",
		Type:          "prompt",
		Version:       "v1",
		Enabled:       true,
		Content:       "标题：{{title}}",
		VariablesJSON: []byte(`{"required":["title"]}`),
		UpdatedBy:     "tester",
	}

	require.NoError(t, provider.TemplateDefinitionRepo().Upsert(t.Context(), tpl))

	stored, err := provider.TemplateDefinitionRepo().GetByID(t.Context(), tpl.ID)
	require.NoError(t, err)
	require.Equal(t, tpl.Content, stored.Content)
	require.Equal(t, tpl.VariablesJSON, stored.VariablesJSON)
	require.False(t, stored.UpdatedAt.IsZero())

	list, err := provider.TemplateDefinitionRepo().List(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, tpl.ID, list[0].ID)
	require.Equal(t, tpl.Name, list[0].Name)
}

func TestTemplateDefinitionRepoNormalizesEmptyVariablesJSON(t *testing.T) {
	provider := newRuntimeProvider(t)
	tpl := &domain.TemplateDefinition{
		ID:        id.New(),
		Name:      "空变量模板",
		Type:      "prompt",
		Version:   "v1",
		Enabled:   true,
		Content:   "正文：{{body}}",
		UpdatedBy: "tester",
	}

	require.NoError(t, provider.TemplateDefinitionRepo().Upsert(t.Context(), tpl))
	require.Equal(t, []byte(`{}`), tpl.VariablesJSON)

	stored, err := provider.TemplateDefinitionRepo().GetByID(t.Context(), tpl.ID)
	require.NoError(t, err)
	require.Equal(t, []byte(`{}`), stored.VariablesJSON)
}

func TestTemplateDefinitionRepoDeleteRemovesStoredDefinition(t *testing.T) {
	provider := newRuntimeProvider(t)
	tpl := &domain.TemplateDefinition{
		ID:            id.New(),
		Name:          "待删除模板",
		Type:          "prompt",
		Version:       "v1",
		Enabled:       true,
		Content:       "正文：{{body}}",
		VariablesJSON: []byte(`{"body":"string"}`),
	}

	require.NoError(t, provider.TemplateDefinitionRepo().Upsert(t.Context(), tpl))
	require.NoError(t, provider.TemplateDefinitionRepo().Delete(t.Context(), tpl.ID))

	_, err := provider.TemplateDefinitionRepo().GetByID(t.Context(), tpl.ID)
	require.Error(t, err)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrNotFound, appErr.Code)

	list, err := provider.TemplateDefinitionRepo().List(t.Context(), 10)
	require.NoError(t, err)
	require.Empty(t, list)
}
