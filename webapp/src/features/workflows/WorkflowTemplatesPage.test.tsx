import { describe, expect, it } from 'vitest';
import { createLocalWorkflowEdge } from './WorkflowTemplatesPage';

describe('WorkflowTemplatesPage local edge helpers', () => {
  it('creates a new same-pair edge id that does not collide after delete and reconnect', () => {
    const survivingEdge = createLocalWorkflowEdge({
      source: 'node-1',
      target: 'node-2',
      condition: 'always',
      priority: 1,
    });

    const reconnectedEdge = createLocalWorkflowEdge({
      source: 'node-1',
      target: 'node-2',
      condition: 'always',
      priority: 1,
    });

    expect(reconnectedEdge.id).not.toBe(survivingEdge.id);
  });
});
