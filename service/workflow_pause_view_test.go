package service

import (
	"content-hub/domain"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowPauseViewSummaryProjectsPausedRunFromCheckpointAndRunState(t *testing.T) {
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{
		WorkflowID:         "workflow-editorial",
		WorkflowVersion:    "v3",
		EntryNodeID:        "draft",
		WorkspaceArticleID: "article-42",
	})
	require.NoError(t, err)
	run.ID = "run-42"
	run.Status = domain.WorkflowRunPaused
	run.CurrentNodeID = "human-review"
	run.ResumeFromCheckpointID = "checkpoint-42"
	run.Metadata = map[string]any{
		workflowPauseSourceMetadataKey: string(WorkflowPauseSourceHumanNode),
		workflowPauseReasonMetadataKey: "awaiting editor approval",
	}

	checkpoint := domain.WorkflowCheckpoint{
		ID:            "checkpoint-42",
		WorkflowRunID: run.ID,
		NodeID:        "human-review",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		CreatedAt:     time.Date(2026, time.May, 9, 7, 0, 0, 0, time.UTC),
		Metadata: map[string]any{
			workflowPauseSourceMetadataKey:             string(WorkflowPauseSourceHumanNode),
			workflowPauseScopeMetadataKey:              string(WorkflowPauseScopeToken),
			workflowPauseReasonMetadataKey:             "awaiting editor approval",
			workflowPauseAllowedResumeModesMetadataKey: []string{string(WorkflowResumeModeContinueToken), string(WorkflowResumeModeReplayFromCheckpoint)},
			"token_id":                               "token-1",
			"node_id":                                "human-review",
		},
	}

	view, err := BuildWorkflowPauseView(run, nil, &checkpoint)

	require.NoError(t, err)
	assert.Equal(t, WorkflowPausedRunSummary{
		RunID:              "run-42",
		Status:             domain.WorkflowRunPaused,
		WorkflowID:         "workflow-editorial",
		WorkflowVersion:    "v3",
		PauseSource:        WorkflowPauseSourceHumanNode,
		PauseReason:        "awaiting editor approval",
		AllowedResumeModes: []WorkflowResumeMode{WorkflowResumeModeContinueToken, WorkflowResumeModeReplayFromCheckpoint},
		AffectedTokenCount: 1,
		AffectedTokenIDs:   []string{"token-1"},
		CurrentNodeIDs:     []string{"human-review"},
		CheckpointID:       "checkpoint-42",
		WorkspaceArticleID: "article-42",
	}, view.Summary)
	require.Len(t, view.TaskItems, 1)
	assert.Empty(t, view.FullAuditRefs)
}

func TestWorkflowPauseViewSummaryCountsAffectedTokens(t *testing.T) {
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{
		WorkflowID:      "workflow-branching",
		WorkflowVersion: "v1",
		EntryNodeID:     "start",
	})
	require.NoError(t, err)
	run.ID = "run-branch-1"
	run.Status = domain.WorkflowRunPaused
	run.CurrentNodeID = "merge-review"
	run.ResumeFromCheckpointID = "checkpoint-branch-1"

	checkpoint := domain.WorkflowCheckpoint{
		ID:            "checkpoint-branch-1",
		WorkflowRunID: run.ID,
		NodeID:        "merge-review",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		Metadata: map[string]any{
			workflowPauseSourceMetadataKey:             string(WorkflowPauseSourceManual),
			workflowPauseScopeMetadataKey:              string(WorkflowPauseScopeRun),
			workflowPauseReasonMetadataKey:             "operator paused parallel review",
			workflowPauseAllowedResumeModesMetadataKey: []any{string(WorkflowResumeModeContinueActiveTokens), string(WorkflowResumeModeReplayFromCheckpoint)},
			workflowActiveTokenSetMetadataKey: []any{
				map[string]any{"token_id": "token-left", "node_id": "review-left"},
				map[string]any{"token_id": "token-right", "node_id": "review-right"},
			},
		},
	}

	view, err := BuildWorkflowPauseView(run, nil, &checkpoint)

	require.NoError(t, err)
	assert.Equal(t, 2, view.Summary.AffectedTokenCount)
	assert.Equal(t, []string{"token-left", "token-right"}, view.Summary.AffectedTokenIDs)
	assert.Equal(t, []string{"review-left", "review-right"}, view.Summary.CurrentNodeIDs)
	assert.Equal(t, []WorkflowResumeMode{WorkflowResumeModeContinueActiveTokens, WorkflowResumeModeReplayFromCheckpoint}, view.Summary.AllowedResumeModes)
	require.Len(t, view.TaskItems, 2)
	assert.Empty(t, view.FullAuditRefs)
}

