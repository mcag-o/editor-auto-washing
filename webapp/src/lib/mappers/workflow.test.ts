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
        { id: 'edge-b', source: 'node-2', target: 'node-1', data: { priority: 0 } },
        { id: 'edge-a', source: 'node-1', target: 'node-2', data: { priority: 1 } },
      ],
    });

    expect(api.entry_node_id).toBe('node-2');
    expect(api.edges[0]?.priority).toBe(0);
    expect(api.edges[1]?.priority).toBe(1);
  });

  it('preserves backend edge conditions through a load and save round-trip', () => {
    const workflow: WorkflowDefinition = {
      id: 'wf-conditions',
      name: 'Condition Workflow',
      description: 'verifies edge condition preservation',
      version: 'v1',
      enabled: true,
      entry_node_id: 'node-1',
      updated_by: 'tester',
      updated_at: '2026-05-07T00:00:00Z',
      nodes: [
        {
          id: 'node-1',
          type: 'input',
          name: 'Input',
          config_json: JSON.stringify({ label: 'Input', type: 'input' }),
        },
        {
          id: 'node-2',
          type: 'rewrite',
          name: 'Retry Handler',
          config_json: JSON.stringify({ label: 'Retry Handler', type: 'rewrite' }),
        },
      ],
      edges: [
        {
          from_node_id: 'node-1',
          to_node_id: 'node-2',
          condition: 'retry',
          priority: 0,
        },
      ],
    };

    const form = mapApiWorkflowToForm(workflow);

    expect(form.edges[0]?.label).toBe('retry');

    const api = mapWorkflowFormToApi(form);

    expect(api.edges[0]?.condition).toBe('retry');
  });

  it('preserves backend edge priority through a load and save round-trip', () => {
    const workflow: WorkflowDefinition = {
      id: 'wf-priority',
      name: 'Priority Workflow',
      description: 'verifies edge priority preservation',
      version: 'v1',
      enabled: true,
      entry_node_id: 'node-1',
      updated_by: 'tester',
      updated_at: '2026-05-07T00:00:00Z',
      nodes: [
        {
          id: 'node-1',
          type: 'input',
          name: 'Input',
          config_json: JSON.stringify({ label: 'Input', type: 'input' }),
        },
        {
          id: 'node-2',
          type: 'rewrite',
          name: 'Priority Handler',
          config_json: JSON.stringify({ label: 'Priority Handler', type: 'rewrite' }),
        },
      ],
      edges: [
        {
          from_node_id: 'node-1',
          to_node_id: 'node-2',
          condition: 'retry',
          priority: 7,
        },
      ],
    };

    const form = mapApiWorkflowToForm(workflow);
    const api = mapWorkflowFormToApi(form);

    expect(api.edges[0]?.priority).toBe(7);
  });

  it('defaults a newly created edge condition to always when no prior condition exists', () => {
    const api = mapWorkflowFormToApi({
      id: 'wf-new-edge',
      name: 'New Edge Workflow',
      description: 'verifies new edge default',
      version: 'v1',
      enabled: true,
      updatedBy: 'tester',
      updatedAt: '2026-05-07T00:00:00Z',
      entryNodeId: 'node-1',
      nodeCount: 2,
      nodes: [
        {
          id: 'node-1',
          type: 'default',
          position: { x: 0, y: 0 },
          data: {
            label: 'Start',
            type: 'input',
            rawType: 'input',
            template: '',
            model: '',
            context: '',
            isEntry: true,
          },
        },
        {
          id: 'node-2',
          type: 'default',
          position: { x: 100, y: 0 },
          data: {
            label: 'Next',
            type: 'rewrite',
            rawType: 'rewrite',
            template: '',
            model: '',
            context: '',
            isEntry: false,
          },
        },
      ],
      edges: [{ id: 'edge-new', source: 'node-1', target: 'node-2' }],
    });

    expect(api.edges[0]?.condition).toBe('always');
  });

  it('keeps same-pair backend edges distinct after mapping to frontend state', () => {
    const workflow: WorkflowDefinition = {
      id: 'wf-duplicate-pair',
      name: 'Duplicate Pair Workflow',
      description: 'verifies edge identity for same source and target',
      version: 'v1',
      enabled: true,
      entry_node_id: 'node-1',
      updated_by: 'tester',
      updated_at: '2026-05-07T00:00:00Z',
      nodes: [
        {
          id: 'node-1',
          type: 'input',
          name: 'Start',
          config_json: JSON.stringify({ label: 'Start', type: 'input' }),
        },
        {
          id: 'node-2',
          type: 'rewrite',
          name: 'Decision',
          config_json: JSON.stringify({ label: 'Decision', type: 'rewrite' }),
        },
      ],
      edges: [
        {
          from_node_id: 'node-1',
          to_node_id: 'node-2',
          condition: 'retry',
          priority: 0,
        },
        {
          from_node_id: 'node-1',
          to_node_id: 'node-2',
          condition: 'fallback',
          priority: 1,
        },
      ],
    };

    const form = mapApiWorkflowToForm(workflow);

    expect(form.edges).toHaveLength(2);
    expect(form.edges[0]?.id).not.toBe(form.edges[1]?.id);
    expect(form.edges.map((edge) => edge.label)).toEqual(['retry', 'fallback']);
  });

  it('round-trips same-pair edges with different conditions without collapsing them', () => {
    const workflow: WorkflowDefinition = {
      id: 'wf-duplicate-round-trip',
      name: 'Duplicate Round Trip Workflow',
      description: 'verifies same-pair edge round-trip',
      version: 'v1',
      enabled: true,
      entry_node_id: 'node-1',
      updated_by: 'tester',
      updated_at: '2026-05-07T00:00:00Z',
      nodes: [
        {
          id: 'node-1',
          type: 'input',
          name: 'Start',
          config_json: JSON.stringify({ label: 'Start', type: 'input' }),
        },
        {
          id: 'node-2',
          type: 'rewrite',
          name: 'Decision',
          config_json: JSON.stringify({ label: 'Decision', type: 'rewrite' }),
        },
      ],
      edges: [
        {
          from_node_id: 'node-1',
          to_node_id: 'node-2',
          condition: 'retry',
          priority: 0,
        },
        {
          from_node_id: 'node-1',
          to_node_id: 'node-2',
          condition: 'fallback',
          priority: 1,
        },
      ],
    };

    const form = mapApiWorkflowToForm(workflow);
    const api = mapWorkflowFormToApi(form);

    expect(api.edges).toHaveLength(2);
    expect(api.edges.map((edge) => edge.condition)).toEqual(['retry', 'fallback']);
    expect(api.edges.map((edge) => edge.priority)).toEqual([0, 1]);
  });

  it('does not rely on source-target alone for local edge identity', () => {
    const api = mapWorkflowFormToApi({
      id: 'wf-local-edges',
      name: 'Local Edge Workflow',
      description: 'verifies local edge identity can represent same-pair edges',
      version: 'v1',
      enabled: true,
      updatedBy: 'tester',
      updatedAt: '2026-05-07T00:00:00Z',
      entryNodeId: 'node-1',
      nodeCount: 2,
      nodes: [
        {
          id: 'node-1',
          type: 'default',
          position: { x: 0, y: 0 },
          data: {
            label: 'Start',
            type: 'input',
            rawType: 'input',
            template: '',
            model: '',
            context: '',
            isEntry: true,
          },
        },
        {
          id: 'node-2',
          type: 'default',
          position: { x: 100, y: 0 },
          data: {
            label: 'Next',
            type: 'rewrite',
            rawType: 'rewrite',
            template: '',
            model: '',
            context: '',
            isEntry: false,
          },
        },
      ],
      edges: [
        { id: 'edge-node-1-node-2-retry-0', source: 'node-1', target: 'node-2', label: 'retry' },
        { id: 'edge-node-1-node-2-fallback-1', source: 'node-1', target: 'node-2', label: 'fallback' },
      ],
    });

    expect(api.edges).toHaveLength(2);
    expect(api.edges.map((edge) => edge.condition)).toEqual(['retry', 'fallback']);
  });
});
