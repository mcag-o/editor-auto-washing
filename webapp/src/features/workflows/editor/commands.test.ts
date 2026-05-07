import { describe, expect, it, vi } from 'vitest';
import type { Edge, Node } from 'reactflow';
import type { WorkflowCanvasNodeData } from '../components/WorkflowGraphPanel';
import type { WorkflowEdgeData, WorkflowFormTemplate } from '../../../lib/mappers/workflow';
import { appendDownstreamNode, deleteSelectedNode, duplicateSelectedNode } from './commands';

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
    edges: overrides.edges ?? [createEdge('edge-1', 'node-entry', 'node-rewrite', 2)],
    ...overrides,
  };
}

describe('workflow editor commands', () => {
  it('appends a downstream node from the selected node and selects it', () => {
    const template = createTemplate();
    const createEdgeSpy = vi.fn((input: { source: string; target: string; condition?: string; priority: number }) =>
      createEdge(`edge-${input.source}-${input.target}-${input.priority}`, input.source, input.target, input.priority),
    );

    const result = appendDownstreamNode({
      template,
      selection: { selectedNodeId: 'node-rewrite', selectedEdgeId: null },
      createNodeId: () => 'node-render',
      createNode: ({ id, position, index }) =>
        createNode(id, { label: `新节点 ${index}`, type: 'render', rawType: 'render' }, position.x, position.y),
      createEdge: createEdgeSpy,
    });

    expect(result.selection).toEqual({
      selectedNodeId: 'node-render',
      selectedEdgeId: null,
    });
    expect(result.template.nodes).toHaveLength(3);
    expect(result.template.nodes[2]).toMatchObject({
      id: 'node-render',
      position: { x: 560, y: 120 },
      data: { label: '新节点 3', rawType: 'render' },
    });
    expect(result.template.edges.map((edge) => edge.id)).toEqual([
      'edge-1',
      'edge-node-rewrite-node-render-3',
    ]);
    expect(createEdgeSpy).toHaveBeenCalledWith({
      source: 'node-rewrite',
      target: 'node-render',
      condition: 'always',
      priority: 3,
    });
  });

  it('deletes the selected node, removes incident edges, and falls back to the next selection', () => {
    const syncNodes = vi.fn((nodes: Array<Node<WorkflowCanvasNodeData>>, entryNodeId: string | null) =>
      nodes.map((node) => ({
        ...node,
        data: {
          ...node.data,
          isEntry: node.id === entryNodeId,
        },
      })),
    );

    const result = deleteSelectedNode({
      template: createTemplate({
        edges: [
          createEdge('edge-1', 'node-entry', 'node-rewrite', 0),
          createEdge('edge-2', 'node-rewrite', 'node-render', 1),
        ],
        nodes: [
          createNode('node-entry', { label: '入口节点', type: 'input', rawType: 'input', isEntry: true }, 80, 120),
          createNode('node-rewrite', { label: '改写节点' }, 320, 120),
          createNode('node-render', { label: '渲染节点', type: 'render', rawType: 'render' }, 560, 120),
        ],
        entryNodeId: 'node-entry',
        nodeCount: 3,
      }),
      selection: { selectedNodeId: 'node-entry', selectedEdgeId: 'edge-1' },
      syncNodes,
    });

    expect(result.template.entryNodeId).toBe('node-rewrite');
    expect(result.template.nodes.map((node) => node.id)).toEqual(['node-rewrite', 'node-render']);
    expect(result.template.edges.map((edge) => edge.id)).toEqual(['edge-2']);
    expect(result.selection).toEqual({
      selectedNodeId: 'node-rewrite',
      selectedEdgeId: null,
    });
    expect(syncNodes).toHaveBeenCalledWith(expect.any(Array), 'node-rewrite');
  });

  it('duplicates the selected node with an offset position and selects the copy', () => {
    const result = duplicateSelectedNode({
      template: createTemplate(),
      selection: { selectedNodeId: 'node-rewrite', selectedEdgeId: null },
      createNodeId: () => 'node-rewrite-copy',
    });

    expect(result.selection).toEqual({
      selectedNodeId: 'node-rewrite-copy',
      selectedEdgeId: null,
    });
    expect(result.template.nodes).toHaveLength(3);
    expect(result.template.nodes[2]).toMatchObject({
      id: 'node-rewrite-copy',
      position: { x: 368, y: 168 },
      data: {
        label: '改写节点（副本）',
        type: 'rewrite',
        rawType: 'rewrite',
      },
    });
    expect(result.template.edges.map((edge) => edge.id)).toEqual([
      'edge-1',
    ]);
  });
});