func TestWorkflowPauseViewSummaryRejectsMissingResumeCheckpoint(t *testing.T) {
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{
		WorkflowID:      "workflow-editorial",
		WorkflowVersion: "v1",
		EntryNodeID:     "draft",
	})
	require.NoError(t, err)
	run.ID = "run-missing-anchor"
	run.Status = domain.WorkflowRunPaused
	run.CurrentNodeID = "human-review"
	run.ResumeFromCheckpointID = "checkpoint-anchor"

	otherCheckpoint := domain.WorkflowCheckpoint{
		ID:            "checkpoint-other",
		WorkflowRunID: run.ID,
		NodeID:        "human-review",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		Metadata: map[string]any{
			workflowPauseSourceMetadataKey:             string(WorkflowPauseSourceHumanNode),
			workflowPauseScopeMetadataKey:              string(WorkflowPauseScopeToken),
			workflowPauseReasonMetadataKey:             "awaiting editor approval",
			workflowPauseAllowedResumeModesMetadataKey: []string{string(WorkflowResumeModeContinueToken)},
			"token_id":                               "token-other",
			"node_id":                                "human-review",
		},
	}

	view, err := BuildWorkflowPauseView(run, nil, &otherCheckpoint)

	require.Error(t, err)
	assert.Equal(t, WorkflowPauseView{}, view)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	assert.Equal(t, domain.ErrValidation, appErr.Code)
	assert.Equal(t, "workflow resume checkpoint not found", appErr.Message)
}

func TestWorkflowPauseViewSummaryDoesNotInventTokenForRunScopedPause(t *testing.T) {
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{
		WorkflowID:      "workflow-editorial",
		WorkflowVersion: "v1",
		EntryNodeID:     "draft",
	})
	require.NoError(t, err)
	run.ID = "run-manual-pause"
	run.Status = domain.WorkflowRunPaused
	run.CurrentNodeID = "policy-gate"
	run.ResumeFromCheckpointID = "checkpoint-manual"

	checkpoint := domain.WorkflowCheckpoint{
		ID:            "checkpoint-manual",
		WorkflowRunID: run.ID,
		NodeID:        "policy-gate",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		Metadata: map[string]any{
			workflowPauseSourceMetadataKey:             string(WorkflowPauseSourceManual),
			workflowPauseScopeMetadataKey:              string(WorkflowPauseScopeRun),
			workflowPauseReasonMetadataKey:             "operator paused run",
			workflowPauseAllowedResumeModesMetadataKey: []string{string(WorkflowResumeModeContinueActiveTokens), string(WorkflowResumeModeReplayFromCheckpoint)},
			workflowPausePayloadMetadataKey: map[string]any{
				"note": "awaiting ops check",
			},
		},
	}

	view, err := BuildWorkflowPauseView(run, nil, &checkpoint)

	require.NoError(t, err)
	assert.Equal(t, 0, view.Summary.AffectedTokenCount)
	assert.Empty(t, view.Summary.AffectedTokenIDs)
	assert.Equal(t, []string{"policy-gate"}, view.Summary.CurrentNodeIDs)
	require.Len(t, view.TaskItems, 1)
	assert.Equal(t, WorkflowPausedTaskItem{
		TaskID:      "run-manual-pause:checkpoint-manual:policy-gate",
		RunID:       "run-manual-pause",
		PauseSource: WorkflowPauseSourceManual,
		PausedAt:     checkpoint.CreatedAt,
		NodeID:      "policy-gate",
		Title:       "Manual review required",
		Summary:     "operator paused run",
		AllowedResumeModes: []WorkflowResumeMode{
			WorkflowResumeModeContinueActiveTokens,
			WorkflowResumeModeReplayFromCheckpoint,
		},
		AvailableActions: []string{"resume", "replay"},
		PausePayloadPreview: map[string]any{
			"note": "awaiting ops check",
		},
	}, view.TaskItems[0])
	assert.Empty(t, view.FullAuditRefs)
}

