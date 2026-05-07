import { CssBaseline, ThemeProvider } from '@mui/material';
import { act } from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Edge, Node } from 'reactflow';
import WorkflowTemplatesPage, { createLocalWorkflowEdge } from './WorkflowTemplatesPage';
import theme from '../../theme/theme';

const fitViewSpy = vi.fn();

vi.mock('reactflow', async () => {
  const actual = await vi.importActual<typeof import('reactflow')>('reactflow');

  function MockReactFlow({
    nodes,
    edges,
    onNodeClick,
    onEdgeClick,
    onPaneClick,
    children,
  }: {
    nodes: Node[];
    edges: Edge[];
    onNodeClick?: (_event: unknown, node: Node) => void;
    onEdgeClick?: (_event: unknown, edge: Edge) => void;
    onPaneClick?: () => void;
    children?: React.ReactNode;
  }) {
    return (
      <div data-testid="workflow-canvas-shell">
        <button type="button" onClick={() => onPaneClick?.()}>
          清空画布选择
        </button>
        <div data-testid="mock-node-layer">
          {nodes.map((node) => (
            <button
              key={node.id}
              type="button"
              data-testid={`node-${node.id}`}
              aria-pressed={node.selected ? 'true' : 'false'}
              style={node.style as React.CSSProperties | undefined}
              onClick={() => onNodeClick?.({}, node)}
            >
              <span>{String(node.data?.label ?? node.id)}</span>
            </button>
          ))}
        </div>
        <div data-testid="mock-edge-layer">
          {edges.map((edge) => (
            <button
              key={edge.id}
              type="button"
              data-testid={`edge-${edge.id}`}
              aria-pressed={edge.selected ? 'true' : 'false'}
              onClick={() => onEdgeClick?.({}, edge)}
            >
              {`${edge.source}->${edge.target}:${String(edge.label ?? '')}`}
            </button>
          ))}
        </div>
        {children}
      </div>
    );
  }

  function MockControls() {
    return <div data-testid="mock-reactflow-controls" />;
  }

  function passthrough({ children }: { children?: React.ReactNode }) {
    return <div>{children}</div>;
  }

  return {
    ...actual,
    ReactFlow: MockReactFlow,
    Controls: MockControls,
    Background: () => null,
    MiniMap: () => null,
    Panel: passthrough,
    useReactFlow: () => ({ fitView: fitViewSpy }),
  };
});

vi.mock('../../lib/api/client', async () => {
  const actual = await vi.importActual<typeof import('../../lib/api/client')>('../../lib/api/client');

  return {
    ...actual,
    listWorkflows: vi.fn(),
    createWorkflow: vi.fn(),
    updateWorkflow: vi.fn(),
    deleteWorkflow: vi.fn(),
  };
});

type ApiWorkflow = {
  id: string;
  name: string;
  description: string;
  version: string;
  enabled: boolean;
  updated_by: string;
  updated_at: string;
  entry_node_id: string;
  nodes: Array<{
    id: string;
    type: string;
    name: string;
    config_json: string;
  }>;
  edges: Array<{
    from_node_id: string;
    to_node_id: string;
    condition: string;
    priority: number;
  }>;
};

const workflowFixtures: ApiWorkflow[] = [
  {
    id: 'workflow-alpha',
    name: '品牌改写主链路',
    description: '覆盖默认导入、改写和渲染链路。',
    version: 'v2.0.0',
    enabled: true,
    updated_by: '运营编辑组',
    updated_at: '2026-05-07T08:00:00Z',
    entry_node_id: 'node-input',
    nodes: [
      {
        id: 'node-input',
        type: 'input',
        name: '导入文章',
        config_json: JSON.stringify({
          label: '导入文章',
          type: 'input',
          template: '',
          model: '',
          context: '接收操作员上传或粘贴的原文。',
          position: { x: 80, y: 120 },
        }),
      },
      {
        id: 'node-rewrite',
        type: 'rewrite',
        name: '主文改写',
        config_json: JSON.stringify({
          label: '主文改写',
          type: 'rewrite',
          template: 'rewrite.brand.core',
          model: 'gpt-4.1-mini',
          context: '执行品牌表达统一和结构整理。',
          position: { x: 320, y: 120 },
        }),
      },
      {
        id: 'node-render',
        type: 'render',
        name: '渲染成稿',
        config_json: JSON.stringify({
          label: '渲染成稿',
          type: 'render',
          template: 'render.publish.default',
          model: '',
          context: '输出最终草稿。',
          position: { x: 560, y: 120 },
        }),
      },
    ],
    edges: [
      {
        from_node_id: 'node-input',
        to_node_id: 'node-rewrite',
        condition: 'always',
        priority: 0,
      },
      {
        from_node_id: 'node-rewrite',
        to_node_id: 'node-render',
        condition: 'always',
        priority: 1,
      },
    ],
  },
];

