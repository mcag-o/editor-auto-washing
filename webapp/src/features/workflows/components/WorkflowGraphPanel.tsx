import { useEffect } from 'react';
import type { Connection, Edge, EdgeChange, Node, NodeChange } from 'reactflow';
import { Background, Controls, MiniMap, Panel, ReactFlow, useReactFlow } from 'reactflow';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Divider from '@mui/material/Divider';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { alpha } from '@mui/material/styles';
import WorkflowCanvasQuickActions from './WorkflowCanvasQuickActions';
import type { WorkflowNodeFormValue } from './WorkflowNodeDrawer';
import WorkflowNodeCreateMenu from './WorkflowNodeCreateMenu';

export type WorkflowCanvasNodeData = WorkflowNodeFormValue & {
  isEntry?: boolean;
  rawType?: string;
};

type WorkflowGraphPanelProps = {
  edges: Edge[];
  fitViewRequest: number;
  isDirty: boolean;
  nodes: Array<Node<WorkflowCanvasNodeData>>;
  onConnect: (params: Connection) => void;
  onOpenAppendMenu: () => void;
  onOpenCreateMenu: () => void;
  onCloseCreateMenu: () => void;
  onCreateNode: (type: WorkflowCanvasNodeData['type']) => void;
  onClearSelection: () => void;
  onDeleteSelection: () => void;
  onDuplicateNode: () => void;
  onSetEntryNode: () => void;
  onEdgesChange: (changes: EdgeChange[]) => void;
  onEdgeClick: (edge: Edge) => void;
  onNodeClick: (node: Node<WorkflowCanvasNodeData> | null) => void;
  onNodesChange: (changes: NodeChange[]) => void;
  selectedEdgeId: string | null;
  selectedEdgeLabel: string | null;
  selectedNodeId: string | null;
  selectedNodeLabel: string | null;
  selectedTemplateName: string;
  isPanelCollapsed: boolean;
  createMenuMode: 'create' | 'append' | null;
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
  isDirty,
  nodes,
  onConnect,
  onOpenAppendMenu,
  onOpenCreateMenu,
  onCloseCreateMenu,
  onCreateNode,
  onClearSelection,
  onDeleteSelection,
  onDuplicateNode,
  onSetEntryNode,
  onEdgeClick,
  onEdgesChange,
  onNodeClick,
  onNodesChange,
  selectedEdgeId,
  selectedEdgeLabel,
  selectedNodeId,
  selectedNodeLabel,
  selectedTemplateName,
  isPanelCollapsed,
  createMenuMode,
}: WorkflowGraphPanelProps) {
  const selectionKind = selectedEdgeId ? 'edge' : selectedNodeId ? 'node' : 'idle';

  return (
    <Box
      component="section"
      role="region"
      aria-label="工作流画布"
      data-testid="workflow-graph-panel"
      data-dirty-state={isDirty ? 'dirty' : 'clean'}
      data-emphasis="primary"
      data-selection-kind={selectionKind}
      data-panel-state={isPanelCollapsed ? 'collapsed' : 'expanded'}
      sx={{
        position: 'relative',
        minHeight: 760,
        borderRadius: 6,
        overflow: 'hidden',
        border: `1px solid ${selectionKind === 'edge' ? alpha('#5b3df5', 0.26) : alpha('#15304f', 0.12)}`,
        bgcolor: '#f6f9ff',
        backgroundImage:
          'radial-gradient(circle at top left, rgba(91, 61, 245, 0.16), transparent 28%), radial-gradient(circle at bottom right, rgba(15, 98, 254, 0.2), transparent 32%)',
        boxShadow:
          selectionKind === 'edge'
            ? '0 32px 72px rgba(91, 61, 245, 0.18)'
            : selectionKind === 'node'
              ? '0 32px 72px rgba(15, 98, 254, 0.16)'
              : '0 28px 64px rgba(20, 32, 51, 0.12)',
      }}
    >
      <Stack
        direction={{ xs: 'column', md: 'row' }}
        spacing={1.5}
        justifyContent="space-between"
        alignItems={{ xs: 'flex-start', md: 'center' }}
        sx={{
          position: 'absolute',
          inset: 20,
          bottom: 'auto',
          zIndex: 5,
          pointerEvents: 'none',
        }}
      >
        <Stack
          spacing={0.6}
          sx={{
            p: 1.5,
            borderRadius: 4,
            maxWidth: 520,
            bgcolor: alpha('#081120', 0.72),
            color: '#ffffff',
            backdropFilter: 'blur(16px)',
            boxShadow: '0 18px 48px rgba(8, 17, 32, 0.22)',
          }}
        >
          <Typography variant="overline" sx={{ color: alpha('#ffffff', 0.78), letterSpacing: '0.16em' }}>
            主画布
          </Typography>
          <Typography variant="h6">{selectedTemplateName}</Typography>
          <Typography variant="body2" sx={{ color: alpha('#ffffff', 0.84) }}>
            拖拽、缩放与连线操作集中在此完成
          </Typography>
        </Stack>

        <Stack
          direction={{ xs: 'column', sm: 'row' }}
          spacing={1}
          useFlexGap
          sx={{ pointerEvents: 'auto' }}
        >
          {selectionKind === 'idle' ? (
            <Button variant="contained" color="primary" onClick={onOpenCreateMenu}>
              在空白画布创建节点
            </Button>
          ) : null}
          {selectionKind === 'node' ? (
            <Button variant="contained" color="primary" onClick={onOpenAppendMenu}>
              为当前节点追加下游节点
            </Button>
          ) : null}
          <Chip
            size="small"
            color={selectedNodeId ? 'primary' : 'default'}
            variant={selectedNodeId ? 'filled' : 'outlined'}
            label={selectedNodeLabel ? `节点: ${selectedNodeLabel}` : '当前未选择节点'}
          />
          <Chip
            size="small"
            color={selectedEdgeId ? 'secondary' : 'default'}
            variant={selectedEdgeId ? 'filled' : 'outlined'}
            label={selectedEdgeLabel ? `连线: ${selectedEdgeLabel}` : '当前未选择连线'}
          />
        </Stack>
      </Stack>

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
              mt: 11,
              p: 1.5,
              borderRadius: 4,
              bgcolor: alpha('#ffffff', 0.92),
              border: `1px solid ${alpha('#15304f', 0.1)}`,
              boxShadow: '0 12px 30px rgba(20, 32, 51, 0.08)',
              maxWidth: 340,
            }}
          >
            <Typography variant="subtitle2">当前模板</Typography>
            <Typography variant="body2" color="text.secondary">
              {selectedTemplateName}
            </Typography>
            <Divider />
            <Typography variant="body2" color="text.secondary">
              {selectionKind === 'edge'
                ? '当前正在查看连线条件与去向。'
                : selectionKind === 'node'
                  ? '已锁定当前节点，可直接在右侧修改配置。'
                  : '点击节点或连线后，右侧会同步显示对应配置。'}
            </Typography>
            <Chip size="small" label="拖拽节点可调整布局，拖出连线即可建立连接" />
          </Stack>
        </Panel>

        {selectionKind !== 'idle' ? (
          <Panel position="top-center">
            <Box sx={{ mt: 11 }}>
              <WorkflowCanvasQuickActions
                selectionKind={selectionKind}
                focusLabel={selectedNodeLabel ?? selectedEdgeLabel ?? '当前选择'}
                canDelete
                canDuplicate={selectionKind === 'node'}
                canSetEntryNode={selectionKind === 'node'}
                onDelete={onDeleteSelection}
                onDuplicate={onDuplicateNode}
                onSetEntryNode={onSetEntryNode}
              />
            </Box>
          </Panel>
        ) : null}

        {createMenuMode ? (
          <Panel position="top-right">
            <Box sx={{ mt: 11 }}>
              <WorkflowNodeCreateMenu mode={createMenuMode} onClose={onCloseCreateMenu} onSelectType={onCreateNode} />
            </Box>
          </Panel>
        ) : null}
      </ReactFlow>
    </Box>
  );
}
