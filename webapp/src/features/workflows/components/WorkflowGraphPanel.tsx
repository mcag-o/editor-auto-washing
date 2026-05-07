import { useEffect } from 'react';
import type { Connection, Edge, EdgeChange, Node, NodeChange } from 'reactflow';
import { Background, Controls, MiniMap, Panel, ReactFlow, useReactFlow } from 'reactflow';
import Box from '@mui/material/Box';
import Chip from '@mui/material/Chip';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { alpha } from '@mui/material/styles';
import type { WorkflowNodeFormValue } from './WorkflowNodeDrawer';

export type WorkflowCanvasNodeData = WorkflowNodeFormValue & {
  isEntry?: boolean;
  rawType?: string;
};

type WorkflowGraphPanelProps = {
  edges: Edge[];
  fitViewRequest: number;
  nodes: Array<Node<WorkflowCanvasNodeData>>;
  onConnect: (params: Connection) => void;
  onClearSelection: () => void;
  onEdgesChange: (changes: EdgeChange[]) => void;
  onEdgeClick: (edge: Edge) => void;
  onNodeClick: (node: Node<WorkflowCanvasNodeData> | null) => void;
  onNodesChange: (changes: NodeChange[]) => void;
  selectedEdgeId: string | null;
  selectedEdgeLabel: string | null;
  selectedNodeId: string | null;
  selectedNodeLabel: string | null;
  selectedTemplateName: string;
};

function FitViewOnRequest({ request }: { request: number }) {
  const reactFlow = useReactFlow<WorkflowCanvasNodeData>();

  useEffect(() => {
    if (request === 0) {
      return;
    }

    void reactFlow.fitView({ duration: 220, padding: 0.2 });
  }, [request, reactFlow]);

  return null;
}

export default function WorkflowGraphPanel({
  edges,
  fitViewRequest,
  nodes,
  onConnect,
  onClearSelection,
  onEdgeClick,
  onEdgesChange,
  onNodeClick,
  onNodesChange,
  selectedEdgeId,
  selectedEdgeLabel,
  selectedNodeId,
  selectedNodeLabel,
  selectedTemplateName,
}: WorkflowGraphPanelProps) {
  return (
    <Box
      sx={{
        position: 'relative',
        minHeight: 760,
        borderRadius: 6,
        overflow: 'hidden',
        border: `1px solid ${alpha('#15304f', 0.12)}`,
        bgcolor: '#f6f9ff',
        backgroundImage:
          'radial-gradient(circle at top left, rgba(91, 61, 245, 0.16), transparent 28%), radial-gradient(circle at bottom right, rgba(15, 98, 254, 0.2), transparent 32%)',
        boxShadow: '0 28px 64px rgba(20, 32, 51, 0.12)',
      }}
    >
      <ReactFlow
        fitView
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        onNodeClick={(_, node) => onNodeClick(node)}
        onEdgeClick={(_, edge) => onEdgeClick(edge)}
        onPaneClick={onClearSelection}
        defaultEdgeOptions={{ animated: false, style: { strokeWidth: 2, stroke: '#0f62fe' } }}
      >
        <FitViewOnRequest request={fitViewRequest} />
        <MiniMap
          pannable
          zoomable
          style={{ background: 'rgba(255, 255, 255, 0.92)', border: `1px solid ${alpha('#15304f', 0.1)}` }}
          nodeColor={(node) => (node.id === selectedNodeId ? '#5b3df5' : node.data?.isEntry ? '#0f62fe' : '#9fb4d9')}
        />
        <Controls showInteractive={false} />
        <Background gap={18} size={1} color="rgba(21, 48, 79, 0.12)" />

        <Panel position="top-left">
          <Stack
            spacing={1}
            sx={{
              p: 1.5,
              borderRadius: 4,
              bgcolor: alpha('#ffffff', 0.92),
              border: `1px solid ${alpha('#15304f', 0.1)}`,
              boxShadow: '0 12px 30px rgba(20, 32, 51, 0.08)',
            }}
          >
            <Typography variant="subtitle2">当前模板</Typography>
            <Typography variant="body2" color="text.secondary">
              {selectedTemplateName}
            </Typography>
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} useFlexGap flexWrap="wrap">
              <Chip size="small" color={selectedNodeId ? 'primary' : 'default'} variant={selectedNodeId ? 'filled' : 'outlined'} label={selectedNodeLabel ? `节点: ${selectedNodeLabel}` : '当前未选择节点'} />
              <Chip size="small" color={selectedEdgeId ? 'secondary' : 'default'} variant={selectedEdgeId ? 'filled' : 'outlined'} label={selectedEdgeLabel ? `连线: ${selectedEdgeLabel}` : '当前未选择连线'} />
            </Stack>
            <Chip size="small" label="拖拽节点可调整布局，拖出连线即可建立连接" />
          </Stack>
        </Panel>
      </ReactFlow>
    </Box>
  );
}
