package domain

import (
	"strings"
	"time"
)

type WorkflowDefinition struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	Enabled     bool           `json:"enabled"`
	EntryNodeID string         `json:"entry_node_id"`
	Nodes       []WorkflowNode `json:"nodes"`
	Edges       []WorkflowEdge `json:"edges"`
	UpdatedBy   string         `json:"updated_by"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type WorkflowNode struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	ConfigJSON string `json:"config_json"`
}

type WorkflowEdge struct {
	FromNodeID string `json:"from_node_id"`
	ToNodeID   string `json:"to_node_id"`
	Condition  string `json:"condition"`
	Priority   int    `json:"priority"`
}

func (w WorkflowDefinition) Validate() error {
	if strings.TrimSpace(w.Name) == "" {
		return NewValidationErr("workflow name is required", nil)
	}
	if len(w.Nodes) == 0 {
		return NewValidationErr("workflow nodes are required", nil)
	}
	if strings.TrimSpace(w.EntryNodeID) == "" {
		return NewValidationErr("workflow entry node is required", nil)
	}

	nodeIDs := make(map[string]struct{}, len(w.Nodes))
	for _, node := range w.Nodes {
		if err := node.Validate(); err != nil {
			return err
		}
		if _, exists := nodeIDs[node.ID]; exists {
			return NewValidationErr("workflow node id must be unique", nil)
		}
		nodeIDs[node.ID] = struct{}{}
	}

	if _, ok := nodeIDs[w.EntryNodeID]; !ok {
		return NewValidationErr("workflow entry node must reference an existing node", nil)
	}

	for _, edge := range w.Edges {
		if err := edge.Validate(nodeIDs); err != nil {
			return err
		}
	}

	return nil
}

func (n WorkflowNode) Validate() error {
	if strings.TrimSpace(n.ID) == "" {
		return NewValidationErr("workflow node id is required", nil)
	}
	if strings.TrimSpace(n.Type) == "" {
		return NewValidationErr("workflow node type is required", nil)
	}
	if strings.TrimSpace(n.Name) == "" {
		return NewValidationErr("workflow node name is required", nil)
	}
	return nil
}

func (e WorkflowEdge) Validate(nodeIDs map[string]struct{}) error {
	if strings.TrimSpace(e.FromNodeID) == "" {
		return NewValidationErr("workflow edge from node is required", nil)
	}
	if strings.TrimSpace(e.ToNodeID) == "" {
		return NewValidationErr("workflow edge to node is required", nil)
	}
	if _, ok := nodeIDs[e.FromNodeID]; !ok {
		return NewValidationErr("workflow edge from node must reference an existing node", nil)
	}
	if _, ok := nodeIDs[e.ToNodeID]; !ok {
		return NewValidationErr("workflow edge to node must reference an existing node", nil)
	}
	return nil
}
