import type { ReactNode } from 'react';
import { CssBaseline, ThemeProvider } from '@mui/material';
import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import PageCard from './PageCard';
import PageToolbar from './PageToolbar';
import StatusChip from './StatusChip';
import theme from '../theme/theme';

function renderWithTheme(node: ReactNode) {
  return render(
    <ThemeProvider theme={theme}>
      <CssBaseline />
      {node}
    </ThemeProvider>,
  );
}

describe('page primitives', () => {
  it('groups toolbar actions and filters with accessible labels', () => {
    renderWithTheme(
      <PageToolbar
        title="文章列表"
        description="队列说明"
        actions={<button type="button">新增导入</button>}
        filters={<div>筛选条件</div>}
      />,
    );

    expect(screen.getByRole('heading', { name: '文章列表' })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: '文章列表页面操作' })).toBeInTheDocument();
    expect(screen.getByRole('region', { name: '文章列表筛选' })).toBeInTheDocument();
  });

  it('wraps card header actions in a dedicated action group', () => {
    renderWithTheme(
      <PageCard title="队列表格" description="展示当前内容" action={<button type="button">刷新</button>}>
        <div>卡片正文</div>
      </PageCard>,
    );

    const actionGroup = screen.getByRole('group', { name: '队列表格操作' });
    expect(within(actionGroup).getByRole('button', { name: '刷新' })).toBeInTheDocument();
  });

  it('exposes the normalized status through a data attribute', () => {
    renderWithTheme(<StatusChip status="failed" />);

    expect(screen.getByText('失败').closest('[data-status="failed"]')).toBeInTheDocument();
  });
});
