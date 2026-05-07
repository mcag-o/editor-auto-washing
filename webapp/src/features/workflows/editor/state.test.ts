import { describe, expect, it } from 'vitest';
import type { Edge, Node } from 'reactflow';
import type { WorkflowCanvasNodeData } from '../components/WorkflowGraphPanel';
import type { WorkflowEdgeData, WorkflowFormTemplate } from '../../../lib/mappers/workflow';
import { createWorkflowDraftSignature, getInitialWorkflowSelection, isWorkflowDraftDirty } from './state';

function createNode(id: string, overrides: Partial<WorkflowCanvasNodeData> = {}, x = 0, y = 0): Node<WorkflowCanvasNodeData> {
  return {
    id,
    type: 'default',
    position: { x, y },
    data: {
      label: id,
      type: 'rewrite',
      template: '',
      model: '',
      context: '',
      rawType: 'rewrite',
      ...overrides,
    },
  };
}

function createEdge(id: string, source: string, target: string, priority = 0): Edge<WorkflowEdgeData> {
  return {
    id,
    source,
    target,
    label: 'always',
    data: { priority },
  };
}

function createTemplate(overrides: Partial<WorkflowFormTemplate> = {}): WorkflowFormTemplate {
  const nodes = overrides.nodes ?? [
    createNode('node-entry', { label: '入口节点', type: 'input', rawType: 'input', isEntry: true }, 80, 120),
    createNode('node-rewrite', { label: '改写节点' }, 320, 120),
  ];

  return {
    id: 'workflow-1',
    name: '工作流模板',
    description: '工作流描述',
    version: 'v1.0.0',
    enabled: true,
    updatedBy: 'tester',
    updatedAt: '2026-05-07T00:00:00Z',
    entryNodeId: 'node-entry',
    nodeCount: nodes.length,
    nodes,
    edges: overrides.edges ?? [createEdge('edge-1', 'node-entry', 'node-rewrite', 0)],
    ...overrides,
  };
}

describe('workflow editor state helpers', () => {
  it('prefers the entry node when computing initial selection', () => {
    const selection = getInitialWorkflowSelection(createTemplate());

    expect(selection).toEqual({
      selectedNodeId: 'node-entry',
      selectedEdgeId: null,
    });
  });

  it('falls back to the first node when the template has no entry node', () => {
    const template = createTemplate({
      entryNodeId: null,
    });

    const selection = getInitialWorkflowSelection(template);

    expect(selection).toEqual({
      selectedNodeId: 'node-entry',
      selectedEdgeId: null,
    });
  });

  it('treats identical drafts as clean and changes to raw types as dirty', () => {
    const baseline = createTemplate({
      nodes: [
        createNode('node-entry', { label: '入口节点', type: 'rewrite', rawType: 'vendor.custom', isEntry: true }, 80, 120),
      ],
      edges: [],
      entryNodeId: 'node-entry',
      nodeCount: 1,
    });

    const baselineSignature = createWorkflowDraftSignature(baseline);
    const identicalDraft = createTemplate({
      nodes: [
        createNode('node-entry', { label: '入口节点', type: 'rewrite', rawType: 'vendor.custom', isEntry: true }, 80, 120),
      ],
      edges: [],
      entryNodeId: 'node-entry',
      nodeCount: 1,
    });
    const changedDraft = createTemplate({
      nodes: [
        createNode('node-entry', { label: '入口节点', type: 'rewrite', rawType: 'vendor.experimental', isEntry: true }, 80, 120),
      ],
      edges: [],
      entryNodeId: 'node-entry',
      nodeCount: 1,
    });

    expect(isWorkflowDraftDirty(identicalDraft, baselineSignature)).toBe(false);
    expect(isWorkflowDraftDirty(changedDraft, baselineSignature)).toBe(true);
  });

  it('includes edge ids in the draft signature so local edge identity changes stay visible', () => {
    const firstSignature = createWorkflowDraftSignature(
      createTemplate({
        edges: [createEdge('edge-a', 'node-entry', 'node-rewrite', 0)],
      }),
    );
    const secondSignature = createWorkflowDraftSignature(
      createTemplate({
        edges: [createEdge('edge-b', 'node-entry', 'node-rewrite', 0)],
      }),
    );

    expect(secondSignature).not.toBe(firstSignature);
  });
});
