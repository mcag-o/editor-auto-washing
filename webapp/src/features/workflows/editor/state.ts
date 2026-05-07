import type { WorkflowEditorDraftSignature, WorkflowEditorTemplate, WorkflowEditorSelection } from './types';

export function getInitialWorkflowSelection(template: WorkflowEditorTemplate | null | undefined): WorkflowEditorSelection {
  return {
    selectedNodeId: template?.entryNodeId ?? template?.nodes[0]?.id ?? null,
    selectedEdgeId: null,
  };
}

export function createWorkflowDraftSignature(template: WorkflowEditorTemplate): WorkflowEditorDraftSignature {
  return JSON.stringify({
    name: template.name,
    description: template.description,
    version: template.version,
    enabled: template.enabled,
    entryNodeId: template.entryNodeId,
    nodes: template.nodes.map((node) => ({
      id: node.id,
      position: node.position,
      label: node.data.label,
      type: node.data.type,
      rawType: node.data.rawType,
      template: node.data.template,
      model: node.data.model,
      context: node.data.context,
    })),
    edges: template.edges.map((edge) => ({
      id: edge.id,
      source: edge.source,
      target: edge.target,
      label: edge.label ? String(edge.label) : 'always',
      priority: edge.data?.priority ?? 0,
    })),
  });
}

export function isWorkflowDraftDirty(template: WorkflowEditorTemplate, baselineSignature: WorkflowEditorDraftSignature | null | undefined): boolean {
  if (!baselineSignature) {
    return true;
  }

  return createWorkflowDraftSignature(template) !== baselineSignature;
}
