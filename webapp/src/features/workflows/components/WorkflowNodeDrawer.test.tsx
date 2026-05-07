import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import WorkflowNodeDrawer from './WorkflowNodeDrawer';

describe('WorkflowNodeDrawer node type contract', () => {
  it('shows loading state even when no node value is present yet', () => {
    render(<WorkflowNodeDrawer loading entryNodeLabel="审核入口" selectedNodeId={null} value={null} onChange={vi.fn()} />);

    expect(screen.getByTestId('page-state-loading')).toBeInTheDocument();
    expect(screen.getByText('正在加载节点配置')).toBeInTheDocument();
    expect(screen.queryByTestId('page-state-empty')).not.toBeInTheDocument();
  });

  it('shows an explicit idle inspector state when no node is selected', () => {
    render(<WorkflowNodeDrawer loading={false} entryNodeLabel="审核入口" selectedNodeId={null} value={null} onChange={vi.fn()} />);

    expect(screen.getByText('工作流检查器')).toBeInTheDocument();
    expect(screen.getByTestId('page-state-empty')).toBeInTheDocument();
    expect(screen.getByText('请先在画布中选择一个节点或连线。')).toBeInTheDocument();
  });

  it('shows node inspector context while preserving grouped editing tabs', () => {
    render(
      <WorkflowNodeDrawer
        loading={false}
        entryNodeLabel="审核入口"
        selectedNodeId="node-42"
        value={{
          label: 'Moderation',
          type: 'rewrite',
          rawType: 'moderate',
          template: 'safety.template',
          model: 'gpt-safety',
          context: 'Preserve me',
        }}
        onChange={vi.fn()}
      />,
    );

    expect(screen.getByText('节点检查器')).toBeInTheDocument();
    expect(screen.getByText('当前节点')).toBeInTheDocument();
    expect(screen.getByText('node-42')).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '基础信息' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '模板绑定' })).toBeInTheDocument();
  });

  it('shows unknown backend node types without coercing them away', () => {
    render(
      <WorkflowNodeDrawer
        loading={false}
        entryNodeLabel="审核入口"
        selectedNodeId="node-42"
        value={{
          label: 'Moderation',
          type: 'rewrite',
          rawType: 'moderate',
          template: 'safety.template',
          model: 'gpt-safety',
          context: 'Preserve me',
        }}
        onChange={vi.fn()}
      />,
    );

    expect(screen.getByLabelText('节点类型')).toHaveValue('moderate');
    expect(screen.getByText('保留当前后端类型值，保存时会原样写入节点 config_json。')).toBeInTheDocument();
  });
});
