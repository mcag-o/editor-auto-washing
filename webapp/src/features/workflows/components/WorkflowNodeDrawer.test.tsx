import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import WorkflowNodeDrawer from './WorkflowNodeDrawer';

describe('WorkflowNodeDrawer node type contract', () => {
  it('shows unknown backend node types without coercing them away', () => {
    render(
      <WorkflowNodeDrawer
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
