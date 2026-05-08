package service

import (
	"content-hub/domain"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRewriteWorkflowCompilerBuildsPriorityOrderedWorkflowDefinition(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:          "profile-1",
		Name:        "WeChat Rewrite",
		Description: "Compile rewrite stages into workflow nodes",
		Version:     "v1",
		Enabled:     true,
		Stages: []domain.RewriteStageDefinition{
			{
				Name:      "generate_draft",
				Type:      "generate_draft",
				PromptRef: "generate_draft@v1",
				Enabled:   true,
			},
			{
				Name:          "review_draft",
				Type:          "review",
				PromptRef:     "review_draft@v1",
				QualityChecks: []string{"body_length"},
				Enabled:       true,
			},
			{
				Name:      "repair_draft",
				Type:      "repair",
				PromptRef: "repair_draft@v1",
				Enabled:   true,
			},
		},
	}

	wf, err := compiler.Compile(profile)

	require.NoError(t, err)
	require.Equal(t, profile.ID, wf.ID)
	require.Equal(t, profile.Name, wf.Name)
	require.Equal(t, profile.Description, wf.Description)
	require.Equal(t, profile.Version, wf.Version)
	require.Equal(t, profile.Enabled, wf.Enabled)
	require.Equal(t, "generate_draft", wf.EntryNodeID)
	require.Len(t, wf.Nodes, 4)
	require.Len(t, wf.Edges, 3)
	require.Equal(t, []domain.WorkflowEdge{
		{FromNodeID: "generate_draft", ToNodeID: "review_draft", Priority: 1},
		{FromNodeID: "review_draft", ToNodeID: "repair_draft", Condition: "payload.quality_route_decision == pass", Priority: 2},
		{FromNodeID: "repair_draft", ToNodeID: "materialize_draft", Condition: "payload.quality_route_decision == pass", Priority: 1},
	}, wf.Edges)

	var reviewConfig rewriteWorkflowNodeConfig
	require.NoError(t, json.Unmarshal([]byte(wf.Nodes[1].ConfigJSON), &reviewConfig))
	require.Equal(t, profile.Stages[1].Name, reviewConfig.Stage.Name)
	require.Equal(t, "", reviewConfig.RouteOnQualityAction)

	var materializeConfig rewriteWorkflowMaterializeNodeConfig
	require.NoError(t, json.Unmarshal([]byte(wf.Nodes[3].ConfigJSON), &materializeConfig))
	require.Equal(t, profile.MaterializationPolicy, materializeConfig.Policy)
	if err := wf.Validate(); err != nil {
		t.Fatalf("compiled workflow should validate: %v", err)
	}
}

func TestRewriteWorkflowCompilerRoutesReviewToConfiguredNonAdjacentRepairStage(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-2",
		Name:    "Rewrite With Non Adjacent Repair",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{
				Name:      "generate_draft",
				Type:      "generate_draft",
				PromptRef: "generate_draft@v1",
				Enabled:   true,
			},
			{
				Name:          "review_draft",
				Type:          "review",
				PromptRef:     "review_draft@v1",
				QualityChecks: []string{"body_length"},
				OnFailure: domain.RewriteFailurePolicy{
					Action:      QualityDecisionRepair,
					RepairStage: "repair_draft",
				},
				Enabled: true,
			},
			{
				Name:      "finalize",
				Type:      "finalize",
				PromptRef: "finalize@v1",
				Enabled:   true,
			},
			{
				Name:      "repair_draft",
				Type:      "repair",
				PromptRef: "repair_draft@v1",
				Enabled:   true,
			},
		},
	}

	wf, err := compiler.Compile(profile)

	require.NoError(t, err)
	require.Equal(t, []domain.WorkflowEdge{
		{FromNodeID: "review_draft", ToNodeID: "repair_draft", Condition: "payload.quality_decision == route_to_repair", Priority: 1},
		{FromNodeID: "review_draft", ToNodeID: "finalize", Condition: "payload.quality_route_decision == pass", Priority: 2},
	}, outgoingEdges(wf.Edges, "review_draft"))
	require.Contains(t, wf.Edges, domain.WorkflowEdge{
		FromNodeID: "review_draft",
		ToNodeID:   "repair_draft",
		Condition:  "payload.quality_decision == route_to_repair",
		Priority:   1,
	})
}