func TestWorkflowPauseViewProjectsHumanNodeTaskItem(t *testing.T) {
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{
		WorkflowID:      "workflow-editorial",
		WorkflowVersion: "v3",
		EntryNodeID:     "draft",
	})
	require.NoError(t, err)
	run.ID = "run-human-task"
	run.Status = domain.WorkflowRunPaused
	run.CurrentNodeID = "human-review"
	run.ResumeFromCheckpointID = "checkpoint-human-task"

	pausedAt := time.Date(2026, time.May, 9, 7, 30, 0, 0, time.UTC)
	checkpoint := domain.WorkflowCheckpoint{
		ID:            "checkpoint-human-task",
		WorkflowRunID: run.ID,
		NodeID:        "human-review",
		ResumeToken:   "token-human-1",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		CreatedAt:     pausedAt,
		Metadata: map[string]any{
			workflowPauseSourceMetadataKey:             string(WorkflowPauseSourceHumanNode),
			workflowPauseScopeMetadataKey:              string(WorkflowPauseScopeToken),
			workflowPauseReasonMetadataKey:             "awaiting editor approval",
			workflowPauseAllowedResumeModesMetadataKey: []string{string(WorkflowResumeModeContinueToken)},
			workflowPausePayloadMetadataKey: map[string]any{
				"node_id":              "human-review",
				"token_id":             "token-human-1",
				"allowed_resume_modes": []any{string(WorkflowResumeModeContinueToken)},
				"action_schema": map[string]any{
					"type": "approve_or_reject",
				},
				"form_schema": map[string]any{
					"fields": []any{"comment"},
				},
			},
			workflowLatestAuditHintMetadataKey: "editor requested changes",
		},
	}

	view, err := BuildWorkflowPauseView(run, nil, &checkpoint)

	require.NoError(t, err)
	require.Len(t, view.TaskItems, 1)
	assert.Equal(t, WorkflowPausedTaskItem{
		TaskID:      "run-human-task:token-human-1:human-review",
		RunID:       "run-human-task",
		TokenID:     "token-human-1",
		PauseSource: WorkflowPauseSourceHumanNode,
		PausedAt:    pausedAt,
		NodeID:      "human-review",
		Title:       "Human review required",
		Summary:     "awaiting editor approval",
		AllowedResumeModes: []WorkflowResumeMode{
			WorkflowResumeModeContinueToken,
		},
		AvailableActions: []string{"submit"},
		PausePayloadPreview: map[string]any{
			"node_id":              "human-review",
			"token_id":             "token-human-1",
			"allowed_resume_modes": []any{string(WorkflowResumeModeContinueToken)},
		},
		LatestAuditHint: "editor requested changes",
	}, view.TaskItems[0])
}

