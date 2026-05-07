import { CssBaseline, ThemeProvider } from '@mui/material';
import { act } from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import AuditPage from './AuditPage';
import theme from '../../theme/theme';

function deferredResponse() {
  let resolve: (response: Response) => void;
  const promise = new Promise<Response>((res) => {
    resolve = res;
  });

  return {
    promise,
    resolve: resolve!,
  };
}

function jsonResponse(body: unknown, init: ResponseInit = {}) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
}

function renderAuditPage() {
  return render(
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <AuditPage />
    </ThemeProvider>,
  );
}

describe('AuditPage', () => {
  beforeEach(() => {
    vi.spyOn(globalThis, 'fetch').mockImplementation((input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const method = init?.method ?? 'GET';

      if (url.endsWith('/api/audit') && method === 'GET') {
        return Promise.resolve(
          jsonResponse({
            data: [
              {
                id: 'log-1',
                actor: 'local-admin',
                action: 'pause',
                resource: 'system',
                resource_id: 'system-1',
                result: 'failure',
                message: '流程暂停',
                metadata: { reason: 'manual' },
                created_at: '2026-05-07T03:00:00Z',
              },
              {
                id: 'log-2',
                actor: 'workflow-bot',
                action: 'resume',
                resource: 'article',
                resource_id: 'article-2',
                result: 'success',
                message: '任务恢复',
                metadata: { source: 'scheduler' },
                created_at: '2026-05-07T04:00:00Z',
              },
            ],
          }),
        );
      }

      if (url.endsWith('/api/audit/log-1') && method === 'GET') {
        return Promise.resolve(
          jsonResponse({
            id: 'log-1',
            actor: 'local-admin',
            action: 'pause',
            resource: 'system',
            resource_id: 'system-1',
            result: 'failure',
            message: '流程暂停',
            metadata: { reason: 'manual', operator: 'A-01' },
            created_at: '2026-05-07T03:00:00Z',
          }),
        );
      }

      if (url.endsWith('/api/audit/log-2') && method === 'GET') {
        return Promise.resolve(
          new Response(JSON.stringify({ message: '审计详情加载失败' }), {
            status: 500,
            headers: { 'Content-Type': 'application/json' },
          }),
        );
      }

      throw new Error(`Unhandled fetch: ${method} ${url}`);
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('loads audit rows and renders the detail panel without crashing', async () => {
    renderAuditPage();

    expect(screen.getByRole('heading', { name: '操作审计' })).toBeInTheDocument();
    expect(await screen.findByText('流程暂停')).toBeInTheDocument();
    expect(screen.getByText('任务恢复')).toBeInTheDocument();

    const detailCard = screen.getByTestId('audit-detail-card');
    expect(await within(detailCard).findByText('操作人')).toBeInTheDocument();
    expect(within(detailCard).getByText('local-admin')).toBeInTheDocument();
    expect(within(detailCard).getByText('操作结果')).toBeInTheDocument();
    expect(within(detailCard).getByText('结果：失败')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('row', { name: /log-1/i }));
    });

    expect(await within(detailCard).findByText('目标资源')).toBeInTheDocument();
    expect(within(detailCard).getByText('system / system-1')).toBeInTheDocument();
    expect(within(detailCard).getByDisplayValue(/"operator": "A-01"/)).toBeInTheDocument();
  });

  it('auto-selects the first visible record so list and detail stay in sync', async () => {
    renderAuditPage();

    expect(await screen.findByRole('row', { name: /log-1/i })).toHaveClass('Mui-selected');
    expect(screen.getAllByText('local-admin').length).toBeGreaterThan(0);
    expect(screen.getByText('system')).toBeInTheDocument();
    expect(screen.getByText('system-1')).toBeInTheDocument();
  });

  it('shows an empty state when no audit records are returned', async () => {
    vi.mocked(globalThis.fetch).mockImplementation((input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const method = init?.method ?? 'GET';

      if (url.endsWith('/api/audit') && method === 'GET') {
        return Promise.resolve(jsonResponse({ data: [] }));
      }

      throw new Error(`Unhandled fetch: ${method} ${url}`);
    });

    renderAuditPage();

    expect(await screen.findByText('暂无审计记录')).toBeInTheDocument();
    const listCard = await screen.findByTestId('audit-list-card');
    expect(within(listCard).getByTestId('page-state-empty')).toBeInTheDocument();
    expect(within(listCard).getByText('暂无审计记录')).toBeInTheDocument();
    expect(screen.getByText('当前结果 0 条，列表与详情都来自现有审计 API。')).toBeInTheDocument();
    expect(within(screen.getByTestId('audit-detail-card')).getByTestId('page-state-empty')).toBeInTheDocument();
    expect(screen.getByText('请选择一条记录查看审计详情。')).toBeInTheDocument();
  });

  it('keeps the list visible when detail fetch fails', async () => {
    renderAuditPage();

    expect(await screen.findByText('流程暂停')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('row', { name: /log-2/i }));
    });

    const detailCard = screen.getByRole('heading', { name: '审计详情' }).closest('.MuiCard-root');
    expect(detailCard).not.toBeNull();

    expect(await within(detailCard as HTMLElement).findByTestId('page-state-error')).toBeInTheDocument();
    expect(screen.getByText('流程暂停')).toBeInTheDocument();
    expect(screen.getByText('任务恢复')).toBeInTheDocument();
    expect(within(detailCard as HTMLElement).getByTestId('page-state-error')).toBeInTheDocument();
    expect(within(detailCard as HTMLElement).getByText('审计详情加载失败', { exact: false })).toBeInTheDocument();
    expect(within(detailCard as HTMLElement).getAllByRole('alert')).toHaveLength(1);
  });

  it('keeps the latest failed selection without showing stale prior detail', async () => {
    renderAuditPage();

    expect(await screen.findByText('流程暂停')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('row', { name: /log-1/i }));
    });

    expect(await screen.findByText('system / system-1')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('row', { name: /log-2/i }));
    });

    const detailCard = screen.getByRole('heading', { name: '审计详情' }).closest('.MuiCard-root');
    expect(detailCard).not.toBeNull();

    expect(await within(detailCard as HTMLElement).findByTestId('page-state-error')).toBeInTheDocument();
    expect(screen.getByRole('row', { name: /log-2/i })).toHaveClass('Mui-selected');
    expect(within(detailCard as HTMLElement).queryByText('system / system-1')).not.toBeInTheDocument();
    expect(within(detailCard as HTMLElement).queryByDisplayValue(/"operator": "A-01"/)).not.toBeInTheDocument();
  });

  it('ignores out-of-order detail responses and keeps the latest selection visible', async () => {
    const firstDetail = deferredResponse();
    const secondDetail = deferredResponse();

    vi.mocked(globalThis.fetch).mockImplementation((input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const method = init?.method ?? 'GET';

      if (url.endsWith('/api/audit') && method === 'GET') {
        return Promise.resolve(
          jsonResponse({
            data: [
              {
                id: 'log-1',
                actor: 'local-admin',
                action: 'pause',
                resource: 'system',
                resource_id: 'system-1',
                result: 'failure',
                message: '流程暂停',
                metadata: { reason: 'manual' },
                created_at: '2026-05-07T03:00:00Z',
              },
              {
                id: 'log-2',
                actor: 'workflow-bot',
                action: 'resume',
                resource: 'article',
                resource_id: 'article-2',
                result: 'success',
                message: '任务恢复',
                metadata: { source: 'scheduler' },
                created_at: '2026-05-07T04:00:00Z',
              },
            ],
          }),
        );
      }

      if (url.endsWith('/api/audit/log-1') && method === 'GET') {
        return firstDetail.promise;
      }

      if (url.endsWith('/api/audit/log-2') && method === 'GET') {
        return secondDetail.promise;
      }

      throw new Error(`Unhandled fetch: ${method} ${url}`);
    });

    renderAuditPage();

    expect(await screen.findByText('流程暂停')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('row', { name: /log-1/i }));
      fireEvent.click(screen.getByRole('row', { name: /log-2/i }));
    });

    await act(async () => {
      secondDetail.resolve(
        jsonResponse({
          id: 'log-2',
          actor: 'workflow-bot',
          action: 'resume',
          resource: 'article',
          resource_id: 'article-2',
          result: 'success',
          message: '任务恢复',
          metadata: { source: 'scheduler', operator: 'B-02' },
          created_at: '2026-05-07T04:00:00Z',
        }),
      );
    });

    expect(await screen.findByText('article / article-2')).toBeInTheDocument();
    expect(screen.getByDisplayValue(/"operator": "B-02"/)).toBeInTheDocument();

    await act(async () => {
      firstDetail.resolve(
        jsonResponse({
          id: 'log-1',
          actor: 'local-admin',
          action: 'pause',
          resource: 'system',
          resource_id: 'system-1',
          result: 'failure',
          message: '流程暂停',
          metadata: { reason: 'manual', operator: 'A-01' },
          created_at: '2026-05-07T03:00:00Z',
        }),
      );
    });

    expect(screen.getByRole('row', { name: /log-2/i })).toHaveClass('Mui-selected');
    expect(screen.getByText('article / article-2')).toBeInTheDocument();
    expect(screen.getByDisplayValue(/"operator": "B-02"/)).toBeInTheDocument();
    expect(screen.queryByText('system / system-1')).not.toBeInTheDocument();
    expect(screen.queryByDisplayValue(/"operator": "A-01"/)).not.toBeInTheDocument();
  });

  it('clears stale detail when filters reduce the visible list to zero rows', async () => {
    renderAuditPage();

    expect(await screen.findByRole('row', { name: /log-1/i })).toHaveClass('Mui-selected');
    expect(screen.getByText('system / system-1')).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText('搜索操作人、资源、动作或详情'), {
      target: { value: 'not-found-keyword' },
    });

    expect(await screen.findByText('暂无审计记录')).toBeInTheDocument();
    const detailCard = screen.getByTestId('audit-detail-card');
    expect(await within(detailCard).findByTestId('page-state-empty')).toBeInTheDocument();
    expect(within(detailCard).getByText('请选择一条记录查看审计详情。')).toBeInTheDocument();
    expect(within(detailCard).queryByText('system / system-1')).not.toBeInTheDocument();
    expect(within(detailCard).queryByDisplayValue(/"operator": "A-01"/)).not.toBeInTheDocument();
  });
});
