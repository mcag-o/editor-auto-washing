import { describe, expect, it } from 'vitest';
import type { WorkflowDefinition } from '../api/types';
import { mapApiWorkflowToForm, mapWorkflowFormToApi, reconcileCreatedWorkflow } from './workflow';

describe('workflow mapper contract', () => {
  it('preserves unknown backend node types through round-trip mapping', () => {
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

    expect(form.nodes[0]?.data.type).toBe('rewrite');
    expect(form.nodes[0]?.data.rawType).toBe('moderate');

    const api = mapWorkflowFormToApi(form);
    const config = JSON.parse(api.nodes[0]?.config_json ?? '{}') as { type?: string };

    expect(api.nodes[0]?.type).toBe('moderate');
    expect(config.type).toBe('moderate');
  });

  it('reconciles created workflows by replacing temporary ids', () => {
    const result = reconcileCreatedWorkflow({
      created: { id: 'wf-real', name: 'Saved Workflow' },
      selectedTemplateId: 'workflow-local-1',
      temporaryTemplateId: 'workflow-local-1',
      templates: [
        { id: 'workflow-local-1', name: 'Local Draft' },
        { id: 'wf-existing', name: 'Existing Workflow' },
      ],
    });

    expect(result.selectedTemplateId).toBe('wf-real');
    expect(result.templates).toContainEqual({ id: 'wf-real', name: 'Saved Workflow' });
    expect(result.templates.some((template) => template.id === 'workflow-local-1')).toBe(false);
  });

  it('preserves the configured entry node and edge priority order', () => {
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

    expect(api.entry_node_id).toBe('node-2');
    expect(api.edges[0]?.priority).toBe(0);
    expect(api.edges[1]?.priority).toBe(1);
  });
});