func TestWorkflowPauseViewProjectsManualPauseTaskItem(t *testing.T) {
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{
		WorkflowID:      "workflow-editorial",
		WorkflowVersion: "v3",
		EntryNodeID:     "draft",
	})
	require.NoError(t, err)
	run.ID = "run-manual-task"
	run.Status = domain.WorkflowRunPaused
	run.CurrentNodeID = "ops-review"
	run.ResumeFromCheckpointID = "checkpoint-manual-task"

	newerPausedAt := time.Date(2026, time.May, 9, 8, 30, 0, 0, time.UTC)
	olderPausedAt := newerPausedAt.Add(-1 * time.Hour)
	newer := domain.WorkflowCheckpoint{
		ID:            "checkpoint-manual-task",
		WorkflowRunID: run.ID,
		NodeID:        "ops-review",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		CreatedAt:     newerPausedAt,
		Metadata: map[string]any{
			workflowPauseSourceMetadataKey:             string(WorkflowPauseSourceManual),
			workflowPauseScopeMetadataKey:              string(WorkflowPauseScopeRun),
			workflowPauseReasonMetadataKey:             "operator paused for compliance check",
			workflowPauseAllowedResumeModesMetadataKey: []string{string(WorkflowResumeModeContinueActiveTokens), string(WorkflowResumeModeReplayFromCheckpoint)},
			workflowPausePayloadMetadataKey: map[string]any{
				"operator_id": "ops-7",
				"ticket_id":   "ticket-55",
			},
			workflowActiveTokenSetMetadataKey: []any{
				map[string]any{"token_id": "token-new", "node_id": "ops-review"},
			},
			workflowLatestAuditHintMetadataKey: "ops escalation pending",
		},
	}
	older := domain.WorkflowCheckpoint{
		ID:            "checkpoint-manual-task-older",
		WorkflowRunID: run.ID,
		NodeID:        "ops-review-old",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		CreatedAt:     olderPausedAt,
		Metadata: map[string]any{
			workflowPauseSourceMetadataKey:             string(WorkflowPauseSourceManual),
			workflowPauseScopeMetadataKey:              string(WorkflowPauseScopeRun),
			workflowPauseReasonMetadataKey:             "older manual pause",
			workflowPauseAllowedResumeModesMetadataKey: []string{string(WorkflowResumeModeContinueActiveTokens)},
			workflowPausePayloadMetadataKey: map[string]any{
				"operator_id": "ops-1",
			},
			workflowActiveTokenSetMetadataKey: []any{
				map[string]any{"token_id": "token-old", "node_id": "ops-review-old"},
			},
		},
	}

	view, err := BuildWorkflowPauseView(run, nil, &older, &newer)

	require.NoError(t, err)
	require.Len(t, view.TaskItems, 2)
	assert.Equal(t, "run-manual-task:token-new:ops-review", view.TaskItems[0].TaskID)
	assert.Equal(t, newerPausedAt, view.TaskItems[0].PausedAt)
	assert.Equal(t, WorkflowPauseSourceManual, view.TaskItems[0].PauseSource)
	assert.Equal(t, []WorkflowResumeMode{WorkflowResumeModeContinueActiveTokens, WorkflowResumeModeReplayFromCheckpoint}, view.TaskItems[0].AllowedResumeModes)
	assert.Equal(t, []string{"resume", "replay"}, view.TaskItems[0].AvailableActions)
	assert.Equal(t, map[string]any{"operator_id": "ops-7", "ticket_id": "ticket-55"}, view.TaskItems[0].PausePayloadPreview)
	assert.Equal(t, "ops escalation pending", view.TaskItems[0].LatestAuditHint)
	assert.Equal(t, "run-manual-task:token-old:ops-review-old", view.TaskItems[1].TaskID)
	assert.Equal(t, olderPausedAt, view.TaskItems[1].PausedAt)
}

func TestWorkflowPauseViewProjectsPolicyPauseTaskItem(t *testing.T) {
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{
		WorkflowID:      "workflow-editorial",
		WorkflowVersion: "v3",
		EntryNodeID:     "draft",
	})
	require.NoError(t, err)
	run.ID = "run-policy-task"
	run.Status = domain.WorkflowRunPaused
	run.CurrentNodeID = "policy-gate"
	run.ResumeFromCheckpointID = "checkpoint-policy-task"

	pausedAt := time.Date(2026, time.May, 9, 9, 15, 0, 0, time.UTC)
	checkpoint := domain.WorkflowCheckpoint{
		ID:            "checkpoint-policy-task",
		WorkflowRunID: run.ID,
		NodeID:        "policy-gate",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		CreatedAt:     pausedAt,
		Metadata: map[string]any{
			workflowPauseSourceMetadataKey:             string(WorkflowPauseSourcePolicy),
			workflowPauseScopeMetadataKey:              string(WorkflowPauseScopeRun),
			workflowPauseReasonMetadataKey:             "policy threshold exceeded",
			workflowPauseAllowedResumeModesMetadataKey: []string{string(WorkflowResumeModeContinueActiveTokens), string(WorkflowResumeModeReplayFromCheckpoint)},
			workflowPausePayloadMetadataKey: map[string]any{
				"trigger_context": map[string]any{"rule_id": "policy-9", "score": 0.97},
			},
			workflowActiveTokenSetMetadataKey: []any{
				map[string]any{"token_id": "token-policy-1", "node_id": "policy-gate"},
			},
			workflowLatestAuditHintMetadataKey: "policy flagged high-risk output",
		},
	}

	view, err := BuildWorkflowPauseView(run, nil, &checkpoint)

	require.NoError(t, err)
	require.Len(t, view.TaskItems, 1)
	assert.Equal(t, WorkflowPausedTaskItem{
		TaskID:      "run-policy-task:token-policy-1:policy-gate",
		RunID:       "run-policy-task",
		TokenID:     "token-policy-1",
		PauseSource: WorkflowPauseSourcePolicy,
		PausedAt:    pausedAt,
		NodeID:      "policy-gate",
		Title:       "Policy review required",
		Summary:     "policy threshold exceeded",
		AllowedResumeModes: []WorkflowResumeMode{
			WorkflowResumeModeContinueActiveTokens,
			WorkflowResumeModeReplayFromCheckpoint,
		},
		AvailableActions: []string{"resume", "replay"},
		PausePayloadPreview: map[string]any{
			"trigger_context": map[string]any{"rule_id": "policy-9", "score": 0.97},
		},
		LatestAuditHint: "policy flagged high-risk output",
	}, view.TaskItems[0])
}

