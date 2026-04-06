import { describe, expect, test } from 'vitest';
import { createShaoshupaiCrawler } from '../../src/platforms/shaoshupai.js';
import { createWeiboCrawler } from '../../src/platforms/weibo.js';

describe('json platform crawlers', () => {
  test('normalize two representative APIs', async () => {
    const shaoshupai = createShaoshupaiCrawler({
      requestJson: async () => ({ data: [{ id: 1, title: 'A', summary: 'S' }] })
    });
    const weibo = createWeiboCrawler({
      requestJson: async () => ({ data: { realtime: [{ word: '热搜词', raw_hot: 123 }] } })
    });

    const s1 = await shaoshupai.collect({
      platform: 'shaoshupai',
      canonicalPlatform: 'shaoshupai',
      meta: { displayName: '少数派', aliases: ['shaoshupai', 'sspai'] }
    });
    const s2 = await weibo.collect({
      platform: 'weibo',
      canonicalPlatform: 'weibo',
      meta: { displayName: '微博热搜', aliases: ['weibo'] }
    });

    expect(s1.items[0].title).toBe('A');
    expect(s2.items[0].title).toBe('热搜词');
  });
});
