import { useEffect, useMemo, useState } from 'react';
import AutoFixHighRoundedIcon from '@mui/icons-material/AutoFixHighRounded';
import ChevronLeftRoundedIcon from '@mui/icons-material/ChevronLeftRounded';
import ChevronRightRoundedIcon from '@mui/icons-material/ChevronRightRounded';
import FitScreenRoundedIcon from '@mui/icons-material/FitScreenRounded';
import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import IconButton from '@mui/material/IconButton';
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
import { ApiError, createWorkflow, deleteWorkflow, listWorkflows, updateWorkflow } from '../../lib/api/client';
import {
  buildWorkflowEdgeId,
  getWorkflowNodeDisplayType,
  mapApiWorkflowToForm,
  mapWorkflowFormToApi,
  reconcileCreatedWorkflow,
  type WorkflowEdgeData,
  type WorkflowFormTemplate,
} from '../../lib/mappers/workflow';
import PageToolbar from '../../components/PageToolbar';
import PageState from '../../components/PageState';
import StatusChip from '../../components/StatusChip';
import WorkflowEdgePanel, { type WorkflowEdgeSummary } from './components/WorkflowEdgePanel';
import WorkflowGraphPanel, { type WorkflowCanvasNodeData } from './components/WorkflowGraphPanel';
import WorkflowListPanel, { type WorkflowTemplateSummary } from './components/WorkflowListPanel';
import WorkflowNodeDrawer, { commonWorkflowNodeTypes, type WorkflowNodeFormValue, type WorkflowNodeType } from './components/WorkflowNodeDrawer';
import WorkflowToolbar from './components/WorkflowToolbar';

type WorkflowTemplate = WorkflowFormTemplate & { updatedAtLabel: string };

function createLocalId(prefix: string) {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return `${prefix}-${crypto.randomUUID()}`;
  }

  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

export function createLocalWorkflowEdge(input: {
  source: string;
  target: string;
  condition?: string;
  priority: number;
}): Edge<WorkflowEdgeData> {
  const condition = input.condition ?? 'always';
  const uniqueToken = createLocalId('edge-local');

  return {
    id: `${buildWorkflowEdgeId({
      source: input.source,
      target: input.target,
      condition,
      priority: input.priority,
    })}-${uniqueToken}`,
    source: input.source,
    target: input.target,
    label: condition,
    data: {
      priority: input.priority,
    },
    markerEnd: { type: MarkerType.ArrowClosed, color: '#0f62fe' },
    style: { stroke: '#0f62fe', strokeWidth: 2 },
  };
}

const typeLabelMap: Record<WorkflowNodeType, string> = {
  input: '导入节点',
  rewrite: '改写节点',
  review: '审核节点',
  render: '渲染节点',
};

function buildNodeLabel(data: WorkflowCanvasNodeData) {
  const displayTypeLabel = data.rawType && !(data.rawType in typeLabelMap) ? `${data.rawType}（保留）` : typeLabelMap[data.type];
  return `${data.label}\n${displayTypeLabel}`;
}

