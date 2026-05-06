import { CssBaseline, ThemeProvider } from '@mui/material';
import { act } from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import TemplatesPage from './TemplatesPage';
import { parseTemplateStages } from '../../lib/mappers/template';
import theme from '../../theme/theme';

function renderTemplatesPage() {
  return render(
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <TemplatesPage />
    </ThemeProvider>,
  );
}

describe('TemplatesPage', () => {
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

  it('renders a local template management shell with table, preview, and drawer interactions', async () => {
    renderTemplatesPage();

    expect(screen.getByRole('heading', { name: '模板管理' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '模板列表' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '内容预览' })).toBeInTheDocument();
    expect(screen.getByText('当前页面仅演示本地模板交互，不会请求后端接口。')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('row', { name: /多语气社媒扩写/i }));
    });
    expect(screen.getByText('更新人 内容策略组')).toBeInTheDocument();
    expect(screen.getByText('模板类型')).toBeInTheDocument();
    expect(screen.getByText('3 个阶段')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('row', { name: /品牌改写主模板/i }));
    });
    expect(screen.getByText('更新人 运营编辑组')).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '新建模板' }));
    });
    expect(screen.getByRole('heading', { name: '新建模板' })).toBeInTheDocument();
    expect(screen.getByLabelText('模板名称')).toHaveValue('新模板');

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '关闭' }));
    });

    await act(async () => {
      fireEvent.click(screen.getAllByRole('button', { name: '操作' })[0]);
    });
    await act(async () => {
      fireEvent.click(screen.getByRole('menuitem', { name: '编辑模板' }));
    });

    expect(screen.getByRole('heading', { name: '编辑模板' })).toBeInTheDocument();
    expect(screen.getByLabelText('模板名称')).toHaveValue('品牌改写主模板');
  });
});
