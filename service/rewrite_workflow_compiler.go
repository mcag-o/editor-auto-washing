package service

import (
	"content-hub/domain"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	rewriteWorkflowNodeTypeGenerate    = "rewrite_generate"
	rewriteWorkflowNodeTypeReview      = "rewrite_review"
	rewriteWorkflowNodeTypeRepair      = "rewrite_repair"
	rewriteWorkflowNodeTypeMaterialize = "rewrite_materialize_draft"

	rewriteWorkflowMaterializeNodeID = "materialize_draft"
)

type RewriteWorkflowCompiler struct{}

type rewriteWorkflowNodeConfig struct {
	Stage                domain.RewriteStageDefinition `json:"stage"`
	DefaultLLMProfile    string                        `json:"default_llm_profile,omitempty"`
	RouteOnQualityAction string                        `json:"route_on_quality_action,omitempty"`
}

type rewriteWorkflowMaterializeNodeConfig struct {
	Policy string `json:"policy,omitempty"`
}

func NewRewriteWorkflowCompiler() *RewriteWorkflowCompiler {
	return &RewriteWorkflowCompiler{}
}

func (c *RewriteWorkflowCompiler) Compile(profile *domain.RewritePipelineProfile) (*domain.WorkflowDefinition, error) {
	if profile == nil {
		return nil, domain.NewValidationErr("rewrite profile is required", nil)
	}
	allStagesByName := make(map[string]domain.RewriteStageDefinition, len(profile.Stages))
	for _, stage := range profile.Stages {
		allStagesByName[strings.TrimSpace(stage.Name)] = stage
	}

	enabledStages := make([]domain.RewriteStageDefinition, 0, len(profile.Stages))
	for _, stage := range profile.Stages {
		if stage.Enabled {
			enabledStages = append(enabledStages, stage)
		}
	}
	if len(enabledStages) == 0 {
		return nil, domain.NewValidationErr("rewrite profile must contain at least one enabled stage", nil)
	}
	for _, stage := range enabledStages {
		stage = applyProfileDefaultsToStage(profile, stage)
		if !rewriteStageDeclaresInvalidRepairPolicy(stage) {
			continue
		}
		return nil, domain.NewValidationErr(fmt.Sprintf("rewrite stage %q declares repair policy without repair stage", strings.TrimSpace(stage.Name)), nil)
	}

	nodes := make([]domain.WorkflowNode, 0, len(enabledStages)+1)
	edges := make([]domain.WorkflowEdge, 0, len(enabledStages)*2)

	stagesByName := make(map[string]domain.RewriteStageDefinition, len(enabledStages))
	for _, stage := range enabledStages {
		stagesByName[strings.TrimSpace(stage.Name)] = stage
	}

	stageTypesByName := make(map[string]string, len(enabledStages))
	for _, stage := range enabledStages {
		stageTypesByName[strings.TrimSpace(stage.Name)] = classifyRewriteWorkflowNodeType(stage)
	}
	for _, stage := range enabledStages {
		stage = applyProfileDefaultsToStage(profile, stage)
		if !hasExplicitRewriteRepairRoute(stage) {
			continue
		}
		repairStageName := strings.TrimSpace(stage.OnFailure.RepairStage)
		targetStage, ok := allStagesByName[repairStageName]
		if !ok {
			return nil, domain.NewValidationErr(fmt.Sprintf("rewrite stage %q references missing repair target %q", strings.TrimSpace(stage.Name), repairStageName), nil)
		}
		if !targetStage.Enabled {
			return nil, domain.NewValidationErr(fmt.Sprintf("rewrite stage %q references disabled repair target %q", strings.TrimSpace(stage.Name), repairStageName), nil)
		}
		if classifyRewriteWorkflowNodeType(targetStage) != rewriteWorkflowNodeTypeRepair {
			return nil, domain.NewValidationErr(fmt.Sprintf("rewrite stage %q references non-repair target %q", strings.TrimSpace(stage.Name), repairStageName), nil)
		}
	}

	repairStageTargets := make(map[string]struct{}, len(enabledStages))
	repairStageSourceReviews := make(map[string][]string, len(enabledStages))
	for i, stage := range enabledStages {
		stage = applyProfileDefaultsToStage(profile, stage)
		if !hasExplicitRewriteRepairRoute(stage) {
			continue
		}
		repairStageName := resolveRewriteRepairStageName(stage, i+1, enabledStages)
		if strings.TrimSpace(repairStageName) == "" {
			continue
		}
		if stageTypesByName[repairStageName] == rewriteWorkflowNodeTypeRepair {
			repairStageTargets[repairStageName] = struct{}{}
			repairStageSourceReviews[repairStageName] = append(repairStageSourceReviews[repairStageName], strings.TrimSpace(stage.Name))
		}
	}

	repairStageRejoinTargets := make(map[string]string, len(enabledStages))
	for i, stage := range enabledStages {
		stage = applyProfileDefaultsToStage(profile, stage)
		if !hasExplicitRewriteRepairRoute(stage) {
			continue
		}
		repairStageName := resolveRewriteRepairStageName(stage, i+1, enabledStages)
		if strings.TrimSpace(repairStageName) == "" {
			continue
		}
		if stageTypesByName[repairStageName] == rewriteWorkflowNodeTypeRepair {
			rejoinStageName := rewriteWorkflowNextNonRepairStageName(i+1, enabledStages)
			if rejoinStageName == strings.TrimSpace(repairStageName) {
				rejoinStageName = rewriteWorkflowNextNonRepairStageName(i+2, enabledStages)
			}
			if rejoinStageName != "" {
				repairNodeID := compileRepairNodeID(repairStageName, strings.TrimSpace(stage.Name), repairStageSourceReviews)
				repairStageRejoinTargets[repairNodeID] = rejoinStageName
			}
		}
	}

	for _, stage := range enabledStages {
		nodeType := classifyRewriteWorkflowNodeType(stage)
		stage = applyProfileDefaultsToStage(profile, stage)
		nodeID := strings.TrimSpace(stage.Name)
		if nodeType == rewriteWorkflowNodeTypeRepair {
			sourceReviews := repairStageSourceReviews[nodeID]
			if len(sourceReviews) > 1 {
				for _, reviewName := range sourceReviews {
					configJSON, err := json.Marshal(rewriteWorkflowNodeConfig{Stage: stage, DefaultLLMProfile: strings.TrimSpace(profile.DefaultLLMProfile)})
					if err != nil {
						return nil, fmt.Errorf("marshal rewrite workflow node config for %s: %w", stage.Name, err)
					}
					nodes = append(nodes, domain.WorkflowNode{
						ID:         compileRepairNodeID(nodeID, reviewName, repairStageSourceReviews),
						Type:       nodeType,
						Name:       stage.Name,
						ConfigJSON: string(configJSON),
					})
				}
				continue
			}
		}
		cfg := rewriteWorkflowNodeConfig{
			Stage:                stage,
			DefaultLLMProfile:    strings.TrimSpace(profile.DefaultLLMProfile),
			RouteOnQualityAction: strings.TrimSpace(stage.OnFailure.Action),
		}
		configJSON, err := json.Marshal(cfg)
		if err != nil {
			return nil, fmt.Errorf("marshal rewrite workflow node config for %s: %w", stage.Name, err)
		}
		nodes = append(nodes, domain.WorkflowNode{
			ID:         nodeID,
			Type:       nodeType,
			Name:       stage.Name,
			ConfigJSON: string(configJSON),
		})
	}

	materializeConfig, err := json.Marshal(rewriteWorkflowMaterializeNodeConfig{Policy: profile.MaterializationPolicy})
	if err != nil {
		return nil, fmt.Errorf("marshal rewrite materialize workflow node config: %w", err)
	}
	nodes = append(nodes, domain.WorkflowNode{
		ID:         rewriteWorkflowMaterializeNodeID,
		Type:       rewriteWorkflowNodeTypeMaterialize,
		Name:       rewriteWorkflowMaterializeNodeID,
		ConfigJSON: string(materializeConfig),
	})

	for i, stage := range enabledStages {
		stage = applyProfileDefaultsToStage(profile, stage)
		nextStageName := rewriteWorkflowNextContinuationStageName(i+1, enabledStages, repairStageTargets)
		stageType := classifyRewriteWorkflowNodeType(stage)
		if hasExplicitRewriteRepairRoute(stage) {
			repairStageName := resolveRewriteRepairStageName(stage, i+1, enabledStages)
			if repairStageName != "" {
				repairStage, ok := stagesByName[repairStageName]
				if ok && stageTypesByName[repairStageName] == rewriteWorkflowNodeTypeRepair {
					repairNodeID := compileRepairNodeID(repairStage.Name, strings.TrimSpace(stage.Name), repairStageSourceReviews)
					edges = append(edges, domain.WorkflowEdge{
						FromNodeID: stage.Name,
						ToNodeID:   repairNodeID,
						Condition:  "payload.quality_decision == " + QualityDecisionRepair,
						Priority:   1,
					})
				}
			}
		}

		if stageType == rewriteWorkflowNodeTypeReview {
			nextStageName = rewriteWorkflowNextPassStageName(i+1, enabledStages, repairStageTargets)
			if nextStageName != "" {
				passCondition := "payload.quality_route_decision == " + QualityDecisionPass
				edges = append(edges, domain.WorkflowEdge{
					FromNodeID: stage.Name,
					ToNodeID:   nextStageName,
					Condition:  passCondition,
					Priority:   2,
				})
			} else {
				passCondition := "payload.quality_route_decision == " + QualityDecisionPass
				edges = append(edges, domain.WorkflowEdge{
					FromNodeID: stage.Name,
					ToNodeID:   rewriteWorkflowMaterializeNodeID,
					Condition:  passCondition,
					Priority:   2,
				})
			}
			continue
		}

		if nextStageName != "" {
			if stageType == rewriteWorkflowNodeTypeRepair {
				sourceReviews := repairStageSourceReviews[strings.TrimSpace(stage.Name)]
				if len(sourceReviews) > 1 {
					for _, reviewName := range sourceReviews {
						repairNodeID := compileRepairNodeID(strings.TrimSpace(stage.Name), reviewName, repairStageSourceReviews)
						rejoinStageName := nextStageName
						if configuredRejoin := strings.TrimSpace(repairStageRejoinTargets[repairNodeID]); configuredRejoin != "" {
							rejoinStageName = configuredRejoin
						}
						edges = append(edges, domain.WorkflowEdge{
							FromNodeID: repairNodeID,
							ToNodeID:   rejoinStageName,
							Condition:  "payload.quality_route_decision == pass",
							Priority:   1,
						})
					}
					continue
				}
				if rejoinStageName := strings.TrimSpace(repairStageRejoinTargets[strings.TrimSpace(stage.Name)]); rejoinStageName != "" {
					nextStageName = rejoinStageName
				}
			}
			condition := ""
			if stageType == rewriteWorkflowNodeTypeRepair || rewriteStageRequiresPassContinuation(stage) {
				condition = "payload.quality_route_decision == pass"
			}
			edges = append(edges, domain.WorkflowEdge{
				FromNodeID: stage.Name,
				ToNodeID:   nextStageName,
				Condition:  condition,
				Priority:   1,
			})
			continue
		}

		condition := ""
		if stageType == rewriteWorkflowNodeTypeRepair || rewriteStageRequiresPassContinuation(stage) {
			condition = "payload.quality_route_decision == pass"
		}
		edges = append(edges, domain.WorkflowEdge{
			FromNodeID: stage.Name,
			ToNodeID:   rewriteWorkflowMaterializeNodeID,
			Condition:  condition,
			Priority:   1,
		})
	}

	workflow := &domain.WorkflowDefinition{
		ID:          profile.ID,
		Name:        compiledRewriteWorkflowName(profile),
		Description: profile.Description,
		Version:     profile.Version,
		Enabled:     profile.Enabled,
		EntryNodeID: enabledStages[0].Name,
		Nodes:       nodes,
		Edges:       edges,
		UpdatedAt:   time.Now().UTC(),
	}
	if err := workflow.Validate(); err != nil {
		return nil, err
	}
	return workflow, nil
}

