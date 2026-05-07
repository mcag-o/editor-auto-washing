import { CssBaseline, ThemeProvider } from '@mui/material';
import { act } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import TemplatesPage from './TemplatesPage';
import { parseTemplateStages } from '../../lib/mappers/template';
import theme from '../../theme/theme';

const apiTemplates = [
  {
    id: 'tpl-brand-core',
    name: '品牌改写主模板',
    type: 'prompt',
    version: 'v2.1.0',
    enabled: true,
    content: '你是一名中文内容编辑。',
    variables_json: {
      summary: '适用于品牌稿的主模板。',
      stages: [
        { label: '语境校准', note: '识别品牌主体和读者对象。' },
        { label: '主文改写', note: '输出结构稳定的正文。' },
      ],
    },
    updated_by: '运营编辑组',
    updated_at: '2026-05-06T20:30:00Z',
  },
  {
    id: 'tpl-review-safe',
    name: '审校兜底模板',
    type: 'review',
    version: 'v1.0.3',
    enabled: false,
    content: '你是质量审校助手。',
    variables_json: {
      summary: '渲染前做事实和风格复核。',
      stages: [{ label: '事实核查', note: '关注日期和数据。' }],
    },
    updated_by: '审核流程模板',
    updated_at: '2026-05-04T16:48:00Z',
  },
];

function jsonResponse(body: unknown, init: ResponseInit = {}) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
}

function renderTemplatesPage() {
  return render(
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <TemplatesPage />
    </ThemeProvider>,
  );
}

describe('TemplatesPage', () => {
  beforeEach(() => {
    vi.spyOn(globalThis, 'fetch').mockImplementation((input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const method = init?.method ?? 'GET';

      if (url.endsWith('/api/templates') && method === 'GET') {
        return Promise.resolve(jsonResponse({ data: apiTemplates }));
      }

      if (url.endsWith('/api/templates') && method === 'POST') {
        return Promise.resolve(
          jsonResponse(
            {
              id: 'tpl-created',
              name: '新建模板',
              type: 'prompt',
              version: 'v4.0.0',
              enabled: false,
              content: '新的提示词',
              variables_json: {
                summary: '新的模板摘要',
                stages: [{ label: '输入理解', note: '说明第一阶段。' }],
              },
              updated_by: 'react-webapp',
              updated_at: '2026-05-07T03:00:00Z',
            },
            { status: 201 },
          ),
        );
      }

      if (url.endsWith('/api/templates/tpl-brand-core') && method === 'PUT') {
        return Promise.resolve(
          jsonResponse({
            ...apiTemplates[0],
            enabled: false,
            updated_by: 'react-webapp',
            updated_at: '2026-05-07T03:00:00Z',
          }),
        );
      }

      throw new Error(`Unhandled fetch: ${method} ${url}`);
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('parses stage lines with an ASCII colon', () => {
    expect(parseTemplateStages('阶段名: 说明')).toEqual([
      {
        key: 'stage-1',
        label: '阶段名',
        note: '说明',
      },
    ]);
  });

  it('parses stage lines with a full-width Chinese colon', () => {
    expect(parseTemplateStages('阶段名：说明')).toEqual([
      {
        key: 'stage-1',
        label: '阶段名',
        note: '说明',
      },
    ]);
  });

  it('loads templates from the API and allows creating a new template', async () => {
    renderTemplatesPage();

    expect(screen.getByRole('heading', { name: '模板管理' })).toBeInTheDocument();
    expect(await screen.findByText('共 2 个模板，1 个已启用。')).toBeInTheDocument();
    expect(screen.getAllByText('适用于品牌稿的主模板。').length).toBeGreaterThan(0);
    expect(screen.getAllByText('更新人 运营编辑组').length).toBeGreaterThan(0);

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '新建模板' }));
    });
    expect(screen.getByRole('heading', { name: '新建模板' })).toBeInTheDocument();

    await act(async () => {
      fireEvent.change(screen.getByLabelText('模板名称'), { target: { value: '新建模板' } });
      fireEvent.change(screen.getByLabelText('模板摘要'), { target: { value: '新的模板摘要' } });
      fireEvent.change(screen.getByLabelText('主提示词'), { target: { value: '新的提示词' } });
      fireEvent.change(screen.getByLabelText('阶段说明'), { target: { value: '输入理解: 说明第一阶段。' } });
    });

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '保存模板' }));
    });

    expect(await screen.findByText('模板已创建。')).toBeInTheDocument();
    expect(screen.getByText('共 3 个模板，1 个已启用。')).toBeInTheDocument();
    expect(screen.getAllByText('新的模板摘要').length).toBeGreaterThan(0);
    expect(screen.getAllByText('新建模板').length).toBeGreaterThan(0);
  });

  it('loads templates from the API and allows updating an existing template', async () => {
    renderTemplatesPage();

    expect(await screen.findByText('共 2 个模板，1 个已启用。')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getAllByRole('button', { name: '操作' })[0]);
    });

    await act(async () => {
      fireEvent.click(screen.getByRole('menuitem', { name: '编辑模板' }));
    });

    expect(screen.getByRole('heading', { name: '编辑模板' })).toBeInTheDocument();
    expect(screen.getByLabelText('模板名称')).toHaveValue('品牌改写主模板');

    await act(async () => {
      fireEvent.click(screen.getByLabelText('启用中'));
    });

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '保存模板' }));
    });

    await waitFor(() => {
      expect(screen.getByText('模板已保存。')).toBeInTheDocument();
    });
    expect(screen.getAllByText('已停用').length).toBeGreaterThan(0);
  });

  it('shows an explicit empty state when no templates are returned', async () => {
    vi.mocked(globalThis.fetch).mockImplementation((input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const method = init?.method ?? 'GET';

      if (url.endsWith('/api/templates') && method === 'GET') {
        return Promise.resolve(jsonResponse({ data: [] }));
      }

      throw new Error(`Unhandled fetch: ${method} ${url}`);
    });

    renderTemplatesPage();

    expect((await screen.findAllByTestId('page-state-empty')).length).toBe(2);
    expect(screen.getByText('暂无模板记录')).toBeInTheDocument();
    expect(screen.getAllByTestId('page-state-empty')).toHaveLength(2);
    expect(screen.getByText('请选择一个模板查看详细内容。')).toBeInTheDocument();
  });

  it('shows an explicit error state while keeping the page structure visible', async () => {
    vi.mocked(globalThis.fetch).mockImplementation((input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const method = init?.method ?? 'GET';

      if (url.endsWith('/api/templates') && method === 'GET') {
        return Promise.resolve(
          new Response(JSON.stringify({ message: '模板列表加载失败，请刷新后重试。' }), {
            status: 500,
            headers: { 'Content-Type': 'application/json' },
          }),
        );
      }

      throw new Error(`Unhandled fetch: ${method} ${url}`);
    });

    renderTemplatesPage();

    expect(await screen.findByTestId('page-state-error')).toBeInTheDocument();
    expect(screen.getByText('模板列表加载失败，请刷新后重试。')).toBeInTheDocument();
    expect(screen.getByText('模板列表')).toBeInTheDocument();
    expect(screen.getByText('内容预览')).toBeInTheDocument();
  });
});
