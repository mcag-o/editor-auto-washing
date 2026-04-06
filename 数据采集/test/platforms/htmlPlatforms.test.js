import { describe, expect, test } from 'vitest';
import fs from 'node:fs/promises';
import { createHackernewsCrawler } from '../../src/platforms/hackernews.js';

describe('html crawlers', () => {
  test('parses hacker news rows from html', async () => {
    const html = await fs.readFile(new URL('../fixtures/hackernews.html', import.meta.url), 'utf8');
    const crawler = createHackernewsCrawler({ requestText: async () => html, browserEnabled: false });

    const result = await crawler.collect({
      platform: 'hackernews',
      canonicalPlatform: 'hackernews',
      meta: { displayName: 'Hacker News', aliases: ['hackernews'] }
    });

    expect(result.items[0].title).toBe('Example HN Post');
    expect(result.items[0].author).toBe('alice');
  });
});