func TestWorkflowPauseViewExcludesResolvedTaskItems(t *testing.T) {
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{
		WorkflowID:      "workflow-editorial",
		WorkflowVersion: "v3",
		EntryNodeID:     "draft",
	})
	require.NoError(t, err)
	run.ID = "run-resolved-task"
	run.Status = domain.WorkflowRunPaused
	run.CurrentNodeID = "human-review"
	run.ResumeFromCheckpointID = "checkpoint-active-task"

	active := domain.WorkflowCheckpoint{
		ID:            "checkpoint-active-task",
		WorkflowRunID: run.ID,
		NodeID:        "human-review",
		ResumeToken:   "token-active",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		CreatedAt:     time.Date(2026, time.May, 9, 10, 0, 0, 0, time.UTC),
		Metadata: map[string]any{
			workflowPauseSourceMetadataKey:             string(WorkflowPauseSourceHumanNode),
			workflowPauseScopeMetadataKey:              string(WorkflowPauseScopeToken),
			workflowPauseReasonMetadataKey:             "awaiting editor approval",
			workflowPauseAllowedResumeModesMetadataKey: []string{string(WorkflowResumeModeContinueToken)},
			workflowPausePayloadMetadataKey: map[string]any{
				"node_id":  "human-review",
				"token_id": "token-active",
			},
		},
	}
	resolved := domain.WorkflowCheckpoint{
		ID:            "checkpoint-resolved-task",
		WorkflowRunID: run.ID,
		NodeID:        "human-review",
		ResumeToken:   "token-resolved",
		State:         domain.WorkflowCheckpointStateTerminal,
		Resumable:     false,
		CreatedAt:     time.Date(2026, time.May, 9, 9, 0, 0, 0, time.UTC),
		Metadata: map[string]any{
			workflowPauseSourceMetadataKey:             string(WorkflowPauseSourceHumanNode),
			workflowPauseScopeMetadataKey:              string(WorkflowPauseScopeToken),
			workflowPauseReasonMetadataKey:             "resolved approval",
			workflowPauseAllowedResumeModesMetadataKey: []string{string(WorkflowResumeModeContinueToken)},
			workflowPausePayloadMetadataKey: map[string]any{
				"node_id":  "human-review",
				"token_id": "token-resolved",
			},
		},
	}

	view, err := BuildWorkflowPauseView(run, nil, &resolved, &active)

	require.NoError(t, err)
	require.Len(t, view.TaskItems, 1)
	assert.Equal(t, "run-resolved-task:token-active:human-review", view.TaskItems[0].TaskID)
	assert.Equal(t, "token-active", view.TaskItems[0].TokenID)
}

