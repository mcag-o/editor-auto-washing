import Divider from '@mui/material/Divider';
import type { ReactNode } from 'react';
import PageCard from '../../../components/PageCard';
import PageState from '../../../components/PageState';

type WorkflowInspectorMode = 'idle' | 'node' | 'edge';

type WorkflowInspectorPanelProps = {
  mode: WorkflowInspectorMode;
  title: string;
  description: string;
  action?: ReactNode;
  children?: ReactNode;
};

export default function WorkflowInspectorPanel({ mode, title, description, action, children }: WorkflowInspectorPanelProps) {
  return (
    <PageCard title={title} description={description} action={action}>
      {mode === 'idle' ? (
        <PageState title="等待选择" description="请先在画布中选择一个节点或连线。" tone="empty" />
      ) : (
        <>
          <Divider />
          {children}
        </>
      )}
    </PageCard>
  );
}