func TestRewriteWorkflowCompilerRewritesEveryReviewStageWithRepairRouting(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-3",
		Name:    "Rewrite With Multiple Review Stages",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{
				Name:      "generate_draft",
				Type:      "generate_draft",
				PromptRef: "generate_draft@v1",
				Enabled:   true,
			},
			{
				Name:          "review_structure",
				Type:          "review",
				PromptRef:     "review_structure@v1",
				QualityChecks: []string{"body_length"},
				OnFailure: domain.RewriteFailurePolicy{
					Action:      QualityDecisionRepair,
					RepairStage: "repair_structure",
				},
				Enabled: true,
			},
			{
				Name:      "rewrite_body",
				Type:      "generate_draft",
				PromptRef: "rewrite_body@v1",
				Enabled:   true,
			},
			{
				Name:          "review_tone",
				Type:          "review",
				PromptRef:     "review_tone@v1",
				QualityChecks: []string{"body_length"},
				OnFailure: domain.RewriteFailurePolicy{
					Action:      QualityDecisionRepair,
					RepairStage: "repair_tone",
				},
				Enabled: true,
			},
			{
				Name:      "finalize",
				Type:      "finalize",
				PromptRef: "finalize@v1",
				Enabled:   true,
			},
			{
				Name:      "repair_structure",
				Type:      "repair",
				PromptRef: "repair_structure@v1",
				Enabled:   true,
			},
			{
				Name:      "repair_tone",
				Type:      "repair",
				PromptRef: "repair_tone@v1",
				Enabled:   true,
			},
		},
	}

	wf, err := compiler.Compile(profile)

	require.NoError(t, err)
	require.Equal(t, []domain.WorkflowEdge{
		{FromNodeID: "review_structure", ToNodeID: "repair_structure", Condition: "payload.quality_decision == route_to_repair", Priority: 1},
		{FromNodeID: "review_structure", ToNodeID: "rewrite_body", Condition: "payload.quality_route_decision == pass", Priority: 2},
	}, outgoingEdges(wf.Edges, "review_structure"))
	require.Equal(t, []domain.WorkflowEdge{
		{FromNodeID: "review_tone", ToNodeID: "repair_tone", Condition: "payload.quality_decision == route_to_repair", Priority: 1},
		{FromNodeID: "review_tone", ToNodeID: "finalize", Condition: "payload.quality_route_decision == pass", Priority: 2},
	}, outgoingEdges(wf.Edges, "review_tone"))
}

func TestRewriteWorkflowCompilerUsesExplicitPassConditionForFailRunReviewPolicy(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-4",
		Name:    "Rewrite With Fail Run Review",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{
				Name:      "generate_draft",
				Type:      "generate_draft",
				PromptRef: "generate_draft@v1",
				Enabled:   true,
			},
			{
				Name:          "review_draft",
				Type:          "review",
				PromptRef:     "review_draft@v1",
				QualityChecks: []string{"body_length"},
				OnFailure: domain.RewriteFailurePolicy{
					Action: QualityDecisionFail,
				},
				Enabled: true,
			},
			{
				Name:      "finalize",
				Type:      "finalize",
				PromptRef: "finalize@v1",
				Enabled:   true,
			},
		},
	}

	wf, err := compiler.Compile(profile)

	require.NoError(t, err)
	require.Equal(t, []domain.WorkflowEdge{
		{FromNodeID: "review_draft", ToNodeID: "finalize", Condition: "payload.quality_route_decision == pass", Priority: 2},
	}, outgoingEdges(wf.Edges, "review_draft"))
}

