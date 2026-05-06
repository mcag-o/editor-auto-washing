import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import WorkflowEdgePanel from './WorkflowEdgePanel';

describe('WorkflowEdgePanel', () => {
  it('renders the preserved edge condition for the selected edge', () => {
    render(
      <WorkflowEdgePanel
        selectedEdge={{
          id: 'edge-node-1-node-2',
          sourceLabel: '开始节点',
          targetLabel: '重试节点',
          condition: 'retry',
        }}
        onDeleteEdge={vi.fn()}
      />, 
    );

    expect(screen.getByText('条件分支')).toBeInTheDocument();
    expect(screen.getByText('retry')).toBeInTheDocument();
  });
});