func compiledRewriteWorkflowName(profile *domain.RewritePipelineProfile) string {
	if profile == nil {
		return ""
	}
	if name := strings.TrimSpace(profile.Name); name != "" {
		return name
	}
	if id := strings.TrimSpace(profile.ID); id != "" {
		return id
	}
	return strings.TrimSpace(profile.TargetType) + "/" + strings.TrimSpace(profile.SourceProfile)
}

func classifyRewriteWorkflowNodeType(stage domain.RewriteStageDefinition) string {
	stageType := strings.ToLower(strings.TrimSpace(stage.Type))
	stageName := strings.ToLower(strings.TrimSpace(stage.Name))
	switch {
	case strings.Contains(stageType, "repair") || strings.Contains(stageName, "repair"):
		return rewriteWorkflowNodeTypeRepair
	case strings.Contains(stageType, "review") || strings.Contains(stageType, "quality") || strings.Contains(stageName, "review") || len(stage.QualityChecks) > 0:
		return rewriteWorkflowNodeTypeReview
	default:
		return rewriteWorkflowNodeTypeGenerate
	}
}

func resolveRewriteRepairStageName(stage domain.RewriteStageDefinition, nextIndex int, enabledStages []domain.RewriteStageDefinition) string {
	repairStageName := strings.TrimSpace(stage.OnFailure.RepairStage)
	if repairStageName != "" {
		return repairStageName
	}
	if classifyRewriteWorkflowNodeType(stage) != rewriteWorkflowNodeTypeReview {
		return ""
	}
	if nextIndex < 0 || nextIndex >= len(enabledStages) {
		return ""
	}
	nextStage := enabledStages[nextIndex]
	if classifyRewriteWorkflowNodeType(nextStage) != rewriteWorkflowNodeTypeRepair {
		return ""
	}
	return strings.TrimSpace(nextStage.Name)
}