func TestRewriteWorkflowCompilerKeepsRepairStagesOutOfNormalPassPath(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-5",
		Name:    "Rewrite With Isolated Repair Stage",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{
				Name:      "generate_draft",
				Type:      "generate_draft",
				PromptRef: "generate_draft@v1",
				Enabled:   true,
			},
			{
				Name:          "review_draft",
				Type:          "review",
				PromptRef:     "review_draft@v1",
				QualityChecks: []string{"body_length"},
				OnFailure: domain.RewriteFailurePolicy{
					Action:      QualityDecisionRepair,
					RepairStage: "repair_draft",
				},
				Enabled: true,
			},
			{
				Name:      "finalize",
				Type:      "finalize",
				PromptRef: "finalize@v1",
				Enabled:   true,
			},
			{
				Name:      "repair_draft",
				Type:      "repair",
				PromptRef: "repair_draft@v1",
				Enabled:   true,
			},
			{
				Name:      "publish_ready",
				Type:      "finalize",
				PromptRef: "publish_ready@v1",
				Enabled:   true,
			},
		},
	}

	wf, err := compiler.Compile(profile)

	require.NoError(t, err)
	require.Equal(t, []domain.WorkflowEdge{
		{FromNodeID: "review_draft", ToNodeID: "repair_draft", Condition: "payload.quality_decision == route_to_repair", Priority: 1},
		{FromNodeID: "review_draft", ToNodeID: "finalize", Condition: "payload.quality_route_decision == pass", Priority: 2},
	}, outgoingEdges(wf.Edges, "review_draft"))
	require.Equal(t, []domain.WorkflowEdge{
		{FromNodeID: "finalize", ToNodeID: "publish_ready", Priority: 1},
	}, outgoingEdges(wf.Edges, "finalize"))
	require.Equal(t, []domain.WorkflowEdge{
		{FromNodeID: "repair_draft", ToNodeID: "finalize", Condition: "payload.quality_route_decision == pass", Priority: 1},
	}, outgoingEdges(wf.Edges, "repair_draft"))
	require.Equal(t, []domain.WorkflowEdge{
		{FromNodeID: "publish_ready", ToNodeID: "materialize_draft", Priority: 1},
	}, outgoingEdges(wf.Edges, "publish_ready"))
}

func TestRewriteWorkflowCompilerCreatesReviewSpecificRepairNodesForSharedRepairStage(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-6",
		Name:    "Rewrite With Shared Repair Stage",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{Name: "generate_draft", Type: "generate_draft", PromptRef: "generate_draft@v1", Enabled: true},
			{
				Name:          "review_alpha",
				Type:          "review",
				PromptRef:     "review_alpha@v1",
				QualityChecks: []string{"body_length"},
				OnFailure:     domain.RewriteFailurePolicy{Action: QualityDecisionRepair, RepairStage: "repair_shared"},
				Enabled:       true,
			},
			{Name: "finalize_alpha", Type: "finalize", PromptRef: "finalize_alpha@v1", Enabled: true},
			{
				Name:          "review_beta",
				Type:          "review",
				PromptRef:     "review_beta@v1",
				QualityChecks: []string{"body_length"},
				OnFailure:     domain.RewriteFailurePolicy{Action: QualityDecisionRepair, RepairStage: "repair_shared"},
				Enabled:       true,
			},
			{Name: "finalize_beta", Type: "finalize", PromptRef: "finalize_beta@v1", Enabled: true},
			{Name: "repair_shared", Type: "repair", PromptRef: "repair_shared@v1", Enabled: true},
			{Name: "publish_ready", Type: "finalize", PromptRef: "publish_ready@v1", Enabled: true},
		},
	}

	wf, err := compiler.Compile(profile)

	require.NoError(t, err)
	require.Equal(t, []domain.WorkflowEdge{
		{FromNodeID: "review_alpha", ToNodeID: "repair_shared__review_alpha", Condition: "payload.quality_decision == route_to_repair", Priority: 1},
		{FromNodeID: "review_alpha", ToNodeID: "finalize_alpha", Condition: "payload.quality_route_decision == pass", Priority: 2},
	}, outgoingEdges(wf.Edges, "review_alpha"))
	require.Equal(t, []domain.WorkflowEdge{
		{FromNodeID: "review_beta", ToNodeID: "repair_shared__review_beta", Condition: "payload.quality_decision == route_to_repair", Priority: 1},
		{FromNodeID: "review_beta", ToNodeID: "finalize_beta", Condition: "payload.quality_route_decision == pass", Priority: 2},
	}, outgoingEdges(wf.Edges, "review_beta"))
	require.Equal(t, []domain.WorkflowEdge{{FromNodeID: "repair_shared__review_alpha", ToNodeID: "finalize_alpha", Condition: "payload.quality_route_decision == pass", Priority: 1}}, outgoingEdges(wf.Edges, "repair_shared__review_alpha"))
	require.Equal(t, []domain.WorkflowEdge{{FromNodeID: "repair_shared__review_beta", ToNodeID: "finalize_beta", Condition: "payload.quality_route_decision == pass", Priority: 1}}, outgoingEdges(wf.Edges, "repair_shared__review_beta"))
}