function createNode(id: string, label: string, type: WorkflowNodeType, x: number, y: number, isEntry = false, template = '', model = '', context = '', rawType?: string): Node<WorkflowCanvasNodeData> {
  const data: WorkflowCanvasNodeData = {
    label,
    type,
    template,
    model,
    context,
    isEntry,
    rawType: rawType || type,
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

function renderNodes(nodes: Array<Node<WorkflowCanvasNodeData>>, entryNodeId: string | null, selectedNodeId: string | null) {
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

function renderEdges(edges: Array<Edge<WorkflowEdgeData>>, selectedEdgeId: string | null) {
  return edges.map((edge) => ({
    ...edge,
    markerEnd: {
      type: MarkerType.ArrowClosed,
      color: edge.id === selectedEdgeId ? '#5b3df5' : '#0f62fe',
    },
    style: {
      stroke: edge.id === selectedEdgeId ? '#5b3df5' : '#0f62fe',
      strokeWidth: edge.id === selectedEdgeId ? 4 : 2,
    },
    selected: edge.id === selectedEdgeId,
  }));
}

function workflowNodeType(type: string): WorkflowNodeType {
  if (commonWorkflowNodeTypes.includes(type as WorkflowNodeType)) {
    return type as WorkflowNodeType;
  }
  return 'rewrite';
}

function decorateWorkflowTemplate(template: WorkflowFormTemplate): WorkflowTemplate {
  return {
    ...template,
    updatedAtLabel: template.updatedAt ? new Date(template.updatedAt).toLocaleString('zh-CN', { hour12: false }) : '未记录',
    nodes: template.nodes.map((node) =>
      createNode(
        node.id,
        node.data.label,
        getWorkflowNodeDisplayType(node.data.rawType || node.data.type),
        node.position.x,
        node.position.y,
        node.id === template.entryNodeId,
        node.data.template,
        node.data.model,
        node.data.context,
        node.data.rawType,
      ),
    ),
    edges: template.edges.map((edge) => ({
      ...edge,
      markerEnd: { type: MarkerType.ArrowClosed, color: '#0f62fe' },
      style: { stroke: '#0f62fe', strokeWidth: 2 },
    })),
  };
}

export default function WorkflowTemplatesPage() {
  const [templates, setTemplates] = useState<WorkflowTemplate[]>([]);
  const [selectedTemplateId, setSelectedTemplateId] = useState('');
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [selectedEdgeId, setSelectedEdgeId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [fitViewRequest, setFitViewRequest] = useState(0);
  const [isRightPanelCollapsed, setIsRightPanelCollapsed] = useState(false);

  const loadWorkflows = async () => {
    setLoading(true);
    setLoadError(null);

    try {
      const items = await listWorkflows();
      const mapped = items.map(mapApiWorkflowToForm).map(decorateWorkflowTemplate);
      setTemplates(mapped);
      const nextId = selectedTemplateId || mapped[0]?.id || '';
      setSelectedTemplateId(nextId);
      const selected = mapped.find((template) => template.id === nextId) ?? mapped[0];
      setSelectedNodeId(selected?.entryNodeId ?? selected?.nodes[0]?.id ?? null);
      setSelectedEdgeId(null);
    } catch (apiError) {
      setLoadError(apiError instanceof ApiError ? apiError.message : '工作流列表加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadWorkflows();
  }, []);

  const selectedTemplate = useMemo(
    () => templates.find((template) => template.id === selectedTemplateId) ?? templates[0],
    [selectedTemplateId, templates],
  );

  const selectedNode = selectedTemplate?.nodes.find((node) => node.id === selectedNodeId) ?? null;
  const selectedEdge = selectedTemplate?.edges.find((edge) => edge.id === selectedEdgeId) ?? null;
  const selectedNodeLabel = selectedNode?.data.label ?? null;
  const selectedEdgeLabel = selectedEdge && selectedTemplate
    ? `${selectedTemplate.nodes.find((node) => node.id === selectedEdge.source)?.data.label ?? selectedEdge.source} -> ${selectedTemplate.nodes.find((node) => node.id === selectedEdge.target)?.data.label ?? selectedEdge.target}`
    : null;

  const selectedNodeFormValue: WorkflowNodeFormValue | null = selectedNode
      ? {
          label: selectedNode.data.label,
          type: selectedNode.data.type,
          rawType: selectedNode.data.rawType || selectedNode.data.type,
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
    updatedAt: template.updatedAtLabel,
  }));

  const selectedEdgeSummary: WorkflowEdgeSummary | null = selectedEdge && selectedTemplate
    ? {
        id: selectedEdge.id,
        sourceLabel: selectedTemplate.nodes.find((node) => node.id === selectedEdge.source)?.data.label ?? selectedEdge.source,
        targetLabel: selectedTemplate.nodes.find((node) => node.id === selectedEdge.target)?.data.label ?? selectedEdge.target,
        condition: selectedEdge.label ? String(selectedEdge.label) : 'always',
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
    const newTemplate = decorateWorkflowTemplate({
      id: nextId,
      name: `工作流模板 ${nextLabelIndex}`,
      description: '补充工作流用途、入口条件和关键处理节点。',
      version: `v${nextLabelIndex}.0.0`,
      enabled: true,
      updatedBy: 'react-webapp',
      updatedAt: new Date().toISOString(),
      entryNodeId: seedNodeId,
      nodes: [createNode(seedNodeId, '起始节点', 'input', 120, 220, true)],
      edges: [],
      nodeCount: 1,
    });

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
    if (!selectedTemplate) {
      return;
    }
    const nextIndex = selectedTemplate.nodes.length + 1;
    const nextNodeId = createLocalId(`${selectedTemplate.id}-node`);
    const newNode = createNode(nextNodeId, `新节点 ${nextIndex}`, 'rewrite', 120 + (nextIndex % 3) * 220, 120 + Math.floor(nextIndex / 3) * 180);

    updateSelectedTemplate((template) => ({
      ...template,
      updatedAt: '刚刚',
      nodes: [...template.nodes, newNode],
    }));
    setSelectedNodeId(nextNodeId);
    setSelectedEdgeId(null);
  };

  const handleDeleteNode = () => {
    if (!selectedNodeId || !selectedTemplate) {
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
                ...mapNodeFieldChange(node.data, field, value),
              } as WorkflowCanvasNodeData,
            }
          : node,
      ),
    }));
  };

  function mapNodeFieldChange<K extends keyof WorkflowNodeFormValue>(
    current: WorkflowCanvasNodeData,
    field: K,
    value: WorkflowNodeFormValue[K],
  ): Partial<WorkflowCanvasNodeData> {
    if (field === 'rawType') {
      const rawType = String(value).trim();
      return {
        rawType,
        type: workflowNodeType(rawType),
      };
    }

    return {
      [field]: value,
    } as Partial<WorkflowCanvasNodeData>;
  }

  const graphNodes = useMemo(
    () =>
      selectedTemplate
        ? renderNodes(selectedTemplate.nodes, selectedTemplate.entryNodeId, selectedNodeId).map((node) => ({
            ...node,
            data: {
              ...node.data,
              label: buildNodeLabel(node.data),
            },
          }))
        : [],
    [selectedNodeId, selectedTemplate],
  );

  const graphEdges = useMemo(
    () => renderEdges(selectedTemplate?.edges ?? [], selectedEdgeId),
    [selectedEdgeId, selectedTemplate],
  );

  const handleFitView = () => {
    setFitViewRequest((current) => current + 1);
  };

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
      const nextPriority = template.edges.reduce((maxPriority, edge) => Math.max(maxPriority, edge.data?.priority ?? -1), -1) + 1;
      const defaultCondition = 'always';

      return {
        ...template,
        updatedAt: '刚刚',
        edges: addEdge(
          createLocalWorkflowEdge({
            source,
            target,
            condition: defaultCondition,
            priority: nextPriority,
          }),
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

  const handleSaveTemplate = async () => {
    if (!selectedTemplate) {
      return;
    }

    const hadTemplatesBeforeSave = templates.some((template) => !template.id.startsWith('workflow-local'));
    setSaving(true);
    setActionError(null);
    setSuccessMessage(null);

    try {
      const temporaryTemplateId = selectedTemplate.id;
      const payload = mapWorkflowFormToApi({
        ...selectedTemplate,
        updatedBy: 'react-webapp',
      });
      const saved = temporaryTemplateId.startsWith('workflow-local')
        ? await createWorkflow(payload)
        : await updateWorkflow(temporaryTemplateId, payload);
      const mapped = decorateWorkflowTemplate(mapApiWorkflowToForm(saved));

      if (temporaryTemplateId.startsWith('workflow-local')) {
        const reconciled = reconcileCreatedWorkflow({
          created: mapped,
          selectedTemplateId: temporaryTemplateId,
          templates,
          temporaryTemplateId,
        });
        setTemplates(reconciled.templates);
        setSelectedTemplateId(reconciled.selectedTemplateId);
      } else {
        setTemplates((current) => current.map((template) => (template.id === temporaryTemplateId ? mapped : template)));
        setSelectedTemplateId(mapped.id);
      }

      setSelectedNodeId(mapped.entryNodeId ?? mapped.nodes[0]?.id ?? null);
      setSelectedEdgeId(null);
      setSuccessMessage(temporaryTemplateId.startsWith('workflow-local') ? '工作流模板已创建。' : '工作流模板已保存。');
    } catch (apiError) {
      if (selectedTemplate.id.startsWith('workflow-local') && !hadTemplatesBeforeSave) {
        setTemplates((current) => current.filter((template) => template.id !== selectedTemplate.id));
        setSelectedTemplateId('');
        setSelectedNodeId(null);
        setSelectedEdgeId(null);
      }
      setActionError(apiError instanceof ApiError ? apiError.message : '工作流保存失败');
    } finally {
      setSaving(false);
    }
  };

  const handleDeleteTemplate = async () => {
    if (!selectedTemplate) {
      return;
    }

    if (selectedTemplate.id.startsWith('workflow-local')) {
      setTemplates((current) => current.filter((template) => template.id !== selectedTemplate.id));
      const fallback = templates.find((template) => template.id !== selectedTemplate.id);
      setSelectedTemplateId(fallback?.id ?? '');
      setSelectedNodeId(fallback?.entryNodeId ?? fallback?.nodes[0]?.id ?? null);
      setSelectedEdgeId(null);
      return;
    }

    setSaving(true);
    setActionError(null);
    setSuccessMessage(null);

    try {
      await deleteWorkflow(selectedTemplate.id);
      const remaining = templates.filter((template) => template.id !== selectedTemplate.id);
      const fallback = remaining[0];
      setTemplates(remaining);
      setSelectedTemplateId(fallback?.id ?? '');
      setSelectedNodeId(fallback?.entryNodeId ?? fallback?.nodes[0]?.id ?? null);
      setSelectedEdgeId(null);
      setSuccessMessage('工作流模板已删除。');
    } catch (apiError) {
      setActionError(apiError instanceof ApiError ? apiError.message : '工作流删除失败');
    } finally {
      setSaving(false);
    }
  };

  const entryNodeLabel = selectedTemplate?.nodes.find((node) => node.id === selectedTemplate.entryNodeId)?.data.label ?? null;

  return (
    <Stack spacing={3}>
      <PageToolbar
        title="工作流模板"
        description="使用工作流列表与保存接口维护流程定义，并在画布中编辑节点与连线。"
        leading={<StatusChip status="active" label="React Flow 编辑器" />}
        actions={
          <>
            <Button color="inherit" variant="text" startIcon={<FitScreenRoundedIcon />} onClick={() => void loadWorkflows()} disabled={loading || saving}>
              刷新画布
            </Button>
            <Button variant="contained" startIcon={<AutoFixHighRoundedIcon />} onClick={() => void handleSaveTemplate()} disabled={!selectedTemplate || saving}>
              {saving ? '保存中' : '保存模板'}
            </Button>
          </>
        }
        filters={
          <Stack direction={{ xs: 'column', md: 'row' }} spacing={1} alignItems={{ xs: 'flex-start', md: 'center' }}>
            <StatusChip status="completed" label="节点编辑" />
            <StatusChip status="completed" label="连线编辑" />
            <StatusChip status="completed" label="工作流能力已接入" />
            <Typography variant="body2" color="text.secondary">
              当前选中模板：{selectedTemplate?.name ?? '未选择'}
            </Typography>
          </Stack>
        }
      />

      {actionError ? <Alert severity="error">{actionError}</Alert> : null}
      {successMessage ? <Alert severity="success">{successMessage}</Alert> : null}

      <WorkflowToolbar
        nodeCount={selectedTemplate?.nodes.length ?? 0}
        edgeCount={selectedTemplate?.edges.length ?? 0}
        canFitView={Boolean(selectedTemplate)}
        canDeleteNode={(selectedTemplate?.nodes.length ?? 0) > 0 && Boolean(selectedNodeId)}
        canSetEntryNode={Boolean(selectedNodeId)}
        selectedNodeLabel={selectedNodeLabel}
        selectedEdgeLabel={selectedEdgeLabel}
        entryNodeLabel={entryNodeLabel}
        onAddNode={handleAddNode}
        onDeleteNode={handleDeleteNode}
        onFitView={handleFitView}
        onSelectEntryNode={handleSelectEntryNode}
      />

      <Stack direction={{ xs: 'column', xl: 'row' }} spacing={3} alignItems="stretch">
        <Stack spacing={3} sx={{ width: { xs: '100%', xl: 340 }, flexShrink: 0 }} data-testid="workflow-list-column">
          <WorkflowListPanel items={workflowItems} loading={loading} error={loadError} selectedId={selectedTemplate?.id ?? ''} onCreateTemplate={handleCreateTemplate} onSelectTemplate={handleSelectTemplate} onDeleteTemplate={() => void handleDeleteTemplate()} />
        </Stack>

        <Stack spacing={3} flex={1} minWidth={0} data-testid="workflow-canvas-column" data-panel-state={isRightPanelCollapsed ? 'collapsed' : 'expanded'}>
          <WorkflowGraphPanel
            nodes={graphNodes}
            edges={graphEdges}
            fitViewRequest={fitViewRequest}
            selectedEdgeId={selectedEdgeId}
            selectedEdgeLabel={selectedEdgeLabel}
            selectedNodeId={selectedNodeId}
            selectedNodeLabel={selectedNodeLabel}
            selectedTemplateName={selectedTemplate?.name ?? (loading ? '正在加载模板' : '未选择模板')}
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

        <Stack
          spacing={2}
          data-testid="workflow-side-panel"
          data-panel-state={isRightPanelCollapsed ? 'collapsed' : 'expanded'}
          sx={{
            width: {
              xs: '100%',
              xl: isRightPanelCollapsed ? 72 : 360,
            },
            flexShrink: 0,
            transition: (theme) => theme.transitions.create(['width'], {
              duration: theme.transitions.duration.shorter,
            }),
          }}
        >
          <Stack direction="row" justifyContent={isRightPanelCollapsed ? 'center' : 'space-between'} alignItems="center">
            {isRightPanelCollapsed ? null : (
              <Stack spacing={0.5}>
                <Typography variant="subtitle2">右侧配置面板</Typography>
                <Typography variant="body2" color="text.secondary">
                  按节点或连线查看与编辑当前配置，折叠后为画布释放更多宽度。
                </Typography>
              </Stack>
            )}
            <IconButton
              aria-label={isRightPanelCollapsed ? '展开右侧配置面板' : '折叠右侧配置面板'}
              onClick={() => setIsRightPanelCollapsed((current) => !current)}
              size="small"
            >
              {isRightPanelCollapsed ? <ChevronLeftRoundedIcon /> : <ChevronRightRoundedIcon />}
            </IconButton>
          </Stack>

          {isRightPanelCollapsed ? null : selectedEdgeSummary ? (
            <WorkflowEdgePanel selectedEdge={selectedEdgeSummary} onDeleteEdge={handleDeleteEdge} />
          ) : loadError && templates.length === 0 ? (
            <PageState title="工作流编辑器暂时不可用" description={loadError} tone="error" />
          ) : (
            <WorkflowNodeDrawer loading={loading} selectedNodeId={selectedNodeId} entryNodeLabel={entryNodeLabel} value={selectedNodeFormValue} onChange={handleNodeChange} />
          )}
        </Stack>
      </Stack>
    </Stack>
  );
}
