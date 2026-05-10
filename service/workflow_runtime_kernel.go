package service

import (
	"content-hub/domain"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type workflowRuntimeKernel struct {
	nodes  map[string]WorkflowNode
	router workflowRouter
	pool   *workflowWorkerPool
}

func newWorkflowRuntimeKernel(nodes map[string]WorkflowNode) workflowRuntimeKernel {
	return workflowRuntimeKernel{nodes: nodes, router: newWorkflowRouter(), pool: newWorkflowWorkerPool(workflowRuntimeWorkerPoolSize)}
}

func (k workflowRuntimeKernel) Execute(ctx context.Context, wf *domain.WorkflowDefinition, wc *domain.WorkflowContext) error {
	runtimeCtx := &WorkflowExecutionContext{
		Workflow:      wf,
		Context:       wc,
		CurrentNodeID: wf.EntryNodeID,
	}
	return k.executeFrom(ctx, runtimeCtx, wf.EntryNodeID)
}

func (k workflowRuntimeKernel) Resume(ctx context.Context, runtimeCtx *WorkflowExecutionContext) error {
	checkpoint, err := latestResumableCheckpoint(runtimeCtx.Checkpoints)
	if err != nil {
		return err
	}
	mode := defaultWorkflowResumeMode(checkpoint)
	recordWorkflowResumeCommand(checkpoint, mode, WorkflowResumeCommand{Mode: mode})
	applyWorkflowResumeTarget(runtimeCtx, checkpoint, mode)
	if input := workflowHumanResumeInputFromCheckpoint(checkpoint); len(input) > 0 {
		applyWorkflowHumanResumeInput(runtimeCtx, input)
		runtimeCtx.CurrentToken.State = WorkflowTokenStateActive
	}
	runtimeCtx.CurrentNodeID = checkpoint.NodeID
	return k.executeFrom(ctx, runtimeCtx, checkpoint.NodeID)
}

func (k workflowRuntimeKernel) ResumeWithCommand(ctx context.Context, runtimeCtx *WorkflowExecutionContext, command WorkflowResumeCommand) error {
	checkpoint, err := resumableCheckpointByID(runtimeCtx.Checkpoints, command.CheckpointID)
	if err != nil {
		return err
	}
	mode, err := resolveWorkflowResumeMode(checkpoint, command.Mode)
	if err != nil {
		return err
	}
	recordWorkflowResumeCommand(checkpoint, mode, command)
	applyWorkflowResumeTarget(runtimeCtx, checkpoint, mode)
	if input := workflowHumanResumeInputFromCheckpoint(checkpoint); len(input) > 0 {
		applyWorkflowHumanResumeInput(runtimeCtx, input)
		runtimeCtx.CurrentToken.State = WorkflowTokenStateActive
	}
	runtimeCtx.CurrentNodeID = checkpoint.NodeID
	return k.executeFrom(ctx, runtimeCtx, checkpoint.NodeID)
}

func (k workflowRuntimeKernel) executeFrom(ctx context.Context, runtimeCtx *WorkflowExecutionContext, startNodeID string) error {
	if runtimeCtx == nil || runtimeCtx.Workflow == nil {
		return fmt.Errorf("workflow definition is required")
	}
	if err := validateWorkflowRuntimeGraph(runtimeCtx.Workflow); err != nil {
		if !workflowLoopValidationAllowed(runtimeCtx.Workflow, err) {
			return err
		}
	}
	if runtimeCtx.JoinBarriers == nil {
		runtimeCtx.JoinBarriers = map[string]*workflowJoinBarrier{}
	}
	consumeActiveCheckpoints(runtimeCtx, time.Now().UTC())

	nodeIndex := indexWorkflowNodes(runtimeCtx.Workflow)
	initializeWorkflowTokens(runtimeCtx, startNodeID)
	for len(runtimeCtx.ActiveTokens) > 0 {
		batch := snapshotActiveWorkflowTokens(runtimeCtx)
		if len(batch) == 0 {
			break
		}
		outcomes, err := k.executeActiveTokenBatch(ctx, runtimeCtx, nodeIndex, batch)
		if err != nil {
			return err
		}
		for _, outcome := range outcomes {
			if outcome == nil || outcome.token == nil || strings.TrimSpace(outcome.current) == "" {
				continue
			}
			if err := k.applyTokenExecutionOutcome(runtimeCtx, nodeIndex, outcome); err != nil {
				return err
			}
			if outcome.result.Paused {
				clearWorkflowTokenExecutionFrame(runtimeCtx)
				return nil
			}
		}
	}
	consumeActiveCheckpoints(runtimeCtx, time.Now().UTC())
	clearWorkflowTokenExecutionFrame(runtimeCtx)
	return nil
}

type workflowTokenExecutionOutcome struct {
	token       *WorkflowToken
	current     string
	result      WorkflowNodeResult
	nextEdges   []domain.WorkflowEdge
	context     *domain.WorkflowContext
	latestRoute *WorkflowRouteOutcomeSummary
	err         error
}

type workflowTokenPauseSignal struct{}

func (workflowTokenPauseSignal) Error() string {
	return "workflow token paused"
}

func snapshotActiveWorkflowTokens(runtimeCtx *WorkflowExecutionContext) []*WorkflowToken {
	if runtimeCtx == nil || len(runtimeCtx.ActiveTokens) == 0 {
		return nil
	}
	tokens := make([]*WorkflowToken, 0, len(runtimeCtx.ActiveTokens))
	for len(runtimeCtx.ActiveTokens) > 0 {
		token := popNextActiveWorkflowToken(runtimeCtx)
		if token == nil {
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens
}

func (k workflowRuntimeKernel) executeActiveTokenBatch(ctx context.Context, runtimeCtx *WorkflowExecutionContext, nodeIndex map[string]domain.WorkflowNode, tokens []*WorkflowToken) ([]*workflowTokenExecutionOutcome, error) {
	outcomes := make([]*workflowTokenExecutionOutcome, len(tokens))
	var mu sync.Mutex
	err := k.pool.Run(ctx, tokens, func(runCtx context.Context, token *WorkflowToken) error {
		outcome := k.executeSingleToken(runCtx, runtimeCtx, nodeIndex, token)
		mu.Lock()
		for i := range tokens {
			if tokens[i] == token {
				outcomes[i] = outcome
				break
			}
		}
		mu.Unlock()
		if outcome != nil && outcome.err != nil {
			return outcome.err
		}
		if outcome != nil && outcome.result.Paused {
			return workflowTokenPauseSignal{}
		}
		return nil
	})
	if err != nil {
		if _, ok := err.(workflowTokenPauseSignal); ok {
			return outcomes, nil
		}
		return nil, err
	}
	return outcomes, nil
}

func (k workflowRuntimeKernel) executeSingleToken(ctx context.Context, runtimeCtx *WorkflowExecutionContext, nodeIndex map[string]domain.WorkflowNode, token *WorkflowToken) *workflowTokenExecutionOutcome {
	if token == nil || strings.TrimSpace(token.NodeID) == "" {
		return &workflowTokenExecutionOutcome{}
	}
	current := token.NodeID
	localCtx := newWorkflowTokenRuntimeContext(runtimeCtx, token)
	bindWorkflowTokenExecutionFrame(localCtx, token)
	localCtx.CurrentNodeID = current
	node, ok := k.nodes[current]
	if !ok {
		return &workflowTokenExecutionOutcome{err: fmt.Errorf("workflow node not found: %s", current)}
	}
	result := WorkflowNodeResult{}
	nextEdges := outgoingEdges(localCtx.Workflow.Edges, current)
	graphRouteRequired := routeRequired(nextEdges)
	if runtimeNode, ok := node.(workflowRuntimeNode); ok {
		nodeDef, found := nodeIndex[current]
		if !found {
			return &workflowTokenExecutionOutcome{err: fmt.Errorf("workflow node not found in definition: %s", current)}
		}
		execResult, err := runtimeNode.ExecuteWorkflow(ctx, localCtx, nodeDef)
		if err != nil {
			return &workflowTokenExecutionOutcome{err: err}
		}
		result = execResult
		result.RouteRequired = graphRouteRequired && !result.AllowNaturalTermination
	} else {
		if err := node.Execute(ctx, localCtx.Context); err != nil {
			return &workflowTokenExecutionOutcome{err: err}
		}
		syncWorkflowTokenPayloadBridge(localCtx)
		result.RouteRequired = graphRouteRequired
	}
	storeWorkflowNodeResult(token, result)
	if result.Paused {
		pauseWorkflowToken(token, result.PauseState)
	}
	rebindWorkflowNodeResult(localCtx, token)
	return &workflowTokenExecutionOutcome{
		token:       token,
		current:     current,
		result:      result,
		nextEdges:   nextEdges,
		context:     localCtx.Context,
		latestRoute: localCtx.LatestRoute,
	}
}

func newWorkflowTokenRuntimeContext(runtimeCtx *WorkflowExecutionContext, token *WorkflowToken) *WorkflowExecutionContext {
	if runtimeCtx == nil {
		return &WorkflowExecutionContext{CurrentToken: token}
	}
	local := &WorkflowExecutionContext{
		Workflow:       runtimeCtx.Workflow,
		Context:        cloneWorkflowExecutionDomainContext(runtimeCtx.Context),
		Input:          cloneWorkflowPayload(workflowRuntimeSharedInput(runtimeCtx)),
		sharedInput:    cloneWorkflowPayload(workflowRuntimeSharedInput(runtimeCtx)),
		Metadata:       cloneWorkflowPayload(workflowRuntimeSharedMetadata(runtimeCtx)),
		sharedMetadata: cloneWorkflowPayload(workflowRuntimeSharedMetadata(runtimeCtx)),
		CurrentToken:   token,
		RootToken:      runtimeCtx.RootToken,
	}
	bindWorkflowPayloadBridge(local)
	return local
}

func cloneWorkflowExecutionDomainContext(ctx *domain.WorkflowContext) *domain.WorkflowContext {
	if ctx == nil {
		return &domain.WorkflowContext{}
	}
	return &domain.WorkflowContext{
		Payload:      cloneWorkflowPayload(workflowRuntimeInput(ctx)),
		Document:     ctx.Document,
		ArtifactPath: ctx.ArtifactPath,
		TraceID:      ctx.TraceID,
		Command:      ctx.Command,
	}
}

func syncWorkflowTokenPayloadBridge(runtimeCtx *WorkflowExecutionContext) {
	if runtimeCtx == nil || runtimeCtx.Context == nil {
		return
	}
	sharedInput := workflowRuntimeSharedInput(runtimeCtx)
	variables := map[string]any{}
	for key, value := range runtimeCtx.Context.Payload {
		if sharedValue, shared := sharedInput[key]; shared && fmt.Sprint(sharedValue) == fmt.Sprint(value) {
			continue
		}
		variables[key] = cloneWorkflowValue(value)
	}
	runtimeCtx.Variables = variables
	if runtimeCtx.CurrentToken != nil && runtimeCtx.CurrentToken.Branch != nil {
		runtimeCtx.CurrentToken.Branch.Variables = cloneWorkflowPayload(variables)
	}
	bindWorkflowPayloadBridge(runtimeCtx)
}

func (k workflowRuntimeKernel) applyTokenExecutionOutcome(runtimeCtx *WorkflowExecutionContext, nodeIndex map[string]domain.WorkflowNode, outcome *workflowTokenExecutionOutcome) error {
	token := outcome.token
	current := outcome.current
	result := outcome.result
	runtimeCtx.Context = outcome.context
	runtimeCtx.LatestRoute = outcome.latestRoute
	bindWorkflowTokenExecutionFrame(runtimeCtx, token)
	runtimeCtx.CurrentNodeID = current
	if result.Paused {
		appendCheckpointWithSnapshot(runtimeCtx, "", current, workflowCheckpointSnapshot{Token: token, PauseState: token.PauseState, Metadata: workflowHumanResumeInputMetadata(nil, false)})
		return nil
	}
	if !workflowNodeIsLoop(nodeIndex[current]) {
		if barrier, ok := runtimeCtx.JoinBarriers[current]; ok && barrier != nil {
		barrier.tokens[token.ID] = token
		barrier.Arrive(token.ID)
		if !barrier.Ready() {
			return nil
		}
		joined := workflowJoinToken(current, barrier)
		if joined == nil {
			return nil
		}
		delete(runtimeCtx.JoinBarriers, current)
		runtimeCtx.CurrentToken = joined
		runtimeCtx.CurrentNodeID = current
		runtimeCtx.Input = cloneWorkflowPayload(workflowRuntimeSharedInput(runtimeCtx))
		runtimeCtx.Metadata = cloneWorkflowPayload(workflowRuntimeSharedMetadata(runtimeCtx))
		runtimeCtx.CurrentFrame = joined.Frame
		if joined.Branch != nil {
			runtimeCtx.Variables = joined.Branch.Variables
			runtimeCtx.Result = joined.Branch.Result
			runtimeCtx.Artifacts = joined.Branch.Artifacts
		}
		runtimeCtx.CompletedTokens = append(runtimeCtx.CompletedTokens, joined)
		storeWorkflowNodeResult(joined, result)
		bindWorkflowTokenExecutionFrame(runtimeCtx, joined)
		}
	}

	evaluation := k.router.EvaluateRoutes(runtimeCtx, current, result, outcome.nextEdges)
	recordLatestRouteOutcome(runtimeCtx, workflowRouteOutcomeSummary(evaluation))
	switch evaluation.Outcome {
		case WorkflowRouteOutcomeSingleMatch, WorkflowRouteOutcomeMultiMatch, WorkflowRouteOutcomeFallbackMatch:
		type checkpointChild struct {
			token   *WorkflowToken
			summary WorkflowRouteOutcomeSummary
		}
		children := make([]checkpointChild, 0, len(evaluation.SelectedEdges))
			for i := len(evaluation.SelectedEdges) - 1; i >= 0; i-- {
				nextEdge := evaluation.SelectedEdges[i]
				if _, ok := nodeIndex[nextEdge.ToNodeID]; !ok {
					return fmt.Errorf("workflow entry path references unknown node: %s", nextEdge.ToNodeID)
				}
				routeSummary := workflowRouteOutcomeSummaryForEdge(evaluation, nextEdge)
				child := token.Child(nextEdge.ToNodeID, WorkflowTokenRouteLineage{
					SourceNodeID:   current,
					SelectedEdgeID: workflowEdgeSummaryID(nextEdge),
					SelectedNodeID: nextEdge.ToNodeID,
				})
				if token.Subflow != nil && strings.TrimSpace(token.Subflow.ReturnNodeID) == strings.TrimSpace(nextEdge.ToNodeID) {
					parentBranch := cloneWorkflowBranchContext(token.Subflow.ParentBranch)
					if parentBranch == nil {
						parentBranch = newWorkflowBranchContext(nil, nil)
					}
					parent := &WorkflowToken{Branch: parentBranch}
					applyWorkflowSubflowReturnMapping(parent, token, token.Subflow.ReturnMapping)
					child.Branch = parent.Branch
					child.Subflow = nil
				}
				if incoming := incomingEdges(runtimeCtx.Workflow.Edges, nextEdge.ToNodeID); len(incoming) > 1 && !workflowNodeIsLoop(nodeIndex[nextEdge.ToNodeID]) {
					if runtimeCtx.JoinBarriers[nextEdge.ToNodeID] == nil {
						runtimeCtx.JoinBarriers[nextEdge.ToNodeID] = newWorkflowJoinBarrierWithExpectedCount(nextEdge.ToNodeID, len(incoming))
					}
					barrier := runtimeCtx.JoinBarriers[nextEdge.ToNodeID]
					barrier.tokens[child.ID] = child
					barrier.Arrive(child.ID)
					if !barrier.Ready() {
						continue
					}
					joined := workflowJoinToken(nextEdge.ToNodeID, barrier)
					delete(runtimeCtx.JoinBarriers, nextEdge.ToNodeID)
					if joined != nil {
						pushActiveWorkflowToken(runtimeCtx, joined)
						appendCheckpointWithSnapshot(runtimeCtx, "", joined.NodeID, workflowCheckpointSnapshot{Token: joined})
					}
					continue
				}
				children = append(children, checkpointChild{token: child, summary: routeSummary})
				pushActiveWorkflowToken(runtimeCtx, child)
			}
		for _, child := range children {
			routeSummary := child.summary
			appendCheckpointWithSnapshot(runtimeCtx, "", child.token.NodeID, workflowCheckpointSnapshot{RouteSummary: &routeSummary, Token: child.token})
		}
	case WorkflowRouteOutcomeNoMatch:
		if result.RouteRequired {
			return fmt.Errorf("no matching route for node %s", current)
		}
	case WorkflowRouteOutcomeAmbiguousMatch, WorkflowRouteOutcomeEvaluationErr:
		return evaluation.Error
	}
	runtimeCtx.CompletedTokens = append(runtimeCtx.CompletedTokens, token)
	return nil
}

func initializeWorkflowTokens(runtimeCtx *WorkflowExecutionContext, startNodeID string) {
	if runtimeCtx == nil || len(runtimeCtx.ActiveTokens) > 0 {
		return
	}
	if runtimeCtx.CurrentToken != nil {
		runtimeCtx.CurrentToken.NodeID = strings.TrimSpace(startNodeID)
		ensureWorkflowTokenBranch(runtimeCtx, runtimeCtx.CurrentToken)
		runtimeCtx.ActiveTokens = []*WorkflowToken{runtimeCtx.CurrentToken}
		if runtimeCtx.RootToken == nil && runtimeCtx.CurrentToken.ParentTokenID == "" {
			runtimeCtx.RootToken = runtimeCtx.CurrentToken
		}
		return
	}
	if runtimeCtx.RootToken == nil {
		runtimeCtx.RootToken = newWorkflowRootToken(startNodeID)
	}
	runtimeCtx.RootToken.NodeID = strings.TrimSpace(startNodeID)
	ensureWorkflowTokenBranch(runtimeCtx, runtimeCtx.RootToken)
	runtimeCtx.ActiveTokens = []*WorkflowToken{runtimeCtx.RootToken}
}

func ensureWorkflowTokenBranch(runtimeCtx *WorkflowExecutionContext, token *WorkflowToken) {
	if runtimeCtx == nil || token == nil || token.Branch != nil {
		return
	}
	token.Branch = newWorkflowBranchContext(workflowRuntimeSharedInput(runtimeCtx), runtimeCtx.Metadata)
}

func storeWorkflowNodeResult(token *WorkflowToken, result WorkflowNodeResult) {
	if token == nil || token.Branch == nil {
		return
	}
	if len(result.Output) > 0 {
		token.Branch.Result = cloneWorkflowPayload(result.Output)
	}
}

func pauseWorkflowToken(token *WorkflowToken, pauseState *WorkflowPauseState) {
	if token == nil {
		return
	}
	pauseToken(token)
	if pauseState == nil {
		token.PauseState = nil
		return
	}
	payload := cloneWorkflowPayload(pauseState.Payload)
	allowedResumeModes := make([]WorkflowResumeMode, 0, len(pauseState.AllowedResumeModes))
	allowedResumeModes = append(allowedResumeModes, pauseState.AllowedResumeModes...)
	token.PauseState = &WorkflowPauseState{
		Source:             pauseState.Source,
		Reason:             pauseState.Reason,
		Payload:            payload,
		AllowedResumeModes: allowedResumeModes,
	}
}

func rebindWorkflowNodeResult(runtimeCtx *WorkflowExecutionContext, token *WorkflowToken) {
	if runtimeCtx == nil || token == nil || token.Branch == nil {
		return
	}
	runtimeCtx.Result = token.Branch.Result
	bindWorkflowPayloadBridge(runtimeCtx)
}

func workflowRuntimeInput(ctx *domain.WorkflowContext) map[string]any {
	if ctx == nil {
		return map[string]any{}
	}
	if ctx.Payload == nil {
		ctx.Payload = map[string]any{}
	}
	return ctx.Payload
}

func workflowRuntimeSharedInput(runtimeCtx *WorkflowExecutionContext) map[string]any {
	if runtimeCtx == nil {
		return map[string]any{}
	}
	if runtimeCtx.sharedInput != nil {
		return runtimeCtx.sharedInput
	}
	if runtimeCtx.Context != nil {
		runtimeCtx.sharedInput = cloneWorkflowPayload(workflowRuntimeInput(runtimeCtx.Context))
		if runtimeCtx.Input == nil {
			runtimeCtx.Input = cloneWorkflowPayload(runtimeCtx.sharedInput)
		}
		return runtimeCtx.sharedInput
	}
	runtimeCtx.sharedInput = map[string]any{}
	if runtimeCtx.Input == nil {
		runtimeCtx.Input = map[string]any{}
	}
	return runtimeCtx.sharedInput
}

func workflowRuntimeSharedMetadata(runtimeCtx *WorkflowExecutionContext) map[string]any {
	if runtimeCtx == nil {
		return map[string]any{}
	}
	if runtimeCtx.sharedMetadata != nil {
		return runtimeCtx.sharedMetadata
	}
	if runtimeCtx.Metadata != nil {
		runtimeCtx.sharedMetadata = runtimeCtx.Metadata
		return runtimeCtx.sharedMetadata
	}
	runtimeCtx.sharedMetadata = map[string]any{}
	runtimeCtx.Metadata = runtimeCtx.sharedMetadata
	return runtimeCtx.sharedMetadata
}

func bindWorkflowPayloadBridge(runtimeCtx *WorkflowExecutionContext) {
	if runtimeCtx == nil {
		return
	}
	if runtimeCtx.Context == nil {
		runtimeCtx.Context = &domain.WorkflowContext{}
	}
	runtimeCtx.Context.Payload = workflowRuntimePayload(runtimeCtx)
}

func workflowRuntimePayload(runtimeCtx *WorkflowExecutionContext) map[string]any {
	payload := cloneWorkflowPayload(workflowRuntimeSharedInput(runtimeCtx))
	for key, value := range runtimeCtx.Variables {
		payload[key] = value
	}
	for key, value := range runtimeCtx.Result {
		payload[key] = value
	}
	for key, value := range runtimeCtx.Artifacts {
		payload[key] = value
	}
	return payload
}

func popNextActiveWorkflowToken(runtimeCtx *WorkflowExecutionContext) *WorkflowToken {
	if runtimeCtx == nil || len(runtimeCtx.ActiveTokens) == 0 {
		return nil
	}
	last := len(runtimeCtx.ActiveTokens) - 1
	token := runtimeCtx.ActiveTokens[last]
	runtimeCtx.ActiveTokens = runtimeCtx.ActiveTokens[:last]
	return token
}

func pushActiveWorkflowToken(runtimeCtx *WorkflowExecutionContext, token *WorkflowToken) {
	if runtimeCtx == nil || token == nil {
		return
	}
	runtimeCtx.ActiveTokens = append(runtimeCtx.ActiveTokens, token)
}

func workflowRouteOutcomeSummaryForEdge(evaluation WorkflowRouteEvaluation, selected domain.WorkflowEdge) WorkflowRouteOutcomeSummary {
	summary := workflowRouteOutcomeSummary(evaluation)
	selectedIndex := indexOfWorkflowSelectedEdge(evaluation.EvaluatedEdges, selected)
	summary.SelectedNodeID = strings.TrimSpace(selected.ToNodeID)
	summary.SelectedEdgeID = workflowEvaluatedEdgeSummaryID(selectedIndex, selected)
	return summary
}

func validateWorkflowRuntimeGraph(wf *domain.WorkflowDefinition) error {
	if wf == nil {
		return fmt.Errorf("workflow definition is required")
	}

	nodeIDs := make(map[string]struct{}, len(wf.Nodes))
	for _, node := range wf.Nodes {
		nodeIDs[node.ID] = struct{}{}
	}

	outgoing := make(map[string][]domain.WorkflowEdge, len(wf.Edges))
	for _, edge := range wf.Edges {
		outgoing[edge.FromNodeID] = append(outgoing[edge.FromNodeID], edge)
	}

	visited := make(map[string]struct{}, len(wf.Nodes))
	stack := make(map[string]struct{}, len(wf.Nodes))
	var walk func(string) error
	walk = func(nodeID string) error {
		if _, ok := nodeIDs[nodeID]; !ok {
			return fmt.Errorf("workflow entry path references unknown node: %s", nodeID)
		}
		if _, ok := stack[nodeID]; ok {
			return fmt.Errorf("unsupported cycle in workflow graph at node %s", nodeID)
		}
		if _, ok := visited[nodeID]; ok {
			return nil
		}
		visited[nodeID] = struct{}{}
		stack[nodeID] = struct{}{}

		for _, edge := range outgoing[nodeID] {
			if err := walk(edge.ToNodeID); err != nil {
				return err
			}
		}
		delete(stack, nodeID)
		return nil
	}

	if err := walk(wf.EntryNodeID); err != nil {
		return err
	}
	if len(visited) != len(nodeIDs) {
		return fmt.Errorf("unsupported disconnected workflow graph: only %d of %d nodes reachable from entry node %s", len(visited), len(nodeIDs), wf.EntryNodeID)
	}
	return nil
}

func outgoingEdges(edges []domain.WorkflowEdge, fromNodeID string) []domain.WorkflowEdge {
	result := make([]domain.WorkflowEdge, 0, len(edges))
	for _, edge := range edges {
		if edge.FromNodeID == fromNodeID {
			result = append(result, edge)
		}
	}
	return result
}

func indexWorkflowNodes(wf *domain.WorkflowDefinition) map[string]domain.WorkflowNode {
	result := make(map[string]domain.WorkflowNode, len(wf.Nodes))
	for _, node := range wf.Nodes {
		result[node.ID] = node
	}
	return result
}

func workflowLoopValidationAllowed(wf *domain.WorkflowDefinition, err error) bool {
	if wf == nil || err == nil {
		return false
	}
	message := err.Error()
	if !strings.Contains(message, "unsupported cycle in workflow graph") {
		return false
	}
	for _, node := range wf.Nodes {
		if workflowNodeIsLoop(node) {
			return true
		}
	}
	return false
}

func workflowNodeIsLoop(node domain.WorkflowNode) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(node.Name)), "loop") || strings.Contains(strings.ToLower(strings.TrimSpace(node.Type)), "loop")
}
