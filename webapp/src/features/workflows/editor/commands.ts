import type { WorkflowCanvasNodeData } from '../components/WorkflowGraphPanel';
import type { WorkflowEditorNode, WorkflowEditorSelection, WorkflowEditorTemplate } from './types';

type CreateNodeInput = {
  id: string;
  index: number;
  position: { x: number; y: number };
};

type AppendDownstreamNodeInput = {
  template: WorkflowEditorTemplate;
  selection: WorkflowEditorSelection;
  createNodeId: () => string;
  createNode: (input: CreateNodeInput) => WorkflowEditorNode;
  createEdge: (input: { source: string; target: string; condition?: string; priority: number }) => WorkflowEditorTemplate['edges'][number];
};

type DeleteSelectedNodeInput = {
  template: WorkflowEditorTemplate;
  selection: WorkflowEditorSelection;
  syncNodes: (nodes: Array<WorkflowEditorNode>, entryNodeId: string | null) => Array<WorkflowEditorNode>;
};

type DuplicateSelectedNodeInput = {
  template: WorkflowEditorTemplate;
  selection: WorkflowEditorSelection;
  createNodeId: () => string;
};

type WorkflowEditorCommandResult = {
  template: WorkflowEditorTemplate;
  selection: WorkflowEditorSelection;
};

export function appendDownstreamNode({ template, selection, createNodeId, createNode, createEdge }: AppendDownstreamNodeInput): WorkflowEditorCommandResult {
  const anchorNode = template.nodes.find((node) => node.id === selection.selectedNodeId) ?? template.nodes[template.nodes.length - 1] ?? null;
  const nextIndex = template.nodes.length + 1;
  const nextNodeId = createNodeId();
  const nextNode = createNode({
    id: nextNodeId,
    index: nextIndex,
    position: {
      x: (anchorNode?.position.x ?? 120) + 240,
      y: anchorNode?.position.y ?? 120,
    },
  });
  const nextPriority = template.edges.reduce((maxPriority, edge) => Math.max(maxPriority, edge.data?.priority ?? -1), -1) + 1;
  const nextEdges = anchorNode
    ? [
        ...template.edges,
        createEdge({
          source: anchorNode.id,
          target: nextNodeId,
          condition: 'always',
          priority: nextPriority,
        }),
      ]
    : template.edges;

  return {
    template: {
      ...template,
      nodes: [...template.nodes, nextNode],
      edges: nextEdges,
      nodeCount: template.nodes.length + 1,
    },
    selection: {
      selectedNodeId: nextNodeId,
      selectedEdgeId: null,
    },
  };
}

export function deleteSelectedNode({ template, selection, syncNodes }: DeleteSelectedNodeInput): WorkflowEditorCommandResult {
  if (!selection.selectedNodeId) {
    return {
      template,
      selection,
    };
  }

  const nextNodes = template.nodes.filter((node) => node.id !== selection.selectedNodeId);
  const nextEdges = template.edges.filter((edge) => edge.source !== selection.selectedNodeId && edge.target !== selection.selectedNodeId);
  const nextEntryNodeId = template.entryNodeId === selection.selectedNodeId ? nextNodes[0]?.id ?? null : template.entryNodeId;
  const syncedNodes = syncNodes(nextNodes, nextEntryNodeId);

  return {
    template: {
      ...template,
      entryNodeId: nextEntryNodeId,
      nodes: syncedNodes,
      edges: nextEdges,
      nodeCount: syncedNodes.length,
    },
    selection: {
      selectedNodeId: template.nodes.find((node) => node.id !== selection.selectedNodeId)?.id ?? null,
      selectedEdgeId: null,
    },
  };
}

export function duplicateSelectedNode({ template, selection, createNodeId }: DuplicateSelectedNodeInput): WorkflowEditorCommandResult {
  if (!selection.selectedNodeId) {
    return {
      template,
      selection,
    };
  }

  const sourceNode = template.nodes.find((node) => node.id === selection.selectedNodeId);
  if (!sourceNode) {
    return {
      template,
      selection,
    };
  }

  const duplicateNodeId = createNodeId();
  const duplicateNode = {
    ...sourceNode,
    id: duplicateNodeId,
    position: {
      x: sourceNode.position.x + 48,
      y: sourceNode.position.y + 48,
    },
    data: {
      ...sourceNode.data,
      label: `${sourceNode.data.label}（副本）`,
      isEntry: false,
    } as WorkflowCanvasNodeData,
  };

  return {
    template: {
      ...template,
      nodes: [...template.nodes, duplicateNode],
      edges: template.edges,
      nodeCount: template.nodes.length + 1,
    },
    selection: {
      selectedNodeId: duplicateNodeId,
      selectedEdgeId: null,
    },
  };
}