func TestRewriteWorkflowCompilerUsesPassOnlyContinuationForRepairNodes(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-7",
		Name:    "Rewrite Repair Pass Only",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{Name: "generate_draft", Type: "generate_draft", PromptRef: "generate_draft@v1", Enabled: true},
			{
				Name:          "review_draft",
				Type:          "review",
				PromptRef:     "review_draft@v1",
				QualityChecks: []string{"body_length"},
				OnFailure:     domain.RewriteFailurePolicy{Action: QualityDecisionRepair, RepairStage: "repair_draft"},
				Enabled:       true,
			},
			{Name: "finalize", Type: "finalize", PromptRef: "finalize@v1", Enabled: true},
			{Name: "repair_draft", Type: "repair", PromptRef: "repair_draft@v1", Enabled: true},
		},
	}

	wf, err := compiler.Compile(profile)

	require.NoError(t, err)
	require.Equal(t, []domain.WorkflowEdge{{FromNodeID: "repair_draft", ToNodeID: "materialize_draft", Condition: "payload.quality_route_decision == pass", Priority: 1}}, outgoingEdges(wf.Edges, "repair_draft"))
}

func TestRewriteWorkflowCompilerDoesNotMarkNamedRepairStageAsRepairOnlyWhenPolicyIsFailRun(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-8",
		Name:    "Rewrite Fail Run Does Not Reserve Repair Target",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{Name: "generate_draft", Type: "generate_draft", PromptRef: "generate_draft@v1", Enabled: true},
			{
				Name:          "review_draft",
				Type:          "review",
				PromptRef:     "review_draft@v1",
				QualityChecks: []string{"body_length"},
				OnFailure:     domain.RewriteFailurePolicy{Action: QualityDecisionFail, RepairStage: "repair_candidate"},
				Enabled:       true,
			},
			{Name: "repair_candidate", Type: "repair", PromptRef: "repair_candidate@v1", Enabled: true},
			{Name: "publish_ready", Type: "finalize", PromptRef: "publish_ready@v1", Enabled: true},
		},
	}

	wf, err := compiler.Compile(profile)

	require.NoError(t, err)
	require.Equal(t, []domain.WorkflowEdge{{FromNodeID: "review_draft", ToNodeID: "repair_candidate", Condition: "payload.quality_route_decision == pass", Priority: 2}}, outgoingEdges(wf.Edges, "review_draft"))
	require.Equal(t, []domain.WorkflowEdge{{FromNodeID: "repair_candidate", ToNodeID: "publish_ready", Condition: "payload.quality_route_decision == pass", Priority: 1}}, outgoingEdges(wf.Edges, "repair_candidate"))
}

