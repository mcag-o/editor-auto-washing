import { CssBaseline, ThemeProvider } from '@mui/material';
import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import DashboardPage from './DashboardPage';
import theme from '../../theme/theme';

function jsonResponse(body: unknown, init: ResponseInit = {}) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
}

function renderDashboardPage() {
  return render(
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <DashboardPage />
    </ThemeProvider>,
  );
}

describe('DashboardPage', () => {
  beforeEach(() => {
    vi.spyOn(globalThis, 'fetch').mockImplementation((input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const method = init?.method ?? 'GET';

      if (url.endsWith('/api/system/status') && method === 'GET') {
        return Promise.resolve(
          jsonResponse({
            id: 'control-1',
            state: 'running',
            reason: 'started',
            metadata: { concurrency_limit: 3 },
            updated_by: 'local-admin',
            requested_at: '2026-05-07T02:30:00Z',
            updated_at: '2026-05-07T02:31:00Z',
          }),
        );
      }

      if (url.endsWith('/api/articles') && method === 'GET') {
        return Promise.resolve(
          jsonResponse({
            data: [
              { id: 'a-1', title: '待处理文章', body: 'abc', status: 'pending' },
              { id: 'a-2', title: '处理中文章', body: 'abc', status: 'processing' },
              { id: 'a-3', title: '已完成文章', body: 'abc', status: 'completed' },
              { id: 'a-4', title: '失败文章', body: 'abc', status: 'failed' },
            ],
          }),
        );
      }

      if (url.endsWith('/api/templates') && method === 'GET') {
        return Promise.resolve(
          jsonResponse({
            data: [
              { id: 'tpl-1', enabled: true },
              { id: 'tpl-2', enabled: false },
              { id: 'tpl-3', enabled: true },
            ],
          }),
        );
      }

      if (url.endsWith('/api/audit') && method === 'GET') {
        return Promise.resolve(
          jsonResponse({
            data: [
              { id: 'log-1', result: 'failure', message: '流程暂停', action: 'pause', actor: 'local-admin', resource: 'system' },
            ],
          }),
        );
      }

      throw new Error(`Unhandled fetch: ${method} ${url}`);
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders live summary metrics from API data', async () => {
    renderDashboardPage();

    expect(screen.getByRole('heading', { name: '控制台总览' })).toBeInTheDocument();
    expect(await screen.findByText('系统当前状态：运行中')).toBeInTheDocument();
    expect(await screen.findByText('2 个已启用')).toBeInTheDocument();
    expect(screen.getAllByText('失败文章 1 条，建议优先进入文章队列处理。').length).toBeGreaterThan(0);
    expect(screen.getByText('来源于真实文章队列中的 pending 状态。')).toBeInTheDocument();
  });
});
