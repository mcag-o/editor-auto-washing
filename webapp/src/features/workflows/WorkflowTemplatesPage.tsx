import { useMemo, useState } from 'react';
import AutoFixHighRoundedIcon from '@mui/icons-material/AutoFixHighRounded';
import FitScreenRoundedIcon from '@mui/icons-material/FitScreenRounded';
import Button from '@mui/material/Button';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { alpha } from '@mui/material/styles';
import {
  MarkerType,
  Position,
  addEdge,
  applyEdgeChanges,
  applyNodeChanges,
  type Edge,
  type EdgeChange,
  type Connection,
  type Node,
  type NodeChange,
} from 'reactflow';
import PageToolbar from '../../components/PageToolbar';
import StatusChip from '../../components/StatusChip';
import WorkflowEdgePanel, { type WorkflowEdgeSummary } from './components/WorkflowEdgePanel';
import WorkflowGraphPanel, { type WorkflowCanvasNodeData } from './components/WorkflowGraphPanel';
import WorkflowListPanel, { type WorkflowTemplateSummary } from './components/WorkflowListPanel';
import WorkflowNodeDrawer, { type WorkflowNodeFormValue, type WorkflowNodeType } from './components/WorkflowNodeDrawer';
import WorkflowToolbar from './components/WorkflowToolbar';

type WorkflowTemplate = WorkflowTemplateSummary & {
  entryNodeId: string | null;
  edges: Edge[];
  nodes: Array<Node<WorkflowCanvasNodeData>>;
};

function createLocalId(prefix: string) {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return `${prefix}-${crypto.randomUUID()}`;
  }

  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

const typeLabelMap: Record<WorkflowNodeType, string> = {
  input: '导入节点',
  rewrite: '改写节点',
  review: '审核节点',
  render: '渲染节点',
};

function buildNodeLabel(data: WorkflowCanvasNodeData) {
  return `${data.label}\n${typeLabelMap[data.type]}`;
}

function createNode(id: string, label: string, type: WorkflowNodeType, x: number, y: number, isEntry = false): Node<WorkflowCanvasNodeData> {
  const data: WorkflowCanvasNodeData = {
    label,
    type,
    template: type === 'rewrite' ? 'rewrite.standard' : '',
    model: type === 'rewrite' ? 'gpt-4.1-mini' : '',
    context: type === 'input' ? '接收浏览器上传或粘贴的文章原文。' : '',
    isEntry,
  };

  return {
    id,
    type: 'default',
    position: { x, y },
    data,
    sourcePosition: Position.Right,
    targetPosition: Position.Left,
    style: {
      minWidth: 176,
      borderRadius: 18,
      border: `1px solid ${alpha('#15304f', 0.12)}`,
      padding: 14,
      background: isEntry ? 'linear-gradient(135deg, #0f62fe 0%, #5b3df5 100%)' : '#ffffff',
      color: isEntry ? '#ffffff' : '#142033',
      boxShadow: '0 18px 40px rgba(20, 32, 51, 0.10)',
      whiteSpace: 'pre-line',
      fontWeight: 600,
      lineHeight: 1.5,
    },
    dragHandle: '.react-flow__node-default',
  };
}

const initialTemplates: WorkflowTemplate[] = [
  {
    id: 'workflow-standard',
    name: '标准改写流程',
    description: '从导入到改写再到渲染输出的主链路模板。',
    updatedAt: '今天 22:10',
    entryNodeId: 'node-input',
    nodes: [
      createNode('node-input', '文章导入', 'input', 40, 220, true),
      createNode('node-rewrite', '自动改写', 'rewrite', 320, 220),
      createNode('node-render', '草稿渲染', 'render', 620, 220),
    ],
    edges: [
      {
        id: 'edge-input-rewrite',
        source: 'node-input',
        target: 'node-rewrite',
        markerEnd: { type: MarkerType.ArrowClosed, color: '#0f62fe' },
        style: { stroke: '#0f62fe', strokeWidth: 2 },
      },
      {
        id: 'edge-rewrite-render',
        source: 'node-rewrite',
        target: 'node-render',
        markerEnd: { type: MarkerType.ArrowClosed, color: '#0f62fe' },
        style: { stroke: '#0f62fe', strokeWidth: 2 },
      },
    ],
    nodeCount: 3,
  },
  {
    id: 'workflow-review',
    name: '带人工复核流程',
    description: '在自动改写后增加人工审核节点，适用于高风险主题。',
    updatedAt: '今天 18:30',
    entryNodeId: 'node-review-input',
    nodes: [
      createNode('node-review-input', '导入文章', 'input', 20, 220, true),
      createNode('node-review-rewrite', '改写初稿', 'rewrite', 280, 120),
      createNode('node-review-human', '人工复核', 'review', 280, 320),
      createNode('node-review-render', '输出渲染', 'render', 560, 220),
    ],
    edges: [
      {
        id: 'edge-review-1',
        source: 'node-review-input',
        target: 'node-review-rewrite',
        markerEnd: { type: MarkerType.ArrowClosed, color: '#0f62fe' },
        style: { stroke: '#0f62fe', strokeWidth: 2 },
      },
      {
        id: 'edge-review-2',
        source: 'node-review-rewrite',
        target: 'node-review-human',
        markerEnd: { type: MarkerType.ArrowClosed, color: '#0f62fe' },
        style: { stroke: '#0f62fe', strokeWidth: 2 },
      },
      {
        id: 'edge-review-3',
        source: 'node-review-human',
        target: 'node-review-render',
        markerEnd: { type: MarkerType.ArrowClosed, color: '#0f62fe' },
        style: { stroke: '#0f62fe', strokeWidth: 2 },
      },
    ],
    nodeCount: 4,
  },
];

