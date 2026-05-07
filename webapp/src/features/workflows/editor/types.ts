import type { Edge, Node } from 'reactflow';
import type { WorkflowCanvasNodeData } from '../components/WorkflowGraphPanel';
import type { WorkflowEdgeData, WorkflowFormTemplate } from '../../../lib/mappers/workflow';

export type WorkflowEditorSelection = {
  selectedNodeId: string | null;
  selectedEdgeId: string | null;
};

export type WorkflowEditorDraftSignature = string;

export type WorkflowEditorNode = Node<WorkflowCanvasNodeData>;

export type WorkflowEditorEdge = Edge<WorkflowEdgeData>;

export type WorkflowEditorTemplate = WorkflowFormTemplate;
