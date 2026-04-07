import { describe, expect, test } from 'vitest';
import { parseArgs, writeBundleOutput } from '../../src/cli/run.js';

describe('parseArgs', () => {
  test('parses collect-many arguments', () => {
    const parsed = parseArgs(['--platforms', 'baidu,weibo']);
    expect(parsed.platforms).toEqual(['baidu', 'weibo']);
    expect(parsed.bundleOut).toBe(null);
  });

  test('parses --bundle-out argument', () => {
    const parsed = parseArgs(['--platform', 'weibo', '--bundle-out', './out/content-hub.bundle.json']);
    expect(parsed.platforms).toEqual(['weibo']);
    expect(parsed.bundleOut).toBe('./out/content-hub.bundle.json');
  });
});

describe('writeBundleOutput', () => {
  test('builds bundle json and writes newline-terminated utf8 output', async () => {
    const writes = [];
    const writeFileImpl = async (filePath, payload, encoding) => {
      writes.push({ filePath, payload, encoding });
    };

    const result = {
      finishedAt: '2026-04-07T08:30:00.000Z',
      results: [
        {
          platform: 'weibo',
          canonicalPlatform: 'weibo',
          sourceType: 'json-api',
          displayName: '微博热搜',
          sourceUrl: 'https://weibo.com/ajax/side/hotSearch',
          fetchedAt: '2026-04-07T08:29:00.000Z',
          success: true,
          items: [
            {
              title: '热点标题',
              url: 'https://weibo.com/item/1',
              tags: ['热搜']
            }
          ],
          warnings: []
        }
      ]
    };

    const bundle = await writeBundleOutput('./tmp/content-hub.bundle.json', result, { writeFileImpl });

    expect(writes).toHaveLength(1);
    expect(writes[0].filePath).toBe('./tmp/content-hub.bundle.json');
    expect(writes[0].encoding).toBe('utf8');
    expect(writes[0].payload.endsWith('\n')).toBe(true);
    expect(JSON.parse(writes[0].payload)).toEqual(bundle);
    expect(bundle.generatedAt).toBe('2026-04-07T08:30:00.000Z');
    expect(bundle.items).toHaveLength(1);
  });
});
