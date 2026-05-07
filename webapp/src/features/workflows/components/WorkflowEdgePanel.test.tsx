import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import WorkflowEdgePanel from './WorkflowEdgePanel';

describe('WorkflowEdgePanel', () => {
  it('shows an explicit idle inspector state when no edge is selected', () => {
    render(<WorkflowEdgePanel selectedEdge={null} onDeleteEdge={vi.fn()} onChange={vi.fn()} />);

    expect(screen.getByText('工作流检查器')).toBeInTheDocument();
    expect(screen.getByTestId('page-state-empty')).toBeInTheDocument();
    expect(screen.getByText('请先在画布中选择一个节点或连线。')).toBeInTheDocument();
  });

  it('renders the preserved edge condition for the selected edge', () => {
    render(
      <WorkflowEdgePanel
        selectedEdge={{
          id: 'edge-node-1-node-2',
          sourceLabel: '开始节点',
          targetLabel: '重试节点',
          condition: 'retry',
          priority: 3,
        }}
        onDeleteEdge={vi.fn()}
        onChange={vi.fn()}
      />, 
    );

    expect(screen.getByText('连线检查器')).toBeInTheDocument();
    expect(screen.getByDisplayValue('retry')).toBeInTheDocument();
    expect(screen.getByDisplayValue('3')).toBeInTheDocument();
  });

  it('allows editing edge condition and priority', () => {
    const handleChange = vi.fn();

    render(
      <WorkflowEdgePanel
        selectedEdge={{
          id: 'edge-node-1-node-2',
          sourceLabel: '开始节点',
          targetLabel: '重试节点',
          condition: 'retry',
          priority: 3,
        }}
        onDeleteEdge={vi.fn()}
        onChange={handleChange}
      />,
    );

    fireEvent.change(screen.getByLabelText('条件分支'), { target: { value: 'fallback' } });
    fireEvent.change(screen.getByLabelText('优先级'), { target: { value: '7' } });

    expect(handleChange).toHaveBeenNthCalledWith(1, 'condition', 'fallback');
    expect(handleChange).toHaveBeenNthCalledWith(2, 'priority', 7);
  });
});
