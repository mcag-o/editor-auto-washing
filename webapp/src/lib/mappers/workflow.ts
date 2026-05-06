import type { Edge, Node } from 'reactflow';
import type {
  WorkflowDefinition,
  WorkflowDefinitionInput,
  WorkflowEdgeDefinition,
  WorkflowNodeConfigPayload,
  WorkflowNodeDefinition,
} from '../api/types';
import type { WorkflowCanvasNodeData } from '../../features/workflows/components/WorkflowGraphPanel';

export type WorkflowFormTemplate = {
  id: string;
  name: string;
  description: string;
  version: string;
  enabled: boolean;
  updatedBy: string;
  updatedAt: string;
  entryNodeId: string | null;
  nodeCount: number;
  nodes: Array<Node<WorkflowCanvasNodeData>>;
  edges: Edge[];
};

export function mapApiWorkflowToForm(workflow: WorkflowDefinition): WorkflowFormTemplate {
  const nodes = workflow.nodes.map((node, index) => {
    let config: WorkflowNodeConfigPayload = {};
    try {
      config = JSON.parse(node.config_json || '{}') as WorkflowNodeConfigPayload;
    } catch {
      config = {};
    }

    const position = config.position ?? {
      x: 80 + (index % 3) * 240,
      y: 120 + Math.floor(index / 3) * 180,
    };

    return {
      id: node.id,
      type: 'default',
      position,
      data: {
        label: config.label || node.name,
        type: (config.type || node.type || 'rewrite') as WorkflowCanvasNodeData['type'],
        template: config.template || '',
        model: config.model || '',
        context: config.context || '',
        isEntry: node.id === workflow.entry_node_id,
      },
    } as Node<WorkflowCanvasNodeData>;
  });

  const edges: Edge[] = workflow.edges.map((edge) => ({
    id: `edge-${edge.from_node_id}-${edge.to_node_id}`,
    source: edge.from_node_id,
    target: edge.to_node_id,
  }));

  return {
    id: workflow.id,
    name: workflow.name,
    description: workflow.description,
    version: workflow.version,
    enabled: workflow.enabled,
    updatedBy: workflow.updated_by,
    updatedAt: workflow.updated_at,
    entryNodeId: workflow.entry_node_id,
    nodeCount: nodes.length,
    nodes,
    edges,
  };
}

export function mapWorkflowFormToApi(template: WorkflowFormTemplate): WorkflowDefinitionInput {
  const nodes: WorkflowNodeDefinition[] = template.nodes.map((node) => ({
    id: node.id,
    type: node.data.type,
    name: node.data.label,
    config_json: JSON.stringify({
      label: node.data.label,
      type: node.data.type,
      template: node.data.template,
      model: node.data.model,
      context: node.data.context,
      position: node.position,
    } satisfies WorkflowNodeConfigPayload),
  }));

  const edges: WorkflowEdgeDefinition[] = template.edges.map((edge, index) => ({
    from_node_id: edge.source,
    to_node_id: edge.target,
    condition: '',
    priority: index,
  }));

  return {
    name: template.name,
    description: template.description,
    version: template.version,
    enabled: template.enabled,
    entry_node_id: template.entryNodeId || template.nodes[0]?.id || '',
    nodes,
    edges,
    updated_by: template.updatedBy,
  };
}