func TestRewriteWorkflowCompilerKeepsExplicitRepairTargetOutOfPassPathForNonReviewStage(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-non-review-repair-isolation",
		Name:    "Rewrite Non Review Repair Isolation",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{
				Name:      "generate_draft",
				Type:      "generate_draft",
				PromptRef: "generate_draft@v1",
				Enabled:   true,
				OnFailure: domain.RewriteFailurePolicy{Action: QualityDecisionRepair, RepairStage: "repair_draft"},
			},
			{
				Name:      "publish_ready",
				Type:      "finalize",
				PromptRef: "publish_ready@v1",
				Enabled:   true,
			},
			{
				Name:      "repair_draft",
				Type:      "repair",
				PromptRef: "repair_draft@v1",
				Enabled:   true,
			},
		},
	}

	wf, err := compiler.Compile(profile)

	require.NoError(t, err)
	require.Equal(t, []domain.WorkflowEdge{
		{FromNodeID: "generate_draft", ToNodeID: "repair_draft", Condition: "payload.quality_decision == route_to_repair", Priority: 1},
		{FromNodeID: "generate_draft", ToNodeID: "publish_ready", Condition: "payload.quality_route_decision == pass", Priority: 1},
	}, outgoingEdges(wf.Edges, "generate_draft"))
	require.Equal(t, []domain.WorkflowEdge{{FromNodeID: "publish_ready", ToNodeID: "materialize_draft", Priority: 1}}, outgoingEdges(wf.Edges, "publish_ready"))
	require.Equal(t, []domain.WorkflowEdge{{FromNodeID: "repair_draft", ToNodeID: "materialize_draft", Condition: "payload.quality_route_decision == pass", Priority: 1}}, outgoingEdges(wf.Edges, "repair_draft"))
}

func TestRewriteWorkflowCompilerRejectsRepairPolicyWithoutRepairStage(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-9",
		Name:    "Rewrite Invalid Repair Policy",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{Name: "generate_draft", Type: "generate_draft", PromptRef: "generate_draft@v1", Enabled: true},
			{
				Name:          "review_draft",
				Type:          "review",
				PromptRef:     "review_draft@v1",
				QualityChecks: []string{"body_length"},
				OnFailure:     domain.RewriteFailurePolicy{Action: QualityDecisionRepair},
				Enabled:       true,
			},
			{Name: "finalize", Type: "finalize", PromptRef: "finalize@v1", Enabled: true},
		},
	}

	wf, err := compiler.Compile(profile)

	require.Nil(t, wf)
	require.Error(t, err)
	require.ErrorContains(t, err, "repair stage")
}

func TestRewriteWorkflowCompilerRejectsRepairPolicyWithMissingNamedRepairStage(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-10",
		Name:    "Rewrite Missing Repair Target",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{Name: "generate_draft", Type: "generate_draft", PromptRef: "generate_draft@v1", Enabled: true},
			{
				Name:          "review_draft",
				Type:          "review",
				PromptRef:     "review_draft@v1",
				QualityChecks: []string{"body_length"},
				OnFailure:     domain.RewriteFailurePolicy{Action: QualityDecisionRepair, RepairStage: "repair_missing"},
				Enabled:       true,
			},
			{Name: "finalize", Type: "finalize", PromptRef: "finalize@v1", Enabled: true},
		},
	}

	wf, err := compiler.Compile(profile)

	require.Nil(t, wf)
	require.Error(t, err)
	require.ErrorContains(t, err, "repair target")
	require.ErrorContains(t, err, "repair_missing")
}

func TestRewriteWorkflowCompilerRejectsRepairPolicyWithNonRepairNamedTarget(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-11",
		Name:    "Rewrite Non Repair Target",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{Name: "generate_draft", Type: "generate_draft", PromptRef: "generate_draft@v1", Enabled: true},
			{
				Name:          "review_draft",
				Type:          "review",
				PromptRef:     "review_draft@v1",
				QualityChecks: []string{"body_length"},
				OnFailure:     domain.RewriteFailurePolicy{Action: QualityDecisionRepair, RepairStage: "finalize"},
				Enabled:       true,
			},
			{Name: "finalize", Type: "finalize", PromptRef: "finalize@v1", Enabled: true},
		},
	}

	wf, err := compiler.Compile(profile)

	require.Nil(t, wf)
	require.Error(t, err)
	require.ErrorContains(t, err, "repair target")
	require.ErrorContains(t, err, "finalize")
}

