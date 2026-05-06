package sqlite

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

var _ repo.WorkflowDefinitionRepo = (*workflowDefinitionRepo)(nil)

type workflowDefinitionRepo struct{ db *sql.DB }

func (r *workflowDefinitionRepo) Upsert(ctx context.Context, workflow *domain.WorkflowDefinition) error {
	if err := workflow.Validate(); err != nil {
		return err
	}
	workflow.UpdatedAt = time.Now().UTC()
	nodesJSON, err := json.Marshal(workflow.Nodes)
	if err != nil {
		return fmt.Errorf("marshal workflow definition nodes: %w", err)
	}
	edgesJSON, err := json.Marshal(workflow.Edges)
	if err != nil {
		return fmt.Errorf("marshal workflow definition edges: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO workflow_definitions (id, name, description, version, enabled, entry_node_id, nodes_json, edges_json, updated_by, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			version = excluded.version,
			enabled = excluded.enabled,
			entry_node_id = excluded.entry_node_id,
			nodes_json = excluded.nodes_json,
			edges_json = excluded.edges_json,
			updated_by = excluded.updated_by,
			updated_at = excluded.updated_at
	`, workflow.ID, workflow.Name, workflow.Description, workflow.Version, boolToInt(workflow.Enabled), workflow.EntryNodeID, string(nodesJSON), string(edgesJSON), workflow.UpdatedBy, workflow.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("upsert workflow definition: %w", err)
	}
	return nil
}

func (r *workflowDefinitionRepo) GetByID(ctx context.Context, id string) (*domain.WorkflowDefinition, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, description, version, enabled, entry_node_id, nodes_json, edges_json, updated_by, updated_at FROM workflow_definitions WHERE id = ?`, id)
	workflow, err := scanWorkflowDefinition(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NewNotFoundErr("workflow_definition", id)
		}
		return nil, err
	}
	return workflow, nil
}

func (r *workflowDefinitionRepo) List(ctx context.Context, limit int) ([]domain.WorkflowDefinition, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, description, version, enabled, entry_node_id, nodes_json, edges_json, updated_by, updated_at FROM workflow_definitions ORDER BY updated_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query workflow definitions: %w", err)
	}
	defer rows.Close()
	items := []domain.WorkflowDefinition{}
	for rows.Next() {
		workflow, err := scanWorkflowDefinition(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *workflow)
	}
	return items, rows.Err()
}

func (r *workflowDefinitionRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM workflow_definitions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete workflow definition: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete workflow definition rows affected: %w", err)
	}
	if affected == 0 {
		return domain.NewNotFoundErr("workflow_definition", id)
	}
	return nil
}

type workflowDefinitionScanner interface{ Scan(dest ...any) error }

func scanWorkflowDefinition(row workflowDefinitionScanner) (*domain.WorkflowDefinition, error) {
	var workflow domain.WorkflowDefinition
	var enabled int
	var nodesJSON, edgesJSON, updatedAt string
	if err := row.Scan(&workflow.ID, &workflow.Name, &workflow.Description, &workflow.Version, &enabled, &workflow.EntryNodeID, &nodesJSON, &edgesJSON, &workflow.UpdatedBy, &updatedAt); err != nil {
		return nil, err
	}
	workflow.Enabled = enabled == 1
	if err := json.Unmarshal([]byte(nodesJSON), &workflow.Nodes); err != nil {
		return nil, fmt.Errorf("decode workflow definition nodes: %w", err)
	}
	if err := json.Unmarshal([]byte(edgesJSON), &workflow.Edges); err != nil {
		return nil, fmt.Errorf("decode workflow definition edges: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("decode workflow definition updated_at: %w", err)
	}
	workflow.UpdatedAt = parsedUpdatedAt
	if workflow.Nodes == nil {
		workflow.Nodes = []domain.WorkflowNode{}
	}
	if workflow.Edges == nil {
		workflow.Edges = []domain.WorkflowEdge{}
	}
	return &workflow, nil
}
