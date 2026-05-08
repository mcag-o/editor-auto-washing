package service

import "content-hub/pkg/repo"

type AuditLogQuery struct {
	Resource      string
	WorkflowRunID string
	ActionPrefix  string
	ResourceID    string
}

func (q AuditLogQuery) toRepoQuery() repo.AuditLogQuery {
	return repo.AuditLogQuery{
		Resource:      q.Resource,
		WorkflowRunID: q.WorkflowRunID,
		ActionPrefix:  q.ActionPrefix,
		ResourceID:    q.ResourceID,
	}
}