function syncNodePresentation(nodes: Array<Node<WorkflowCanvasNodeData>>, entryNodeId: string | null) {
  return nodes.map((node) => {
    const isEntry = node.id === entryNodeId;
    return {
      ...node,
      data: {
        ...node.data,
        isEntry,
      },
      style: {
        ...node.style,
        background: isEntry ? 'linear-gradient(135deg, #0f62fe 0%, #5b3df5 100%)' : '#ffffff',
        color: isEntry ? '#ffffff' : '#142033',
      },
    };
  });
}

function renderNodes(
  nodes: Array<Node<WorkflowCanvasNodeData>>,
  entryNodeId: string | null,
  selectedNodeId: string | null,
) {
  return syncNodePresentation(nodes, entryNodeId).map((node) => ({
    ...node,
    data: {
      ...node.data,
      isEntry: node.id === entryNodeId,
    },
    style: {
      ...node.style,
      whiteSpace: 'pre-line',
      outline: node.id === selectedNodeId ? '3px solid rgba(91, 61, 245, 0.24)' : 'none',
    },
    selected: node.id === selectedNodeId,
  }));
}

export default function WorkflowTemplatesPage() {
  const [templates, setTemplates] = useState<WorkflowTemplate[]>(initialTemplates);
  const [selectedTemplateId, setSelectedTemplateId] = useState(initialTemplates[0].id);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(initialTemplates[0].entryNodeId);
  const [selectedEdgeId, setSelectedEdgeId] = useState<string | null>(null);

  const selectedTemplate = useMemo(
    () => templates.find((template) => template.id === selectedTemplateId) ?? templates[0],
    [selectedTemplateId, templates],
  );

  const selectedNode = selectedTemplate.nodes.find((node) => node.id === selectedNodeId) ?? null;
  const selectedEdge = selectedTemplate.edges.find((edge) => edge.id === selectedEdgeId) ?? null;

  const selectedNodeFormValue: WorkflowNodeFormValue | null = selectedNode
    ? {
        label: selectedNode.data.label,
        type: selectedNode.data.type,
        template: selectedNode.data.template,
        model: selectedNode.data.model,
        context: selectedNode.data.context,
      }
    : null;

  const workflowItems: WorkflowTemplateSummary[] = templates.map((template) => ({
    id: template.id,
    name: template.name,
    description: template.description,
    nodeCount: template.nodes.length,
    updatedAt: template.updatedAt,
  }));

  const selectedEdgeSummary: WorkflowEdgeSummary | null = selectedEdge
    ? {
        id: selectedEdge.id,
        sourceLabel: selectedTemplate.nodes.find((node) => node.id === selectedEdge.source)?.data.label ?? selectedEdge.source,
        targetLabel: selectedTemplate.nodes.find((node) => node.id === selectedEdge.target)?.data.label ?? selectedEdge.target,
      }
    : null;

  const updateSelectedTemplate = (updater: (template: WorkflowTemplate) => WorkflowTemplate) => {
    setTemplates((currentTemplates) =>
      currentTemplates.map((template) => {
        if (template.id !== selectedTemplateId) {
          return template;
        }
        const nextTemplate = updater(template);
        return {
          ...nextTemplate,
          nodeCount: nextTemplate.nodes.length,
        };
      }),
    );
  };

  const handleCreateTemplate = () => {
    const nextLabelIndex = templates.length + 1;
    const nextId = createLocalId('workflow-local');
    const seedNodeId = createLocalId('node-local');
    const newTemplate: WorkflowTemplate = {
      id: nextId,
      name: `本地模板 ${nextLabelIndex}`,
      description: '用于验证本地节点编排与侧边配置交互。',
      updatedAt: '刚刚',
      entryNodeId: seedNodeId,
      nodes: [createNode(seedNodeId, '起始节点', 'input', 120, 220, true)],
      edges: [],
      nodeCount: 1,
    };

    setTemplates((current) => [...current, newTemplate]);
    setSelectedTemplateId(nextId);
    setSelectedNodeId(seedNodeId);
    setSelectedEdgeId(null);
  };

  const handleSelectTemplate = (templateId: string) => {
    const nextTemplate = templates.find((template) => template.id === templateId);
    if (!nextTemplate) {
      return;
    }

    setSelectedTemplateId(templateId);
    setSelectedNodeId(nextTemplate.entryNodeId ?? nextTemplate.nodes[0]?.id ?? null);
    setSelectedEdgeId(null);
  };

  const handleAddNode = () => {
    const nextIndex = selectedTemplate.nodes.length + 1;
    const nextNodeId = createLocalId(`${selectedTemplate.id}-node`);
    const newNode = createNode(
      nextNodeId,
      `新节点 ${nextIndex}`,
      'rewrite',
      120 + (nextIndex % 3) * 220,
      120 + Math.floor(nextIndex / 3) * 180,
    );

    updateSelectedTemplate((template) => ({
      ...template,
      updatedAt: '刚刚',
      nodes: [...template.nodes, newNode],
    }));
    setSelectedNodeId(nextNodeId);
    setSelectedEdgeId(null);
  };

  const handleDeleteNode = () => {
    if (!selectedNodeId) {
      return;
    }

    updateSelectedTemplate((template) => {
      const nextNodes = template.nodes.filter((node) => node.id !== selectedNodeId);
      const nextEdges = template.edges.filter((edge) => edge.source !== selectedNodeId && edge.target !== selectedNodeId);
      const nextEntryNodeId = template.entryNodeId === selectedNodeId ? nextNodes[0]?.id ?? null : template.entryNodeId;
      return {
        ...template,
        updatedAt: '刚刚',
        entryNodeId: nextEntryNodeId,
        nodes: syncNodePresentation(nextNodes, nextEntryNodeId),
        edges: nextEdges,
      };
    });

    const fallbackNodeId = selectedTemplate.nodes.find((node) => node.id !== selectedNodeId)?.id ?? null;
    setSelectedNodeId(fallbackNodeId);
    setSelectedEdgeId(null);
  };

  const handleSelectEntryNode = () => {
    if (!selectedNodeId) {
      return;
    }

    updateSelectedTemplate((template) => ({
      ...template,
      updatedAt: '刚刚',
      entryNodeId: selectedNodeId,
      nodes: syncNodePresentation(template.nodes, selectedNodeId),
    }));
  };

  const handleNodeChange = <K extends keyof WorkflowNodeFormValue>(field: K, value: WorkflowNodeFormValue[K]) => {
    if (!selectedNodeId) {
      return;
    }

    updateSelectedTemplate((template) => ({
      ...template,
      updatedAt: '刚刚',
      nodes: template.nodes.map((node) =>
        node.id === selectedNodeId
          ? {
              ...node,
              data: {
                ...node.data,
                [field]: value,
              } as WorkflowCanvasNodeData,
            }
          : node,
      ),
    }));
  };

  const graphNodes = useMemo(
    () =>
      renderNodes(selectedTemplate.nodes, selectedTemplate.entryNodeId, selectedNodeId).map((node) => ({
        ...node,
        data: {
          ...node.data,
          label: buildNodeLabel(node.data),
        },
      })),
    [selectedNodeId, selectedTemplate.entryNodeId, selectedTemplate.nodes],
  );

  const handleNodesChange = (changes: NodeChange[]) => {
    updateSelectedTemplate((template) => ({
      ...template,
      updatedAt: '刚刚',
      nodes: applyNodeChanges(changes, template.nodes),
    }));
  };

  const handleEdgesChange = (changes: EdgeChange[]) => {
    updateSelectedTemplate((template) => ({
      ...template,
      updatedAt: '刚刚',
      edges: applyEdgeChanges(changes, template.edges),
    }));
  };

  const handleConnect = ({ source, target }: Connection) => {
    if (!source || !target || source === target) {
      return;
    }

    updateSelectedTemplate((template) => {
      const duplicate = template.edges.some((edge) => edge.source === source && edge.target === target);
      if (duplicate) {
        return template;
      }

      return {
        ...template,
        updatedAt: '刚刚',
        edges: addEdge(
          {
            id: `edge-${source}-${target}`,
            source,
            target,
            markerEnd: { type: MarkerType.ArrowClosed, color: '#0f62fe' },
            style: { stroke: '#0f62fe', strokeWidth: 2 },
          },
          template.edges,
        ),
      };
    });
  };

  const handleDeleteEdge = () => {
    if (!selectedEdgeId) {
      return;
    }

    updateSelectedTemplate((template) => ({
      ...template,
      updatedAt: '刚刚',
      edges: template.edges.filter((edge) => edge.id !== selectedEdgeId),
    }));
    setSelectedEdgeId(null);
  };

  const entryNodeLabel = selectedTemplate.nodes.find((node) => node.id === selectedTemplate.entryNodeId)?.data.label ?? null;

  return (
    <Stack spacing={3}>
      <PageToolbar
        title="工作流模板"
        description="基于 React Flow 的模板图编辑壳层，先完成本地节点、连线和侧边配置交互，再在后续任务接入真实保存与运行接口。"
        leading={<StatusChip status="active" label="React Flow 编辑器壳层" />}
        actions={
          <>
            <Button variant="outlined" startIcon={<FitScreenRoundedIcon />}>
              画布预留
            </Button>
            <Button variant="contained" startIcon={<AutoFixHighRoundedIcon />}>
              本地编辑中
            </Button>
          </>
        }
        filters={
          <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.25} alignItems={{ xs: 'flex-start', md: 'center' }}>
            <StatusChip status="completed" label="支持新增/删除节点" />
            <StatusChip status="completed" label="支持连接/断开连线" />
            <StatusChip status="pending" label="未接入真实 API" />
            <Typography variant="body2" color="text.secondary">
              当前选中模板：{selectedTemplate.name}
            </Typography>
          </Stack>
        }
      />

      <WorkflowToolbar
        nodeCount={selectedTemplate.nodes.length}
        edgeCount={selectedTemplate.edges.length}
        hasSelection={Boolean(selectedNodeId)}
        canDeleteNode={selectedTemplate.nodes.length > 0 && Boolean(selectedNodeId)}
        onAddNode={handleAddNode}
        onDeleteNode={handleDeleteNode}
        onSelectEntryNode={handleSelectEntryNode}
      />

      <Stack direction={{ xs: 'column', xl: 'row' }} spacing={3} alignItems="stretch">
        <Stack spacing={3} sx={{ width: { xs: '100%', xl: 340 }, flexShrink: 0 }}>
          <WorkflowListPanel
            items={workflowItems}
            selectedId={selectedTemplate.id}
            onCreateTemplate={handleCreateTemplate}
            onSelectTemplate={handleSelectTemplate}
          />
          <WorkflowEdgePanel selectedEdge={selectedEdgeSummary} onDeleteEdge={handleDeleteEdge} />
        </Stack>

        <Stack spacing={3} flex={1} minWidth={0}>
          <WorkflowGraphPanel
            nodes={graphNodes}
            edges={selectedTemplate.edges}
            selectedNodeId={selectedNodeId}
            selectedTemplateName={selectedTemplate.name}
            onNodesChange={handleNodesChange}
            onEdgesChange={handleEdgesChange}
            onConnect={handleConnect}
            onClearSelection={() => {
              setSelectedNodeId(null);
              setSelectedEdgeId(null);
            }}
            onNodeClick={(node) => {
              setSelectedNodeId(node?.id ?? null);
              setSelectedEdgeId(null);
            }}
            onEdgeClick={(edge) => {
              setSelectedEdgeId(edge.id);
              setSelectedNodeId(null);
            }}
          />
        </Stack>

        <Stack spacing={3} sx={{ width: { xs: '100%', xl: 360 }, flexShrink: 0 }}>
          <WorkflowNodeDrawer
            selectedNodeId={selectedNodeId}
            entryNodeLabel={entryNodeLabel}
            value={selectedNodeFormValue}
            onChange={handleNodeChange}
          />
        </Stack>
      </Stack>
    </Stack>
  );
}