func TestWorkflowPauseViewIncludesOnlyPauseResumeAndTaskAuditRefs(t *testing.T) {
	run, err := domain.NewWorkflowRun(domain.WorkflowRunSpec{
		WorkflowID:      "workflow-editorial",
		WorkflowVersion: "v3",
		EntryNodeID:     "draft",
	})
	require.NoError(t, err)
	run.ID = "run-audit-refs"
	run.Status = domain.WorkflowRunPaused
	run.CurrentNodeID = "human-review"
	run.ResumeFromCheckpointID = "checkpoint-audit-refs"

	checkpoint := domain.WorkflowCheckpoint{
		ID:            "checkpoint-audit-refs",
		WorkflowRunID: run.ID,
		NodeID:        "human-review",
		ResumeToken:   "token-1",
		State:         domain.WorkflowCheckpointStateActive,
		Resumable:     true,
		CreatedAt:     time.Date(2026, time.May, 9, 10, 0, 0, 0, time.UTC),
		Metadata: map[string]any{
			workflowPauseSourceMetadataKey:             string(WorkflowPauseSourceHumanNode),
			workflowPauseScopeMetadataKey:              string(WorkflowPauseScopeToken),
			workflowPauseReasonMetadataKey:             "awaiting editor approval",
			workflowPauseAllowedResumeModesMetadataKey: []string{string(WorkflowResumeModeContinueToken)},
			workflowPausePayloadMetadataKey: map[string]any{
				"node_id":  "human-review",
				"token_id": "token-1",
			},
		},
	}

	auditLogs := []domain.AuditLog{
		{
			ID:         "audit-other-newest",
			Actor:      "tester",
			Action:     "web_control.workflow_run.view",
			Resource:   "workflow_run",
			ResourceID: run.ID,
			CreatedAt:  time.Date(2026, time.May, 9, 10, 4, 0, 0, time.UTC),
		},
		{
			ID:         "audit-task-newest",
			Actor:      "tester",
			Action:     "web_control.workflow_task.submit",
			Resource:   "workflow_task",
			ResourceID: "task-1",
			Metadata:   map[string]any{"workflow_run_id": run.ID},
			CreatedAt:  time.Date(2026, time.May, 9, 10, 3, 0, 0, time.UTC),
		},
		{
			ID:         "audit-resume",
			Actor:      "tester",
			Action:     workflowRunResumeAuditAction,
			Resource:   "workflow_run",
			ResourceID: run.ID,
			CreatedAt:  time.Date(2026, time.May, 9, 10, 2, 0, 0, time.UTC),
		},
		{
			ID:         "audit-pause-oldest",
			Actor:      "tester",
			Action:     workflowRunPauseAuditAction,
			Resource:   "workflow_run",
			ResourceID: run.ID,
			CreatedAt:  time.Date(2026, time.May, 9, 10, 1, 0, 0, time.UTC),
		},
		{
			ID:         "audit-other-run",
			Actor:      "tester",
			Action:     workflowRunPauseAuditAction,
			Resource:   "workflow_run",
			ResourceID: "run-other",
			CreatedAt:  time.Date(2026, time.May, 9, 10, 5, 0, 0, time.UTC),
		},
	}

	view, err := BuildWorkflowPauseView(run, auditLogs, &checkpoint)

	require.NoError(t, err)
	require.Len(t, view.FullAuditRefs, 3)
	assert.Equal(t, []string{"audit-task-newest", "audit-resume", "audit-pause-oldest"}, []string{view.FullAuditRefs[0].ID, view.FullAuditRefs[1].ID, view.FullAuditRefs[2].ID})
	assert.Equal(t, []string{"web_control.workflow_task.submit", workflowRunResumeAuditAction, workflowRunPauseAuditAction}, []string{view.FullAuditRefs[0].Action, view.FullAuditRefs[1].Action, view.FullAuditRefs[2].Action})
	assert.Equal(t, WorkflowPausedRunSummary{
		RunID:              "run-audit-refs",
		Status:             domain.WorkflowRunPaused,
		WorkflowID:         "workflow-editorial",
		WorkflowVersion:    "v3",
		PauseSource:        WorkflowPauseSourceHumanNode,
		PauseReason:        "awaiting editor approval",
		AllowedResumeModes: []WorkflowResumeMode{WorkflowResumeModeContinueToken},
		AffectedTokenCount: 1,
		AffectedTokenIDs:   []string{"token-1"},
		CurrentNodeIDs:     []string{"human-review"},
		CheckpointID:       "checkpoint-audit-refs",
		WorkspaceArticleID: "",
	}, view.Summary)
	require.Len(t, view.TaskItems, 1)
	assert.Equal(t, "run-audit-refs:token-1:human-review", view.TaskItems[0].TaskID)
}
