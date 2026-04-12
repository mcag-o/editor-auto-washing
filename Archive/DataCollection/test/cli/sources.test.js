import { describe, expect, test } from 'vitest';
import { createSourceStatusRows } from '../../src/cli/sources.js';

describe('createSourceStatusRows', () => {
  test('maps source configs into display rows', () => {
    const rows = createSourceStatusRows({
      sources: [
        {
          id: 'weibo',
          enabled: false,
          sourceType: 'json-api',
          sourceUrl: 'https://weibo.com/ajax/side/hotSearch'
        }
      ]
    });

    expect(rows).toEqual([
      {
        id: 'weibo',
        enabled: 'off',
        sourceType: 'json-api',
        status: 'configured',
        sourceUrl: 'https://weibo.com/ajax/side/hotSearch'
      }
    ]);
  });
});