function buildSavedWorkflow(entryNodeId = 'node-rewrite'): ApiWorkflow {
  return {
    ...workflowFixtures[0],
    entry_node_id: entryNodeId,
    updated_at: '2026-05-07T09:30:00Z',
  };
}

function renderWorkflowTemplatesPage() {
  return render(
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <WorkflowTemplatesPage />
    </ThemeProvider>,
  );
}

describe('WorkflowTemplatesPage local edge helpers', () => {
  it('creates a new same-pair edge id that does not collide after delete and reconnect', () => {
    const survivingEdge = createLocalWorkflowEdge({
      source: 'node-1',
      target: 'node-2',
      condition: 'always',
      priority: 1,
    });

    const reconnectedEdge = createLocalWorkflowEdge({
      source: 'node-1',
      target: 'node-2',
      condition: 'always',
      priority: 1,
    });

    expect(reconnectedEdge.id).not.toBe(survivingEdge.id);
  });
});

describe('WorkflowTemplatesPage editor interactions', () => {
  beforeEach(async () => {
    fitViewSpy.mockReset();

    const api = await import('../../lib/api/client');
    vi.mocked(api.listWorkflows).mockResolvedValue(workflowFixtures);
    vi.mocked(api.createWorkflow).mockResolvedValue(workflowFixtures[0]);
    vi.mocked(api.updateWorkflow).mockResolvedValue(buildSavedWorkflow());
    vi.mocked(api.deleteWorkflow).mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('highlights selected nodes and selected edges clearly', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    const inputNode = screen.getByTestId('node-node-input');
    const rewriteNode = screen.getByTestId('node-node-rewrite');
    const firstEdge = screen.getByTestId('edge-edge-node-input-node-rewrite-always-0');

    expect(inputNode).toHaveAttribute('aria-pressed', 'true');
    expect(rewriteNode).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByText('当前节点')).toBeInTheDocument();
    expect(screen.getByText('node-input')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(rewriteNode);
    });

    expect(rewriteNode).toHaveAttribute('aria-pressed', 'true');
    expect(inputNode).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByText('node-rewrite')).toBeInTheDocument();
    expect(screen.getByText('入口节点：导入文章')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '设为入口节点' })).toBeEnabled();

    await act(async () => {
      fireEvent.click(firstEdge);
    });

    const leftColumn = screen.getByTestId('workflow-list-column');
    const sidePanel = screen.getByTestId('workflow-side-panel');

    expect(firstEdge).toHaveAttribute('aria-pressed', 'true');
    expect(within(sidePanel).getByText('条件/分支')).toBeInTheDocument();
    expect(within(sidePanel).getByText('来源节点')).toBeInTheDocument();
    expect(within(sidePanel).getByText('导入文章')).toBeInTheDocument();
    expect(within(sidePanel).getByText('目标节点')).toBeInTheDocument();
    expect(within(sidePanel).getByText('主文改写')).toBeInTheDocument();
    expect(within(sidePanel).queryByText('节点配置')).not.toBeInTheDocument();
    expect(within(leftColumn).queryByText('连线信息')).not.toBeInTheDocument();
    expect(within(leftColumn).queryByText('条件/分支')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '设为入口节点' })).toBeDisabled();
    expect(screen.queryByText('请在中间画布点击一个节点，再在此编辑节点名称、类型、模板、模型和上下文配置。')).not.toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '清空画布选择' }));
    });

    expect(screen.getByText('当前未选择节点')).toBeInTheDocument();
    expect(screen.getByText('当前未选择连线')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '设为入口节点' })).toBeDisabled();
  });

  it('supports fit view and graph toolbar actions without breaking selection state', async () => {
    const api = await import('../../lib/api/client');
    vi.mocked(api.updateWorkflow).mockResolvedValue(buildSavedWorkflow('node-rewrite'));

    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByTestId('node-node-rewrite'));
    });

    const toolbar = screen.getByRole('toolbar', { name: '工作流画布工具栏' });
    expect(within(toolbar).getByText('已选择节点: 主文改写')).toBeInTheDocument();
    expect(within(toolbar).getByRole('button', { name: '设为入口节点' })).toBeEnabled();
    expect(within(toolbar).getByRole('button', { name: '删除节点' })).toBeEnabled();
    expect(screen.queryByRole('button', { name: '画布适配视图' })).not.toBeInTheDocument();

    await act(async () => {
      fireEvent.click(within(toolbar).getByRole('button', { name: '适配视图' }));
    });

    expect(fitViewSpy).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId('node-node-rewrite')).toHaveAttribute('aria-pressed', 'true');

    await act(async () => {
      fireEvent.click(within(toolbar).getByRole('button', { name: '设为入口节点' }));
    });

    expect(within(toolbar).getByText('入口节点: 主文改写')).toBeInTheDocument();
    expect(screen.getByText('入口节点：主文改写')).toBeInTheDocument();
    expect(screen.getByTestId('node-node-rewrite')).toHaveAttribute('aria-pressed', 'true');

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '保存模板' }));
    });

    expect(await screen.findByText('工作流模板已保存。')).toBeInTheDocument();
    expect(screen.getByTestId('node-node-rewrite')).toHaveAttribute('aria-pressed', 'true');

    await act(async () => {
      fireEvent.click(within(toolbar).getByRole('button', { name: '删除节点' }));
    });

    expect(screen.queryByTestId('node-node-rewrite')).not.toBeInTheDocument();
    expect(within(toolbar).getByText('已选择节点: 导入文章')).toBeInTheDocument();
  });

  it('clears edge selection when save returns focus to a node', async () => {
    const api = await import('../../lib/api/client');
    vi.mocked(api.updateWorkflow).mockResolvedValue(buildSavedWorkflow('node-input'));

    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByTestId('edge-edge-node-input-node-rewrite-always-0'));
    });

    expect(screen.getByTestId('edge-edge-node-input-node-rewrite-always-0')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByText('已选择连线: 导入文章 -> 主文改写')).toBeInTheDocument();
    expect(screen.getByText('当前未选择节点')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '保存模板' }));
    });

    expect(await screen.findByText('工作流模板已保存。')).toBeInTheDocument();
    expect(screen.getByTestId('node-node-input')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('edge-edge-node-input-node-rewrite-always-0')).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByText('已选择节点: 导入文章')).toBeInTheDocument();
    expect(screen.getByText('已选择连线: 当前未选择连线')).toBeInTheDocument();
  });

  it('collapses and expands the right panel to reclaim canvas space', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    const canvasColumn = screen.getByTestId('workflow-canvas-column');
    const sidePanel = screen.getByTestId('workflow-side-panel');

    expect(sidePanel).toHaveAttribute('data-panel-state', 'expanded');
    expect(canvasColumn).toHaveAttribute('data-panel-state', 'expanded');

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '折叠右侧配置面板' }));
    });

    expect(sidePanel).toHaveAttribute('data-panel-state', 'collapsed');
    expect(canvasColumn).toHaveAttribute('data-panel-state', 'collapsed');
    expect(screen.queryByText('节点配置')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '展开右侧配置面板' })).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '展开右侧配置面板' }));
    });

    expect(sidePanel).toHaveAttribute('data-panel-state', 'expanded');
    expect(canvasColumn).toHaveAttribute('data-panel-state', 'expanded');
    expect(screen.getByText('节点配置')).toBeInTheDocument();
  });

  it('keeps right panel collapse and expand working for edge selections', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    const canvasColumn = screen.getByTestId('workflow-canvas-column');
    const sidePanel = screen.getByTestId('workflow-side-panel');

    await act(async () => {
      fireEvent.click(screen.getByTestId('edge-edge-node-input-node-rewrite-always-0'));
    });

    expect(within(sidePanel).getByText('条件/分支')).toBeInTheDocument();
    expect(within(sidePanel).queryByText('节点配置')).not.toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '折叠右侧配置面板' }));
    });

    expect(sidePanel).toHaveAttribute('data-panel-state', 'collapsed');
    expect(canvasColumn).toHaveAttribute('data-panel-state', 'collapsed');
    expect(screen.queryByText('条件/分支')).not.toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '展开右侧配置面板' }));
    });

    expect(sidePanel).toHaveAttribute('data-panel-state', 'expanded');
    expect(canvasColumn).toHaveAttribute('data-panel-state', 'expanded');
    expect(within(sidePanel).getByText('条件/分支')).toBeInTheDocument();
  });

  it('uses sectioned content instead of one long scrolling side panel', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    expect(screen.getByRole('tab', { name: '基础信息' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '模板绑定' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '模型参数' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '上下文' })).toBeInTheDocument();
    expect(screen.getByText('当前节点')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('tab', { name: '模板绑定' }));
    });

    expect(screen.getByLabelText('模板标识')).toBeInTheDocument();
    expect(screen.queryByLabelText('模型名称')).not.toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('tab', { name: '模型参数' }));
    });

    expect(screen.getByLabelText('模型名称')).toBeInTheDocument();
  });
});
