import { describe, expect, test } from 'vitest';
import { buildContentHubBundle } from '../../src/integration/contentHubBundle.js';

describe('buildContentHubBundle', () => {
  test('maps successful collectMany results into normalized bundle items', () => {
    const collectManyResult = {
      finishedAt: '2026-04-07T08:30:00.000Z',
      results: [
        {
          platform: 'weibo',
          canonicalPlatform: 'weibo',
          aliases: ['weibo'],
          displayName: '微博热搜',
          sourceType: 'json-api',
          sourceUrl: 'https://weibo.com/ajax/side/hotSearch',
          fetchedAt: '2026-04-07T08:29:00.000Z',
          success: true,
          items: [
            {
              id: 'wb-1',
              rank: 1,
              title: '热点标题',
              url: 'https://weibo.com/item/1',
              mobileUrl: null,
              hot: '12345',
              summary: '摘要',
              author: '作者',
              category: '社会',
              tags: ['热搜', '微博'],
              publishTime: '2026-04-07T08:20:00.000Z',
              metadata: { channel: 'realtime' },
              raw: { topic: 'raw-topic' }
            }
          ],
          warnings: []
        }
      ]
    };

    const bundle = buildContentHubBundle(collectManyResult);

    expect(bundle.bundleVersion).toBe('1.0');
    expect(bundle.generatedAt).toBe('2026-04-07T08:30:00.000Z');
    expect(bundle.sources).toHaveLength(1);
    expect(bundle.sources[0]).toMatchObject({
      sourceType: 'json-api',
      platform: 'weibo',
      canonicalPlatform: 'weibo',
      success: true,
      itemCount: 1,
      fetchedAt: '2026-04-07T08:29:00.000Z'
    });

    expect(bundle.items).toHaveLength(1);
    expect(bundle.items[0]).toEqual({
      sourceType: 'json-api',
      platform: 'weibo',
      canonicalPlatform: 'weibo',
      title: '热点标题',
      url: 'https://weibo.com/item/1',
      summary: '摘要',
      author: '作者',
      publishTime: '2026-04-07T08:20:00.000Z',
      tags: ['热搜', '微博'],
      category: '社会',
      rank: 1,
      hot: '12345',
      metadata: { channel: 'realtime' },
      raw: { topic: 'raw-topic' }
    });
    expect(bundle.failures).toEqual([]);
  });

  test('maps failed collectMany results into failures and source metadata', () => {
    const collectManyResult = {
      results: [
        {
          platform: 'zhihu-hot',
          canonicalPlatform: 'zhihu',
          aliases: ['zhihu', 'zhihu-hot'],
          displayName: '知乎热榜',
          sourceType: 'html',
          sourceUrl: 'https://www.zhihu.com/hot',
          fetchedAt: '2026-04-07T08:31:00.000Z',
          success: false,
          items: [],
          warnings: ['timeout'],
          error: {
            code: 'UPSTREAM_TIMEOUT',
            message: 'request timed out',
            retryable: true
          }
        }
      ]
    };

    const bundle = buildContentHubBundle(collectManyResult);

    expect(bundle.bundleVersion).toBe('1.0');
    expect(typeof bundle.generatedAt).toBe('string');
    expect(bundle.sources).toHaveLength(1);
    expect(bundle.sources[0]).toMatchObject({
      sourceType: 'html',
      platform: 'zhihu-hot',
      canonicalPlatform: 'zhihu',
      success: false,
      itemCount: 0,
      warnings: ['timeout'],
      error: {
        code: 'UPSTREAM_TIMEOUT',
        message: 'request timed out',
        retryable: true
      }
    });
    expect(bundle.items).toEqual([]);
    expect(bundle.failures).toEqual([
      {
        sourceType: 'html',
        platform: 'zhihu-hot',
        canonicalPlatform: 'zhihu',
        displayName: '知乎热榜',
        sourceUrl: 'https://www.zhihu.com/hot',
        fetchedAt: '2026-04-07T08:31:00.000Z',
        warnings: ['timeout'],
        error: {
          code: 'UPSTREAM_TIMEOUT',
          message: 'request timed out',
          retryable: true
        }
      }
    ]);
  });
});
