package domain

import (
	"content-hub/pkg/id"
	"strings"
	"time"
)

const (
	WorkflowRunPending   = "pending"
	WorkflowRunRunning   = "running"
	WorkflowRunSucceeded = "succeeded"
	WorkflowRunFailed    = "failed"

	WorkflowNodeExecutionPending   = "pending"
	WorkflowNodeExecutionRunning   = "running"
	WorkflowNodeExecutionSucceeded = "succeeded"
	WorkflowNodeExecutionFailed    = "failed"

	WorkflowCheckpointStateActive   = "active"
	WorkflowCheckpointStateTerminal = "terminal"

	WorkflowFailureClassTransient = "transient"
	WorkflowFailureClassPermanent = "permanent"
	WorkflowFailureClassSystem    = "system"
	WorkflowFailureClassCanceled  = "canceled"
)

type WorkflowRunSpec struct {
	WorkflowID         string
	WorkflowVersion    string
	EntryNodeID        string
	WorkspaceArticleID string
	Metadata           map[string]any
}

type WorkflowRun struct {
	ID                     string         `json:"id"`
	WorkflowID             string         `json:"workflow_id"`
	WorkflowVersion        string         `json:"workflow_version"`
	WorkspaceArticleID     string         `json:"workspace_article_id"`
	Status                 string         `json:"status"`
	CurrentNodeID          string         `json:"current_node_id"`
	StartedAt              time.Time      `json:"started_at"`
	CompletedAt            *time.Time     `json:"completed_at"`
	ErrorSummary           string         `json:"error_summary"`
	FinalFailureClass      string         `json:"final_failure_class"`
	Resumable              bool           `json:"resumable"`
	ResumeFromCheckpointID string         `json:"resume_from_checkpoint_id"`
	Metadata               map[string]any `json:"metadata"`
}

type WorkflowNodeExecution struct {
	ID           string         `json:"id"`
	WorkflowRunID string        `json:"workflow_run_id"`
	NodeID       string         `json:"node_id"`
	NodeType     string         `json:"node_type"`
	Status       string         `json:"status"`
	Attempt      int            `json:"attempt"`
	InputJSON    string         `json:"input_json"`
	OutputJSON   string         `json:"output_json"`
	ErrorSummary string         `json:"error_summary"`
	FailureClass string         `json:"failure_class"`
	StartedAt    time.Time      `json:"started_at"`
	CompletedAt  *time.Time     `json:"completed_at"`
	Metadata     map[string]any `json:"metadata"`
}

type WorkflowCheckpoint struct {
	ID                     string         `json:"id"`
	WorkflowRunID          string         `json:"workflow_run_id"`
	NodeExecutionID        string         `json:"node_execution_id"`
	NodeID                 string         `json:"node_id"`
	State                  string         `json:"state"`
	Resumable              bool           `json:"resumable"`
	ResumeToken            string         `json:"resume_token"`
	PayloadJSON            string         `json:"payload_json"`
	FailureClass           string         `json:"failure_class"`
	CreatedAt              time.Time      `json:"created_at"`
	ConsumedAt             *time.Time     `json:"consumed_at"`
	Metadata               map[string]any `json:"metadata"`
}

var workflowRunStatuses = map[string]struct{}{
	WorkflowRunPending:   {},
	WorkflowRunRunning:   {},
	WorkflowRunSucceeded: {},
	WorkflowRunFailed:    {},
}

var workflowNodeExecutionStatuses = map[string]struct{}{
	WorkflowNodeExecutionPending:   {},
	WorkflowNodeExecutionRunning:   {},
	WorkflowNodeExecutionSucceeded: {},
	WorkflowNodeExecutionFailed:    {},
}

var workflowFailureClasses = map[string]struct{}{
	WorkflowFailureClassTransient: {},
	WorkflowFailureClassPermanent: {},
	WorkflowFailureClassSystem:    {},
	WorkflowFailureClassCanceled:  {},
}

