import { CssBaseline, ThemeProvider } from '@mui/material';
import { act } from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Edge, Node } from 'reactflow';
import WorkflowTemplatesPage, { createLocalWorkflowEdge } from './WorkflowTemplatesPage';
import theme from '../../theme/theme';

const fitViewSpy = vi.fn();

function installMatchMedia(width: number) {
  Object.defineProperty(window, 'innerWidth', {
    configurable: true,
    writable: true,
    value: width,
  });

  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: (query: string) => {
      const minWidth = Number(/min-width:\s*(\d+(?:\.\d+)?)px/.exec(query)?.[1] ?? Number.NEGATIVE_INFINITY);
      const maxWidth = Number(/max-width:\s*(\d+(?:\.\d+)?)px/.exec(query)?.[1] ?? Number.POSITIVE_INFINITY);
      const matches = width >= minWidth && width <= maxWidth;

      return {
        matches,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      };
    },
  });
}

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
  {
    id: 'workflow-beta',
    name: '审核兜底链路',
    description: '用于高风险稿件的人工审核分支。',
    version: 'v1.4.0',
    enabled: true,
    updated_by: '审核运营组',
    updated_at: '2026-05-07T10:15:00Z',
    entry_node_id: 'beta-input',
    nodes: [
      {
        id: 'beta-input',
        type: 'input',
        name: '接收稿件',
        config_json: JSON.stringify({
          label: '接收稿件',
          type: 'input',
          template: '',
          model: '',
          context: '接收待审核稿件。',
          position: { x: 100, y: 140 },
        }),
      },
      {
        id: 'beta-review',
        type: 'review',
        name: '人工审核',
        config_json: JSON.stringify({
          label: '人工审核',
          type: 'review',
          template: 'review.risk.manual',
          model: '',
          context: '由审核编辑确认发布风险。',
          position: { x: 360, y: 140 },
        }),
      },
    ],
    edges: [
      {
        from_node_id: 'beta-input',
        to_node_id: 'beta-review',
        condition: 'always',
        priority: 0,
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
    installMatchMedia(1440);
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

    fireEvent.click(rewriteNode);

    expect(rewriteNode).toHaveAttribute('aria-pressed', 'true');
    expect(inputNode).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByText('node-rewrite')).toBeInTheDocument();
    expect(screen.getByText('入口节点：导入文章')).toBeInTheDocument();
    expect(within(screen.getByRole('toolbar', { name: '工作流画布工具栏' })).getByRole('button', { name: '设为入口节点' })).toBeEnabled();

    fireEvent.click(firstEdge);

    const leftColumn = screen.getByTestId('workflow-list-column');
    const sidePanel = screen.getByTestId('workflow-side-panel');

    expect(firstEdge).toHaveAttribute('aria-pressed', 'true');
    expect(within(sidePanel).getByText('连线检查器')).toBeInTheDocument();
    expect(within(sidePanel).getByDisplayValue('always')).toBeInTheDocument();
    expect(within(sidePanel).getByDisplayValue('0')).toBeInTheDocument();
    expect(within(sidePanel).getByText('条件/分支')).toBeInTheDocument();
    expect(within(sidePanel).getByText('来源节点')).toBeInTheDocument();
    expect(within(sidePanel).getByText('导入文章')).toBeInTheDocument();
    expect(within(sidePanel).getByText('目标节点')).toBeInTheDocument();
    expect(within(sidePanel).getByText('主文改写')).toBeInTheDocument();
    expect(within(sidePanel).queryByText('节点配置')).not.toBeInTheDocument();
    expect(within(leftColumn).queryByText('连线信息')).not.toBeInTheDocument();
    expect(within(leftColumn).queryByText('条件/分支')).not.toBeInTheDocument();
    expect(within(screen.getByRole('toolbar', { name: '工作流画布工具栏' })).getByRole('button', { name: '设为入口节点' })).toBeDisabled();
    expect(screen.queryByText('请在中间画布点击一个节点，再在此编辑节点名称、类型、模板、模型和上下文配置。')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '清空画布选择' }));

    expect(within(sidePanel).getByText('工作流检查器')).toBeInTheDocument();
    expect(within(sidePanel).getByTestId('page-state-empty')).toBeInTheDocument();
    expect(screen.getByText('请先在画布中选择一个节点或连线。')).toBeInTheDocument();
    expect(within(screen.getByRole('toolbar', { name: '工作流画布工具栏' })).getByRole('button', { name: '设为入口节点' })).toBeDisabled();
  }, 10000);

  it('updates edge condition and priority through the inspector while preserving edge contract values', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByTestId('edge-edge-node-input-node-rewrite-always-0'));
    });

    await act(async () => {
      fireEvent.change(screen.getByLabelText('条件分支'), { target: { value: 'fallback' } });
    });

    await act(async () => {
      fireEvent.change(screen.getByLabelText('优先级'), { target: { value: '7' } });
    });

    expect(screen.getByTestId('edge-edge-node-input-node-rewrite-fallback-7')).toBeInTheDocument();
    expect(screen.getByText('已选择连线: 导入文章 -> 主文改写')).toBeInTheDocument();
    expect(screen.getByDisplayValue('fallback')).toBeInTheDocument();
    expect(screen.getByDisplayValue('7')).toBeInTheDocument();
  });

  it('preserves local edge uniqueness when editing a locally unique edge condition and priority', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByTestId('node-node-input'));
    });

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '为当前节点追加下游节点' }));
    });

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '改写节点' }));
    });

    const edgeLayer = screen.getByTestId('mock-edge-layer');
    const localEdge = within(edgeLayer)
      .getAllByRole('button')
      .find((button) => button.getAttribute('data-testid')?.startsWith('edge-edge-node-input-') && button.getAttribute('data-testid')?.includes('-always-2-'));

    expect(localEdge).toBeDefined();

    await act(async () => {
      fireEvent.click(localEdge!);
    });

    const originalTestId = localEdge!.getAttribute('data-testid');
    expect(originalTestId).toContain('-always-2-');

    await act(async () => {
      fireEvent.change(screen.getByLabelText('条件分支'), { target: { value: 'fallback' } });
    });

    await act(async () => {
      fireEvent.change(screen.getByLabelText('优先级'), { target: { value: '7' } });
    });

    const updatedLocalEdge = within(edgeLayer)
      .getAllByRole('button')
      .find((button) => button.getAttribute('data-testid')?.startsWith('edge-edge-node-input-') && button.getAttribute('data-testid')?.includes('-fallback-7-'));

    expect(updatedLocalEdge).toBeDefined();
    expect(updatedLocalEdge?.getAttribute('data-testid')).not.toBe('edge-edge-node-input-node-input-fallback-7');
    expect(updatedLocalEdge?.getAttribute('data-testid')).toContain('-fallback-7-');
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
    expect(within(toolbar).getByText('当前聚焦节点')).toBeInTheDocument();
    expect(screen.getByTestId('workflow-graph-panel')).toHaveAttribute('data-selection-kind', 'node');

    await act(async () => {
      fireEvent.click(within(toolbar).getByRole('button', { name: '适配视图' }));
    });

    expect(fitViewSpy).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId('node-node-rewrite')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByText('已锁定当前节点，可直接在右侧修改配置。')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(within(toolbar).getByRole('button', { name: '设为入口节点' }));
    });

    expect(within(toolbar).getByText('入口节点: 主文改写')).toBeInTheDocument();
    expect(screen.getByText('入口节点：主文改写')).toBeInTheDocument();
    expect(screen.getByTestId('node-node-rewrite')).toHaveAttribute('aria-pressed', 'true');

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '保存未保存更改' }));
    });

    expect(await screen.findByText('工作流模板已保存。')).toBeInTheDocument();
    expect(screen.getByTestId('node-node-rewrite')).toHaveAttribute('aria-pressed', 'true');

    await act(async () => {
      fireEvent.click(within(toolbar).getByRole('button', { name: '删除节点' }));
    });

    expect(screen.queryByTestId('node-node-rewrite')).not.toBeInTheDocument();
    expect(within(toolbar).getByText('已选择节点: 导入文章')).toBeInTheDocument();
  });

  it('gives the canvas primary visual prominence over surrounding controls', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    const graphPanel = screen.getByTestId('workflow-graph-panel');
    const canvasRegion = screen.getByRole('region', { name: '工作流画布' });
    const toolbar = screen.getByRole('toolbar', { name: '工作流画布工具栏' });

    expect(graphPanel).toHaveAttribute('data-emphasis', 'primary');
    expect(graphPanel).toHaveAttribute('data-panel-state', 'expanded');
    expect(graphPanel).toHaveAttribute('data-selection-kind', 'node');
    expect(canvasRegion).toBeInTheDocument();
    expect(screen.getByText('主画布')).toBeInTheDocument();
    expect(screen.getByText('拖拽、缩放与连线操作集中在此完成')).toBeInTheDocument();
    expect(within(toolbar).getByText('画布为主操作区，右侧面板跟随当前选择同步更新。')).toBeInTheDocument();
  });

  it('keeps selection, fit-view, and panel interactions coherent', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    const toolbar = screen.getByRole('toolbar', { name: '工作流画布工具栏' });
    const graphPanel = screen.getByTestId('workflow-graph-panel');

    expect(graphPanel).toHaveAttribute('data-selection-kind', 'node');
    expect(within(toolbar).getByText('当前聚焦节点')).toBeInTheDocument();
    expect(screen.getByText('正在编辑节点')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByTestId('edge-edge-node-input-node-rewrite-always-0'));
    });

    expect(graphPanel).toHaveAttribute('data-selection-kind', 'edge');
    expect(within(toolbar).getByText('当前聚焦连线')).toBeInTheDocument();
    expect(screen.getByText('当前正在查看连线条件与去向。')).toBeInTheDocument();
    expect(screen.getByText('正在检查连线')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(within(toolbar).getByRole('button', { name: '适配视图' }));
    });

    expect(fitViewSpy).toHaveBeenCalledTimes(1);
    expect(graphPanel).toHaveAttribute('data-selection-kind', 'edge');

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '折叠右侧配置面板' }));
    });

    expect(graphPanel).toHaveAttribute('data-panel-state', 'collapsed');

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '展开右侧配置面板' }));
    });

    expect(graphPanel).toHaveAttribute('data-panel-state', 'expanded');
    expect(graphPanel).toHaveAttribute('data-selection-kind', 'edge');
    expect(screen.getByText('当前正在查看连线条件与去向。')).toBeInTheDocument();
  });

  it('preserves edge selection after save even when the saved entry node differs', async () => {
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
    expect(screen.getByTestId('node-node-input')).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByTestId('edge-edge-node-input-node-rewrite-always-0')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByText('已选择节点: 当前未选择节点')).toBeInTheDocument();
    expect(screen.getByText('已选择连线: 导入文章 -> 主文改写')).toBeInTheDocument();
  });

  it('preserves the current edge selection after save when the saved edge id still exists', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByTestId('edge-edge-node-input-node-rewrite-always-0'));
    });

    expect(screen.getByTestId('edge-edge-node-input-node-rewrite-always-0')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByText('已选择连线: 导入文章 -> 主文改写')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '保存模板' }));
    });

    expect(await screen.findByText('工作流模板已保存。')).toBeInTheDocument();
    expect(screen.getByTestId('edge-edge-node-input-node-rewrite-always-0')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByText('已选择连线: 导入文章 -> 主文改写')).toBeInTheDocument();
    expect(screen.getByText('已选择节点: 当前未选择节点')).toBeInTheDocument();
  });

  it('preserves the matching saved edge selection after saving a locally created edge', async () => {
    const api = await import('../../lib/api/client');
    vi.mocked(api.updateWorkflow).mockImplementationOnce(async (_id, payload) => ({
      id: 'workflow-alpha',
      name: workflowFixtures[0].name,
      description: workflowFixtures[0].description,
      version: workflowFixtures[0].version,
      enabled: workflowFixtures[0].enabled,
      updated_by: 'react-webapp',
      updated_at: '2026-05-07T11:30:00Z',
      entry_node_id: payload.entry_node_id,
      nodes: payload.nodes,
      edges: payload.edges,
    }));

    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByTestId('node-node-input'));
    });

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '为当前节点追加下游节点' }));
    });

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '改写节点' }));
    });

    const edgeLayer = screen.getByTestId('mock-edge-layer');
    const localEdge = within(edgeLayer)
      .getAllByRole('button')
      .find((button) => button.getAttribute('data-testid')?.startsWith('edge-edge-node-input-') && button.getAttribute('data-testid')?.includes('-always-2-'));

    expect(localEdge).toBeDefined();

    await act(async () => {
      fireEvent.click(localEdge!);
    });

    expect(localEdge).toHaveAttribute('aria-pressed', 'true');

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '保存未保存更改' }));
    });

    expect(await screen.findByText('工作流模板已保存。')).toBeInTheDocument();

    const savedEdge = within(edgeLayer)
      .getAllByRole('button')
      .find((button) => {
        const testId = button.getAttribute('data-testid') ?? '';
        return testId.startsWith('edge-edge-node-input-') && testId.endsWith('-always-2');
      });

    expect(savedEdge).toBeDefined();
    expect(savedEdge).toHaveAttribute('aria-pressed', 'true');
  });

  it('guards refresh when the current template has unsaved changes', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm');
    confirmSpy.mockReturnValueOnce(false).mockReturnValueOnce(true);

    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.change(screen.getByLabelText('节点名称'), { target: { value: '导入文章（待刷新）' } });
    });

    expect(screen.getByText('有未保存更改')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '刷新画布' }));
    });

    expect(confirmSpy).toHaveBeenCalledWith('当前模板有未保存更改，刷新后将丢失这些修改。是否继续？');
    expect(screen.getByDisplayValue('导入文章（待刷新）')).toBeInTheDocument();
    expect(screen.getByText('有未保存更改')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '刷新画布' }));
    });

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();
    expect(screen.getByDisplayValue('导入文章')).toBeInTheDocument();
    expect(screen.queryByText('有未保存更改')).not.toBeInTheDocument();
  });

  it('does not expose page-level dirty tracking state on the canvas column', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    const canvasColumn = screen.getByTestId('workflow-canvas-column');
    expect(canvasColumn).not.toHaveAttribute('data-dirty-state');
  });

  it('shows explicit dirty-state cues after local edits and clears them after save', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();
    expect(screen.queryByText('有未保存更改')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '保存模板' })).toHaveTextContent('保存模板');

    await act(async () => {
      fireEvent.change(screen.getByLabelText('节点名称'), { target: { value: '导入文章（已修改）' } });
    });

    expect(screen.getByText('有未保存更改')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '保存未保存更改' })).toHaveTextContent('保存未保存更改');
    expect(screen.getByRole('button', { name: /品牌改写主链路.*未保存/ })).toHaveTextContent('未保存');
    expect(screen.getByTestId('workflow-graph-panel')).toHaveAttribute('data-dirty-state', 'dirty');

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '保存未保存更改' }));
    });

    expect(await screen.findByText('工作流模板已保存。')).toBeInTheDocument();
    expect(screen.queryByText('有未保存更改')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '保存模板' })).toHaveTextContent('保存模板');
    expect(screen.getByRole('button', { name: /品牌改写主链路/ })).not.toHaveTextContent('未保存');
    expect(screen.getByTestId('workflow-graph-panel')).toHaveAttribute('data-dirty-state', 'clean');
  });

  it('guards template switching when the current template has unsaved changes', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm');
    confirmSpy.mockReturnValueOnce(false).mockReturnValueOnce(true);

    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.change(screen.getByLabelText('节点名称'), { target: { value: '导入文章（未保存）' } });
    });

    expect(screen.getByText('有未保存更改')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /审核兜底链路/ }));
    });

    expect(confirmSpy).toHaveBeenCalledWith('当前模板有未保存更改，切换后将丢失这些修改。是否继续？');
    expect(screen.getByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();
    expect(screen.getByDisplayValue('导入文章（未保存）')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /审核兜底链路/ }));
    });

    expect(screen.getByText('当前选中模板：审核兜底链路')).toBeInTheDocument();
    expect(screen.queryByText('有未保存更改')).not.toBeInTheDocument();
    expect(screen.getByDisplayValue('接收稿件')).toBeInTheDocument();
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
    expect(screen.getByText('节点检查器')).toBeInTheDocument();
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

  it('creates a node from the empty-canvas creation entry', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '清空画布选择' }));
    });

    const graphPanel = screen.getByTestId('workflow-graph-panel');
    expect(graphPanel).toHaveAttribute('data-selection-kind', 'idle');

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '在空白画布创建节点' }));
    });

    expect(screen.getByRole('textbox', { name: '搜索节点类型' })).toBeInTheDocument();
    expect(within(screen.getByTestId('mock-node-layer')).getAllByRole('button')).toHaveLength(3);
    expect(within(screen.getByTestId('mock-edge-layer')).getAllByRole('button')).toHaveLength(2);

    await act(async () => {
      fireEvent.change(screen.getByRole('textbox', { name: '搜索节点类型' }), { target: { value: '审核' } });
    });

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '审核节点' }));
    });

    const createdNode = within(screen.getByTestId('mock-node-layer'))
      .getByText((content) => content.includes('审核节点 4'))
      .closest('button');
    expect(createdNode).toBeInTheDocument();
    expect(createdNode).toHaveAttribute('aria-pressed', 'true');
    expect(within(screen.getByTestId('mock-node-layer')).getAllByRole('button')).toHaveLength(4);
    expect(within(screen.getByTestId('mock-edge-layer')).getAllByRole('button')).toHaveLength(2);
    expect(screen.getByText('已选择节点: 审核节点 4')).toBeInTheDocument();
    expect(screen.getByText('节点 4')).toBeInTheDocument();
    expect(screen.getByText('连线 2')).toBeInTheDocument();
    expect(graphPanel).toHaveAttribute('data-selection-kind', 'node');
  });

  it('appends a downstream node from the selected-node flow', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByTestId('node-node-rewrite'));
    });

    const graphPanel = screen.getByTestId('workflow-graph-panel');
    expect(graphPanel).toHaveAttribute('data-selection-kind', 'node');

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '为当前节点追加下游节点' }));
    });

    expect(screen.getByRole('textbox', { name: '搜索节点类型' })).toBeInTheDocument();
    expect(within(screen.getByTestId('mock-node-layer')).getAllByRole('button')).toHaveLength(3);
    expect(within(screen.getByTestId('mock-edge-layer')).getAllByRole('button')).toHaveLength(2);

    await act(async () => {
      fireEvent.change(screen.getByRole('textbox', { name: '搜索节点类型' }), { target: { value: '渲染' } });
    });

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '渲染节点' }));
    });

    const appendedNode = within(screen.getByTestId('mock-node-layer'))
      .getByText((content) => content.includes('渲染节点 4'))
      .closest('button');
    expect(appendedNode).toBeInTheDocument();
    expect(appendedNode).toHaveAttribute('aria-pressed', 'true');
    expect(within(screen.getByTestId('mock-node-layer')).getAllByRole('button')).toHaveLength(4);
    expect(within(screen.getByTestId('mock-edge-layer')).getAllByRole('button')).toHaveLength(3);
    expect(screen.getByText('已选择节点: 渲染节点 4')).toBeInTheDocument();
    expect(screen.getByText('节点 4')).toBeInTheDocument();
    expect(screen.getByText('连线 3')).toBeInTheDocument();
    expect(graphPanel).toHaveAttribute('data-selection-kind', 'node');
  });

  it('opens standalone creation from the generic add action when no node is selected', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '清空画布选择' }));
    });

    expect(screen.getByTestId('workflow-graph-panel')).toHaveAttribute('data-selection-kind', 'idle');

    await act(async () => {
      fireEvent.click(within(screen.getByRole('toolbar', { name: '工作流画布工具栏' })).getByRole('button', { name: '新增节点' }));
    });

    expect(screen.getByRole('textbox', { name: '搜索节点类型' })).toBeInTheDocument();
    expect(within(screen.getByTestId('mock-node-layer')).getAllByRole('button')).toHaveLength(3);
    expect(within(screen.getByTestId('mock-edge-layer')).getAllByRole('button')).toHaveLength(2);
  });

  it('supports keyboard delete and escape for the current canvas selection', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByTestId('node-node-rewrite'));
    });

    expect(screen.getByTestId('node-node-rewrite')).toHaveAttribute('aria-pressed', 'true');

    await act(async () => {
      fireEvent.keyDown(window, { key: 'Escape' });
    });

    expect(screen.getByText('当前未选择节点')).toBeInTheDocument();
    expect(screen.getByTestId('workflow-graph-panel')).toHaveAttribute('data-selection-kind', 'idle');

    await act(async () => {
      fireEvent.click(screen.getByTestId('node-node-rewrite'));
    });

    await act(async () => {
      fireEvent.keyDown(window, { key: 'Delete' });
    });

    expect(screen.queryByTestId('node-node-rewrite')).not.toBeInTheDocument();
    expect(screen.getByText('已选择节点: 导入文章')).toBeInTheDocument();
  });

  it('duplicates the selected node from keyboard shortcuts', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByTestId('node-node-rewrite'));
    });

    await act(async () => {
      fireEvent.keyDown(window, { key: 'd', ctrlKey: true });
    });

    const nodeLayer = screen.getByTestId('mock-node-layer');
    expect(within(nodeLayer).getAllByRole('button')).toHaveLength(4);
    expect(within(nodeLayer).getByText((content) => content.includes('主文改写（副本）'))).toBeInTheDocument();
    expect(screen.getByText('已选择节点: 主文改写（副本）')).toBeInTheDocument();
  });

  it('shows canvas quick actions for a selected node and supports quick set-entry and delete flows', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByTestId('node-node-rewrite'));
    });

    const quickActions = screen.getByTestId('workflow-canvas-quick-actions');
    expect(within(quickActions).getByRole('button', { name: '设为入口节点' })).toBeEnabled();
    expect(within(quickActions).getByRole('button', { name: '删除节点' })).toBeEnabled();

    await act(async () => {
      fireEvent.click(within(quickActions).getByRole('button', { name: '设为入口节点' }));
    });

    expect(screen.getByText('入口节点：主文改写')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(within(quickActions).getByRole('button', { name: '删除节点' }));
    });

    expect(screen.queryByTestId('node-node-rewrite')).not.toBeInTheDocument();
  });

  it('closes the create menu when template or destructive selection state changes', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '清空画布选择' }));
    });

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '在空白画布创建节点' }));
    });

    expect(screen.getByRole('textbox', { name: '搜索节点类型' })).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '新建模板' }));
    });

    expect(screen.queryByRole('textbox', { name: '搜索节点类型' })).not.toBeInTheDocument();
    expect(screen.getByText('当前选中模板：工作流模板 3')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '为当前节点追加下游节点' }));
    });

    expect(screen.getByRole('textbox', { name: '搜索节点类型' })).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(within(screen.getByRole('toolbar', { name: '工作流画布工具栏' })).getByRole('button', { name: '删除节点' }));
    });

    expect(screen.queryByRole('textbox', { name: '搜索节点类型' })).not.toBeInTheDocument();
  });

  it('closes the create menu after refresh and successful save recompute editor state', async () => {
    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '清空画布选择' }));
    });

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '在空白画布创建节点' }));
    });

    expect(screen.getByRole('textbox', { name: '搜索节点类型' })).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '刷新画布' }));
    });

    expect(screen.queryByRole('textbox', { name: '搜索节点类型' })).not.toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '为当前节点追加下游节点' }));
    });

    expect(screen.getByRole('textbox', { name: '搜索节点类型' })).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '保存模板' }));
    });

    expect(await screen.findByText('工作流模板已保存。')).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: '搜索节点类型' })).not.toBeInTheDocument();
  });

  it('defaults to a compact collapsed side panel on narrow layouts', async () => {
    installMatchMedia(900);

    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    const canvasColumn = screen.getByTestId('workflow-canvas-column');
    const sidePanel = screen.getByTestId('workflow-side-panel');

    expect(canvasColumn).toHaveAttribute('data-layout-mode', 'stacked');
    expect(sidePanel).toHaveAttribute('data-layout-mode', 'stacked');
    expect(sidePanel).toHaveAttribute('data-panel-state', 'collapsed');
    expect(sidePanel).toHaveAttribute('data-collapsed-footprint', 'compact');
    expect(screen.queryByText('节点配置')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '展开右侧配置面板' })).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '展开右侧配置面板' }));
    });

    expect(sidePanel).toHaveAttribute('data-panel-state', 'expanded');
    expect(sidePanel).toHaveAttribute('data-collapsed-footprint', 'full');
    expect(screen.getByText('节点检查器')).toBeInTheDocument();
  });

  it('uses finite side columns as soon as the layout switches to row mode at lg widths', async () => {
    installMatchMedia(1280);

    renderWorkflowTemplatesPage();

    expect(await screen.findByText('当前选中模板：品牌改写主链路')).toBeInTheDocument();

    const listColumn = screen.getByTestId('workflow-list-column');
    const canvasColumn = screen.getByTestId('workflow-canvas-column');
    const sidePanel = screen.getByTestId('workflow-side-panel');

    expect(canvasColumn).toHaveAttribute('data-layout-mode', 'side-by-side');
    expect(listColumn).toHaveAttribute('data-width-mode', 'fixed');
    expect(sidePanel).toHaveAttribute('data-layout-mode', 'side-by-side');
    expect(sidePanel).toHaveAttribute('data-width-mode', 'fixed');
    expect(sidePanel).toHaveAttribute('data-collapsed-footprint', 'full');
    expect(screen.getByText('节点检查器')).toBeInTheDocument();
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

  it('shows inspector idle guidance when no workflow templates are returned', async () => {
    const api = await import('../../lib/api/client');
    vi.mocked(api.listWorkflows).mockResolvedValue([]);

    renderWorkflowTemplatesPage();

    expect((await screen.findAllByTestId('page-state-empty')).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('暂无工作流模板')).toBeInTheDocument();
    expect(screen.getByText('点击上方“新建模板”开始配置新的流程定义。')).toBeInTheDocument();
    expect(screen.getByText('请先在画布中选择一个节点或连线。')).toBeInTheDocument();
  });

  it('shows an explicit error state while keeping the editor layout visible', async () => {
    const api = await import('../../lib/api/client');
    vi.mocked(api.listWorkflows).mockRejectedValue(new api.ApiError(500, '工作流列表加载失败，请刷新后重试。'));

    renderWorkflowTemplatesPage();

    expect((await screen.findAllByTestId('page-state-error')).length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText('工作流列表加载失败，请刷新后重试。').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByTestId('workflow-list-column')).toBeInTheDocument();
    expect(screen.getByTestId('workflow-canvas-column')).toBeInTheDocument();
    expect(screen.getByTestId('workflow-side-panel')).toBeInTheDocument();
  });

  it('keeps the normal empty state when a workflow action fails on an empty dataset', async () => {
    const api = await import('../../lib/api/client');
    vi.mocked(api.listWorkflows).mockResolvedValue([]);
    vi.mocked(api.createWorkflow).mockRejectedValue(new api.ApiError(409, '工作流保存失败：名称重复。'));

    renderWorkflowTemplatesPage();

    expect((await screen.findAllByText('暂无工作流模板')).length).toBeGreaterThanOrEqual(1);

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '新建模板' }));
    });

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '保存未保存更改' }));
    });

    expect(await screen.findByText('工作流保存失败：名称重复。')).toBeInTheDocument();
    expect(within(screen.getByTestId('workflow-list-column')).getByTestId('page-state-empty')).toBeInTheDocument();
    expect(screen.queryByText('工作流模板暂时不可用')).not.toBeInTheDocument();
    expect(screen.queryByText('工作流编辑器暂时不可用')).not.toBeInTheDocument();
  });
});
