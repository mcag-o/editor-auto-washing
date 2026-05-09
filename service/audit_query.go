package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"strings"
)

const (
	workflowRunPauseAuditAction  = "web_control.workflow_run.pause"
	workflowRunResumeAuditAction = "web_control.workflow_run.resume"
	workflowTaskAuditActionRoot  = "web_control.workflow_task"
)

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

func isWorkflowPauseAuditAction(action string) bool {
	action = strings.TrimSpace(action)
	if action == workflowRunPauseAuditAction || action == workflowRunResumeAuditAction {
		return true
	}
	return action == workflowTaskAuditActionRoot || strings.HasPrefix(action, workflowTaskAuditActionRoot+".")
}

func filterWorkflowPauseAuditLogs(auditLogs []domain.AuditLog, workflowRunID string) []domain.AuditLog {
	workflowRunID = strings.TrimSpace(workflowRunID)
	if len(auditLogs) == 0 || workflowRunID == "" {
		return []domain.AuditLog{}
	}
	filtered := make([]domain.AuditLog, 0, len(auditLogs))
	for _, log := range auditLogs {
		if !isWorkflowPauseAuditLogForRun(log, workflowRunID) {
			continue
		}
		filtered = append(filtered, log)
	}
	return filtered
}

func isWorkflowPauseAuditLogForRun(log domain.AuditLog, workflowRunID string) bool {
	if !isWorkflowPauseAuditAction(log.Action) {
		return false
	}
	if workflowRunID == "" {
		return false
	}
	if strings.TrimSpace(log.Resource) == "workflow_run" && strings.TrimSpace(log.ResourceID) == workflowRunID {
		return true
	}
	return strings.TrimSpace(domain.DraftString(log.Metadata["workflow_run_id"])) == workflowRunID
}