func NewWorkflowRun(spec WorkflowRunSpec) (*WorkflowRun, error) {
	if strings.TrimSpace(spec.WorkflowID) == "" {
		return nil, NewValidationErr("workflow id is required", nil)
	}
	if strings.TrimSpace(spec.WorkflowVersion) == "" {
		return nil, NewValidationErr("workflow version is required", nil)
	}
	if strings.TrimSpace(spec.EntryNodeID) == "" {
		return nil, NewValidationErr("workflow entry node id is required", nil)
	}

	now := time.Now().UTC()
	metadata := spec.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	return &WorkflowRun{
		ID:                 id.New(),
		WorkflowID:         spec.WorkflowID,
		WorkflowVersion:    spec.WorkflowVersion,
		WorkspaceArticleID: spec.WorkspaceArticleID,
		Status:             WorkflowRunPending,
		CurrentNodeID:      spec.EntryNodeID,
		StartedAt:          now,
		Metadata:           metadata,
	}, nil
}

func (r WorkflowRun) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return NewValidationErr("workflow run id is required", nil)
	}
	if strings.TrimSpace(r.WorkflowID) == "" {
		return NewValidationErr("workflow run workflow id is required", nil)
	}
	if strings.TrimSpace(r.WorkflowVersion) == "" {
		return NewValidationErr("workflow run workflow version is required", nil)
	}
	if _, ok := workflowRunStatuses[r.Status]; !ok {
		return NewValidationErr("unsupported workflow run status", nil)
	}
	if r.FinalFailureClass != "" {
		if _, ok := workflowFailureClasses[r.FinalFailureClass]; !ok {
			return NewValidationErr("unsupported workflow run failure class", nil)
		}
	}
	return nil
}

func (e WorkflowNodeExecution) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return NewValidationErr("workflow node execution id is required", nil)
	}
	if strings.TrimSpace(e.WorkflowRunID) == "" {
		return NewValidationErr("workflow node execution workflow run id is required", nil)
	}
	if strings.TrimSpace(e.NodeID) == "" {
		return NewValidationErr("workflow node execution node id is required", nil)
	}
	if strings.TrimSpace(e.NodeType) == "" {
		return NewValidationErr("workflow node execution node type is required", nil)
	}
	if _, ok := workflowNodeExecutionStatuses[e.Status]; !ok {
		return NewValidationErr("unsupported workflow node execution status", nil)
	}
	if e.FailureClass != "" {
		if _, ok := workflowFailureClasses[e.FailureClass]; !ok {
			return NewValidationErr("unsupported workflow node execution failure class", nil)
		}
	}
	return nil
}

func (r *WorkflowRun) MarkSucceeded() error {
	if r == nil {
		return NewValidationErr("workflow run is required", nil)
	}
	if r.Status == WorkflowRunSucceeded || r.Status == WorkflowRunFailed {
		return NewConflictErr("workflow run is already in a terminal state")
	}
	if strings.TrimSpace(r.ID) == "" {
		return NewValidationErr("workflow run id is required", nil)
	}
	if strings.TrimSpace(r.WorkflowID) == "" {
		return NewValidationErr("workflow run workflow id is required", nil)
	}
	if strings.TrimSpace(r.WorkflowVersion) == "" {
		return NewValidationErr("workflow run workflow version is required", nil)
	}

	now := time.Now().UTC()
	r.Status = WorkflowRunSucceeded
	r.CompletedAt = &now
	r.ErrorSummary = ""
	r.FinalFailureClass = ""
	r.Resumable = false
	r.ResumeFromCheckpointID = ""
	return nil
}

func (c WorkflowCheckpoint) Validate() error {
	if c.State != WorkflowCheckpointStateActive && c.State != WorkflowCheckpointStateTerminal {
		return NewValidationErr("workflow checkpoint state must be active or terminal", nil)
	}
	return nil
}
