package service

import (
	"content-hub/pkg/id"
	"strings"
)

type WorkflowTokenRouteLineage struct {
	SourceNodeID   string
	SelectedEdgeID string
	SelectedNodeID string
}

type WorkflowToken struct {
	ID            string
	NodeID        string
	ParentTokenID string
	OriginTokenID string
	OriginRoute   WorkflowTokenRouteLineage
	Branch        *WorkflowBranchContext
}

func newWorkflowRootToken(nodeID string) *WorkflowToken {
	root := &WorkflowToken{
		ID:     id.New(),
		NodeID: strings.TrimSpace(nodeID),
	}
	root.OriginTokenID = root.ID
	return root
}

func (t *WorkflowToken) Child(nodeID string, route WorkflowTokenRouteLineage) *WorkflowToken {
	if t == nil {
		return newWorkflowRootToken(nodeID)
	}
	originRoute := t.OriginRoute
	if originRoute == (WorkflowTokenRouteLineage{}) {
		originRoute = WorkflowTokenRouteLineage{
			SourceNodeID:   strings.TrimSpace(route.SourceNodeID),
			SelectedEdgeID: strings.TrimSpace(route.SelectedEdgeID),
			SelectedNodeID: strings.TrimSpace(route.SelectedNodeID),
		}
	}
	return &WorkflowToken{
		ID:            id.New(),
		NodeID:        strings.TrimSpace(nodeID),
		ParentTokenID: t.ID,
		OriginTokenID: t.OriginTokenID,
		OriginRoute:   originRoute,
		Branch:        cloneWorkflowBranchContext(t.Branch),
	}
}
