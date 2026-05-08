package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowTokenTracksParentAndOriginRouteLineage(t *testing.T) {
	root := newWorkflowRootToken("start")
	child := root.Child("approved", WorkflowTokenRouteLineage{
		SourceNodeID:   "router",
		SelectedEdgeID: "router->approved@1[payload.route == approved]",
		SelectedNodeID: "approved",
	})

	require.NotNil(t, root)
	require.NotNil(t, child)
	assert.NotEmpty(t, root.ID)
	assert.NotEmpty(t, child.ID)
	assert.NotEqual(t, root.ID, child.ID)
	assert.Equal(t, root.ID, child.ParentTokenID)
	assert.Equal(t, root.ID, child.OriginTokenID)
	assert.Equal(t, "approved", child.NodeID)
	assert.Equal(t, WorkflowTokenRouteLineage{
		SourceNodeID:   "router",
		SelectedEdgeID: "router->approved@1[payload.route == approved]",
		SelectedNodeID: "approved",
	}, child.OriginRoute)
	assert.Equal(t, WorkflowTokenRouteLineage{}, root.OriginRoute)
	assert.Equal(t, root.ID, root.OriginTokenID)
}