func rewriteWorkflowNextContinuationStageName(nextIndex int, enabledStages []domain.RewriteStageDefinition, repairStageTargets map[string]struct{}) string {
	for i := nextIndex; i < len(enabledStages); i++ {
		stageName := strings.TrimSpace(enabledStages[i].Name)
		if _, ok := repairStageTargets[stageName]; ok {
			continue
		}
		return stageName
	}
	return ""
}

func rewriteWorkflowNextPassStageName(nextIndex int, enabledStages []domain.RewriteStageDefinition, repairStageTargets map[string]struct{}) string {
	for i := nextIndex; i < len(enabledStages); i++ {
		stageName := strings.TrimSpace(enabledStages[i].Name)
		if _, ok := repairStageTargets[stageName]; ok {
			continue
		}
		return stageName
	}
	return ""
}

func rewriteWorkflowNextNonRepairStageName(nextIndex int, enabledStages []domain.RewriteStageDefinition) string {
	for i := nextIndex; i < len(enabledStages); i++ {
		if classifyRewriteWorkflowNodeType(enabledStages[i]) == rewriteWorkflowNodeTypeRepair {
			continue
		}
		return strings.TrimSpace(enabledStages[i].Name)
	}
	return ""
}

func rewriteStageUsesRepairPolicy(stage domain.RewriteStageDefinition) bool {
	if classifyRewriteWorkflowNodeType(stage) != rewriteWorkflowNodeTypeReview {
		return false
	}
	return strings.TrimSpace(stage.OnFailure.Action) == QualityDecisionRepair &&
		strings.TrimSpace(stage.OnFailure.RepairStage) != ""
}