func TestRewriteWorkflowCompilerRejectsRepairPolicyWithDisabledNamedRepairStage(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-12",
		Name:    "Rewrite Disabled Repair Target",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{Name: "generate_draft", Type: "generate_draft", PromptRef: "generate_draft@v1", Enabled: true},
			{
				Name:          "review_draft",
				Type:          "review",
				PromptRef:     "review_draft@v1",
				QualityChecks: []string{"body_length"},
				OnFailure:     domain.RewriteFailurePolicy{Action: QualityDecisionRepair, RepairStage: "repair_draft"},
				Enabled:       true,
			},
			{Name: "repair_draft", Type: "repair", PromptRef: "repair_draft@v1", Enabled: false},
			{Name: "finalize", Type: "finalize", PromptRef: "finalize@v1", Enabled: true},
		},
	}

	wf, err := compiler.Compile(profile)

	require.Nil(t, wf)
	require.Error(t, err)
	require.ErrorContains(t, err, "disabled")
	require.ErrorContains(t, err, "repair_draft")
}

func TestRewriteWorkflowCompilerRejectsNonReviewRepairPolicyWithMissingNamedRepairStage(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-13",
		Name:    "Rewrite Missing Non Review Repair Target",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{
				Name:      "generate_draft",
				Type:      "generate_draft",
				PromptRef: "generate_draft@v1",
				OnFailure: domain.RewriteFailurePolicy{Action: QualityDecisionRepair, RepairStage: "repair_missing"},
				Enabled:   true,
			},
			{Name: "finalize", Type: "finalize", PromptRef: "finalize@v1", Enabled: true},
		},
	}

	wf, err := compiler.Compile(profile)

	require.Nil(t, wf)
	require.Error(t, err)
	require.ErrorContains(t, err, "repair target")
	require.ErrorContains(t, err, "repair_missing")
}

func TestRewriteWorkflowCompilerRejectsNonReviewRepairPolicyWithoutRepairStage(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-13b",
		Name:    "Rewrite Missing Non Review Repair Stage",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{
				Name:      "generate_draft",
				Type:      "generate_draft",
				PromptRef: "generate_draft@v1",
				OnFailure: domain.RewriteFailurePolicy{Action: QualityDecisionRepair},
				Enabled:   true,
			},
			{Name: "finalize", Type: "finalize", PromptRef: "finalize@v1", Enabled: true},
		},
	}

	wf, err := compiler.Compile(profile)

	require.Nil(t, wf)
	require.Error(t, err)
	require.ErrorContains(t, err, "repair policy without repair stage")
	require.ErrorContains(t, err, "generate_draft")
}

func TestRewriteWorkflowCompilerRejectsNonReviewRepairPolicyWithNonRepairNamedTarget(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-14",
		Name:    "Rewrite Non Review Non Repair Target",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{
				Name:      "generate_draft",
				Type:      "generate_draft",
				PromptRef: "generate_draft@v1",
				OnFailure: domain.RewriteFailurePolicy{Action: QualityDecisionRepair, RepairStage: "finalize"},
				Enabled:   true,
			},
			{Name: "finalize", Type: "finalize", PromptRef: "finalize@v1", Enabled: true},
		},
	}

	wf, err := compiler.Compile(profile)

	require.Nil(t, wf)
	require.Error(t, err)
	require.ErrorContains(t, err, "repair target")
	require.ErrorContains(t, err, "finalize")
}

func TestRewriteWorkflowCompilerRejectsNonReviewRepairPolicyWithDisabledNamedRepairStage(t *testing.T) {
	compiler := NewRewriteWorkflowCompiler()
	profile := &domain.RewritePipelineProfile{
		ID:      "profile-15",
		Name:    "Rewrite Non Review Disabled Repair Target",
		Version: "v1",
		Enabled: true,
		Stages: []domain.RewriteStageDefinition{
			{
				Name:      "generate_draft",
				Type:      "generate_draft",
				PromptRef: "generate_draft@v1",
				OnFailure: domain.RewriteFailurePolicy{Action: QualityDecisionRepair, RepairStage: "repair_disabled"},
				Enabled:   true,
			},
			{Name: "repair_disabled", Type: "repair", PromptRef: "repair_disabled@v1", Enabled: false},
			{Name: "finalize", Type: "finalize", PromptRef: "finalize@v1", Enabled: true},
		},
	}

	wf, err := compiler.Compile(profile)

	require.Nil(t, wf)
	require.Error(t, err)
	require.ErrorContains(t, err, "disabled")
	require.ErrorContains(t, err, "repair_disabled")
}
