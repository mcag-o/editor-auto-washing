import { describe, expect, test } from 'vitest';
import { collectMany } from '../../src/scheduler/collectMany.js';

describe('collectMany', () => {
  test('aggregates success and failure results without aborting', async () => {
    const registry = {
      baidu: { collect: async () => ({ platform: 'baidu', canonicalPlatform: 'baidu', aliases: ['baidu'], displayName: '百度热搜', sourceType: 'json-api', sourceUrl: 'https://top.baidu.com/api/board?platform=wise&tab=realtime', fetchedAt: new Date().toISOString(), success: true, items: [], warnings: [] }) },
      weibo: { collect: async () => ({ platform: 'weibo', canonicalPlatform: 'weibo', aliases: ['weibo'], displayName: '微博热搜', sourceType: 'json-api', sourceUrl: 'https://weibo.com/ajax/side/hotSearch', fetchedAt: new Date().toISOString(), success: false, items: [], warnings: ['403'], error: { code: 'UPSTREAM_HTTP_ERROR', message: '403', retryable: false } }) }
    };

    const result = await collectMany(['baidu', 'weibo'], { registry });

    expect(result.successCount).toBe(1);
    expect(result.failureCount).toBe(1);
    expect(result.results).toHaveLength(2);
  });
});
