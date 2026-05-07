import { CssBaseline, ThemeProvider } from '@mui/material';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ArticlesPage from './ArticlesPage';
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

function renderArticlesPage() {
  return render(
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <ArticlesPage />
    </ThemeProvider>,
  );
}

describe('ArticlesPage', () => {
  beforeEach(() => {
    vi.spyOn(globalThis, 'fetch').mockImplementation((input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const method = init?.method ?? 'GET';

      if (url.endsWith('/api/articles') && method === 'GET') {
        return Promise.resolve(jsonResponse({ data: [] }));
      }

      throw new Error(`Unhandled fetch: ${method} ${url}`);
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('shows an explicit loading state while the article list is loading', async () => {
    const pendingList = deferredResponse();

    vi.mocked(globalThis.fetch).mockImplementation((input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const method = init?.method ?? 'GET';

      if (url.endsWith('/api/articles') && method === 'GET') {
        return pendingList.promise;
      }

      throw new Error(`Unhandled fetch: ${method} ${url}`);
    });

    renderArticlesPage();

    expect(screen.getByRole('heading', { name: '文章列表' })).toBeInTheDocument();
    expect(within(screen.getByTestId('articles-list-card')).getByTestId('page-state-loading')).toBeInTheDocument();
  });

  it('shows an explicit empty state when no articles are returned', async () => {
    renderArticlesPage();

    const listCard = await screen.findByTestId('articles-list-card');
    expect(within(listCard).getByTestId('page-state-empty')).toBeInTheDocument();
    expect(within(listCard).getByText('暂无文章记录')).toBeInTheDocument();

    const detailCard = screen.getByTestId('articles-detail-card');
    expect(within(detailCard).getByTestId('page-state-empty')).toBeInTheDocument();
    expect(within(detailCard).getByText('请选择一篇文章查看阶段详情。')).toBeInTheDocument();
  });

  it('shows an explicit error state without collapsing the page layout', async () => {
    vi.mocked(globalThis.fetch).mockImplementation((input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const method = init?.method ?? 'GET';

      if (url.endsWith('/api/articles') && method === 'GET') {
        return Promise.resolve(
          new Response(JSON.stringify({ message: '文章列表加载失败，请刷新后重试。' }), {
            status: 500,
            headers: { 'Content-Type': 'application/json' },
          }),
        );
      }

      throw new Error(`Unhandled fetch: ${method} ${url}`);
    });

    renderArticlesPage();

    expect(await screen.findByTestId('page-state-error')).toBeInTheDocument();
    expect(screen.getByText('文章列表加载失败，请刷新后重试。')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '队列表格' })).toBeInTheDocument();
    expect(screen.getByTestId('articles-list-card')).toBeInTheDocument();
    expect(screen.getByTestId('articles-detail-card')).toBeInTheDocument();
  });

  it('surfaces the detail API error message in the stage detail panel', async () => {
    vi.mocked(globalThis.fetch).mockImplementation((input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const method = init?.method ?? 'GET';

      if (url.endsWith('/api/articles') && method === 'GET') {
        return Promise.resolve(
          jsonResponse({
            data: [
              {
                id: 'article-1',
                source_type: 'upload',
                original_filename: 'a.md',
                original_path: '',
                archived_path: '',
                file_type: 'markdown',
                title: '测试文章',
                body: '正文内容',
                summary: '',
                metadata: {},
                hash: 'hash-1',
                imported_at: '2026-05-07T03:00:00Z',
                status: 'completed',
                workspace_article_id: '',
                rewrite_run_id: '',
                claimed_by: '',
                claimed_at: null,
                processing_started_at: null,
                completed_at: '2026-05-07T03:10:00Z',
                error_summary: '',
              },
            ],
          }),
        );
      }

      if (url.endsWith('/api/articles/article-1/stages') && method === 'GET') {
        return Promise.resolve(
          new Response(JSON.stringify({ message: '阶段详情加载失败：运行记录不存在。' }), {
            status: 404,
            headers: { 'Content-Type': 'application/json' },
          }),
        );
      }

      throw new Error(`Unhandled fetch: ${method} ${url}`);
    });

    renderArticlesPage();

    expect(await screen.findByText('测试文章')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('row', { name: /article-1/i }));

    const detailCard = screen.getByTestId('articles-detail-card');
    expect(await within(detailCard).findByText('阶段详情加载失败：运行记录不存在。')).toBeInTheDocument();
  });
});
