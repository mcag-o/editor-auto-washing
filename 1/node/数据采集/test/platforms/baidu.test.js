import { describe, expect, test } from 'vitest';
import fs from 'node:fs/promises';
import { createBaiduCrawler } from '../../src/platforms/baidu.js';

describe('createBaiduCrawler', () => {
  test('normalizes baidu upstream payload into standard items', async () => {
    const fixtureText = await fs.readFile(new URL('../fixtures/baidu.json', import.meta.url), 'utf8');
    const fixture = JSON.parse(fixtureText);
    const crawler = createBaiduCrawler({ requestJson: async () => fixture });

    const result = await crawler.collect({
      platform: 'baidu',
      canonicalPlatform: 'baidu',
      meta: { displayName: '百度热搜', aliases: ['baidu'] }
    });

    expect(result.success).toBe(true);
    expect(result.items[0].title).toBe('在雄安 为幸福加码');
    expect(result.items[0].rank).toBe(1);
    expect(result.items[0].url).toContain('www.baidu.com');
  });
});
