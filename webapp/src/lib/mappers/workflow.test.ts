import type { WorkflowDefinition } from '../api/types';
import { mapApiWorkflowToForm, mapWorkflowFormToApi, reconcileCreatedWorkflow } from './workflow';

function assert(condition: unknown, message: string): asserts condition {
  if (!condition) {
    throw new Error(message);
  }
}

function testPreservesUnknownWorkflowNodeTypes() {
  const workflow: WorkflowDefinition = {
    id: 'wf-unknown',
    name: 'Unknown Type Workflow',
    description: 'verifies round-trip preservation',
    version: 'v1',
    enabled: true,
    entry_node_id: 'node-1',
    updated_by: 'tester',
    updated_at: '2026-05-07T00:00:00Z',
    nodes: [
      {
        id: 'node-1',
        type: 'moderate',
        name: 'Moderation',
        config_json: JSON.stringify({
          label: 'Moderation',
          type: 'moderate',
          template: 'safety.template',
          model: 'gpt-safety',
          context: 'Preserve me',
          position: { x: 10, y: 20 },
        }),
      },
    ],
    edges: [],
  };

  const form = mapApiWorkflowToForm(workflow);
  assert(form.nodes[0]?.data.type === 'rewrite', 'unsupported backend types should render through constrained UI type');
  assert(form.nodes[0]?.data.rawType === 'moderate', 'raw backend node type should be preserved on load');

  const api = mapWorkflowFormToApi(form);
  assert(api.nodes[0]?.type === 'moderate', 'unknown node type should round-trip unchanged in node.type');
  const config = JSON.parse(api.nodes[0]?.config_json ?? '{}') as { type?: string };
  assert(config.type === 'moderate', 'unknown node type should round-trip unchanged in config_json.type');
}

function testReconcilesCreatedWorkflowReplacingTemporaryTemplate() {
  const result = reconcileCreatedWorkflow({
    created: { id: 'wf-real', name: 'Saved Workflow' },
    selectedTemplateId: 'workflow-local-1',
    temporaryTemplateId: 'workflow-local-1',
    templates: [
      { id: 'workflow-local-1', name: 'Local Draft' },
      { id: 'wf-existing', name: 'Existing Workflow' },
    ],
  });

  assert(result.selectedTemplateId === 'wf-real', 'selected workflow should switch to backend-assigned id after create');
  assert(result.templates.some((template) => template.id === 'wf-real'), 'created workflow should be inserted into state');
  assert(!result.templates.some((template) => template.id === 'workflow-local-1'), 'temporary workflow should be removed after create');
}

function testWorkflowMapperPreservesConfiguredEntryNodeAndPriorityOrder() {
  const api = mapWorkflowFormToApi({
    id: 'wf-entry',
    name: 'Entry Workflow',
    description: 'verifies entry node and edge ordering',
    version: 'v1',
    enabled: true,
    updatedBy: 'tester',
    updatedAt: '2026-05-07T00:00:00Z',
    entryNodeId: 'node-2',
    nodeCount: 2,
    nodes: [
      {
        id: 'node-1',
        type: 'default',
        position: { x: 10, y: 20 },
        data: {
          label: 'Input',
          type: 'input',
          rawType: 'input',
          template: '',
          model: '',
          context: '',
          isEntry: false,
        },
      },
      {
        id: 'node-2',
        type: 'default',
        position: { x: 30, y: 40 },
        data: {
          label: 'Rewrite',
          type: 'rewrite',
          rawType: 'rewrite',
          template: 'rewrite.standard',
          model: 'gpt-4.1-mini',
          context: 'keep structure',
          isEntry: true,
        },
      },
    ],
    edges: [
      { id: 'edge-b', source: 'node-2', target: 'node-1' },
      { id: 'edge-a', source: 'node-1', target: 'node-2' },
    ],
  });

  assert(api.entry_node_id === 'node-2', 'selected entry node should be preserved in API payload');
  assert(api.edges[0]?.priority === 0, 'first edge should receive priority 0');
  assert(api.edges[1]?.priority === 1, 'second edge should receive priority 1');
}

testPreservesUnknownWorkflowNodeTypes();
testReconcilesCreatedWorkflowReplacingTemporaryTemplate();
testWorkflowMapperPreservesConfiguredEntryNodeAndPriorityOrder();
