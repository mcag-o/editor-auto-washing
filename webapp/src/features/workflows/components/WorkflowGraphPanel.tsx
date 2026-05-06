import type { Connection, Edge, EdgeChange, Node, NodeChange } from 'reactflow';
import { Background, Controls, MiniMap, Panel, ReactFlow } from 'reactflow';
import Box from '@mui/material/Box';
import Chip from '@mui/material/Chip';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { alpha } from '@mui/material/styles';
import type { WorkflowNodeFormValue } from './WorkflowNodeDrawer';

export type WorkflowCanvasNodeData = WorkflowNodeFormValue & {
  isEntry?: boolean;
};

type WorkflowGraphPanelProps = {
  edges: Edge[];
  nodes: Array<Node<WorkflowCanvasNodeData>>;
  onConnect: (params: Connection) => void;
  onEdgesChange: (changes: EdgeChange[]) => void;
  onEdgeClick: (edge: Edge) => void;
  onNodeClick: (node: Node<WorkflowCanvasNodeData> | null) => void;
  onNodesChange: (changes: NodeChange[]) => void;
  selectedNodeId: string | null;
  selectedTemplateName: string;
};

export default function WorkflowGraphPanel({
  edges,
  nodes,
  onConnect,
  onEdgeClick,
  onEdgesChange,
  onNodeClick,
  onNodesChange,
  selectedNodeId,
  selectedTemplateName,
}: WorkflowGraphPanelProps) {
  return (
    <Box
      sx={{
        position: 'relative',
        minHeight: 720,
        borderRadius: 6,
        overflow: 'hidden',
        border: '1px solid',
        borderColor: 'divider',
        bgcolor: '#f8fbff',
        backgroundImage:
          'radial-gradient(circle at top left, rgba(91, 61, 245, 0.14), transparent 28%), radial-gradient(circle at bottom right, rgba(15, 98, 254, 0.18), transparent 32%)',
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
        onPaneClick={() => onNodeClick(null as never)}
        defaultEdgeOptions={{ animated: false, style: { strokeWidth: 2, stroke: '#0f62fe' } }}
      >
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
            <Chip size="small" label="拖拽节点可调整布局，拖出连线即可建立连接" />
          </Stack>
        </Panel>
      </ReactFlow>
    </Box>
  );
}