func rewriteStageDeclaresInvalidRepairPolicy(stage domain.RewriteStageDefinition) bool {
	return strings.TrimSpace(stage.OnFailure.Action) == QualityDecisionRepair &&
		strings.TrimSpace(stage.OnFailure.RepairStage) == ""
}

func rewriteStageDeclaresExplicitRepairPolicy(stage domain.RewriteStageDefinition) bool {
	return classifyRewriteWorkflowNodeType(stage) == rewriteWorkflowNodeTypeReview &&
		strings.TrimSpace(stage.OnFailure.Action) == QualityDecisionRepair
}

func hasExplicitRewriteRepairRoute(stage domain.RewriteStageDefinition) bool {
	return strings.TrimSpace(stage.OnFailure.Action) == QualityDecisionRepair &&
		strings.TrimSpace(stage.OnFailure.RepairStage) != ""
}

func rewriteStageRequiresPassContinuation(stage domain.RewriteStageDefinition) bool {
	return strings.TrimSpace(stage.OnFailure.Action) != ""
}

func compileRepairNodeID(repairStageName string, reviewName string, repairStageSourceReviews map[string][]string) string {
	if len(repairStageSourceReviews[repairStageName]) <= 1 {
		return repairStageName
	}
	return repairStageName + "__" + reviewName
}
