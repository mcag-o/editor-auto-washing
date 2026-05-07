import { CssBaseline, ThemeProvider } from '@mui/material';
import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ControlPage from './ControlPage';
import theme from '../../theme/theme';

function jsonResponse(body: unknown, init: ResponseInit = {}) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
}

function renderControlPage() {
  return render(
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <ControlPage />
    </ThemeProvider>,
  );
}

describe('ControlPage', () => {
  beforeEach(() => {
    vi.spyOn(globalThis, 'fetch').mockImplementation((input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
      const method = init?.method ?? 'GET';

      if (url.endsWith('/api/system/status') && method === 'GET') {
        return Promise.resolve(
          jsonResponse({
            id: 'system-1',
            state: 'paused',
            reason: 'manual pause',
            metadata: { concurrency_limit: 3 },
            updated_by: 'operator-a',
            requested_at: '2026-05-07T03:00:00Z',
            updated_at: '2026-05-07T03:05:00Z',
          }),
        );
      }

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
                title: '测试文章 1',
                body: '正文内容',
                summary: '',
                metadata: {},
                hash: 'hash-1',
                imported_at: '2026-05-07T03:00:00Z',
                status: 'pending',
                workspace_article_id: '',
                rewrite_run_id: '',
                claimed_by: '',
                claimed_at: null,
                processing_started_at: null,
                completed_at: null,
                error_summary: '',
              },
              {
                id: 'article-2',
                source_type: 'upload',
                original_filename: 'b.md',
                original_path: '',
                archived_path: '',
                file_type: 'markdown',
                title: '测试文章 2',
                body: '正文内容',
                summary: '',
                metadata: {},
                hash: 'hash-2',
                imported_at: '2026-05-07T03:00:00Z',
                status: 'failed',
                workspace_article_id: '',
                rewrite_run_id: '',
                claimed_by: '',
                claimed_at: null,
                processing_started_at: null,
                completed_at: null,
                error_summary: 'stage failed',
              },
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

  it('uses clearer control wording and separates state, concurrency, queue, and actions', async () => {
    renderControlPage();

    expect(await screen.findByRole('heading', { name: '流程控制' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '运行状态与控制' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '队列与并发' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '启动主链路' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '提交暂停请求' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '恢复已暂停主链路' })).toBeEnabled();
    expect(screen.getByText('启动会按当前并发上限拉起主链路，仅对未启动状态生效。')).toBeInTheDocument();
    expect(screen.getByText('暂停会提交协作暂停请求，不会强制中断已在执行中的任务。')).toBeInTheDocument();
    expect(screen.getByText('恢复只对已暂停状态生效，会继续处理当前待处理队列。')).toBeInTheDocument();
  });
});
