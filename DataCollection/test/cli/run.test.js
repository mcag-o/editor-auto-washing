import { describe, expect, test } from 'vitest';
import path from 'node:path';
import { parseArgs, writeSourceOutputs } from '../../src/cli/run.js';

describe('parseArgs', () => {
  test('parses collect-many arguments', () => {
    const parsed = parseArgs(['--platforms', 'baidu,weibo']);
    expect(parsed.platforms).toEqual(['baidu', 'weibo']);
    expect(parsed.outputDir).toBe(null);
    expect(parsed.configPath).toBe(null);
    expect(parsed.noOutput).toBe(false);
  });

  test('parses --bundle-out argument', () => {
    const parsed = parseArgs(['--platform', 'weibo', '--output-dir', './out']);
    expect(parsed.platforms).toEqual(['weibo']);
    expect(parsed.outputDir).toBe('./out');
  });

  test('parses config path and output alias', () => {
    const parsed = parseArgs(['--config', './config/sources.yaml', '--output', './tmp/out']);
    expect(parsed.configPath).toBe('./config/sources.yaml');
    expect(parsed.outputDir).toBe('./tmp/out');
  });

  test('parses no-output flag', () => {
    const parsed = parseArgs(['--all', '--no-output']);
    expect(parsed.useEnabledAll).toBe(true);
    expect(parsed.noOutput).toBe(true);
  });
});

describe('writeBundleOutput', () => {
  test('writes one source file per platform using timestamp filename', async () => {
    const writes = [];
    const writeFileImpl = async (filePath, payload, encoding) => {
      writes.push({ filePath, payload, encoding });
    };
    const mkdirCalls = [];
    const mkdirImpl = async (dirPath) => {
      mkdirCalls.push(dirPath);
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
        },
        {
          platform: 'bilibili',
          canonicalPlatform: 'bilibili',
          sourceType: 'json-api',
          displayName: '哔哩哔哩',
          sourceUrl: 'https://api.bilibili.com/x/web-interface/popular',
          fetchedAt: '2026-04-07T08:29:05.000Z',
          success: true,
          items: [],
          warnings: []
        }
      ]
    };

    const outputDir = '/tmp/swap/output';
    const files = await writeSourceOutputs(outputDir, result, { writeFileImpl, mkdirImpl });

    expect(mkdirCalls).toEqual([outputDir]);
    expect(writes).toHaveLength(2);
    expect(writes[0].encoding).toBe('utf8');
    expect(writes[0].payload.endsWith('\n')).toBe(true);
    expect(path.basename(writes[0].filePath)).toBe('weibo-20260407T083000000Z.json');
    expect(path.basename(writes[1].filePath)).toBe('bilibili-20260407T083000000Z.json');
    expect(files).toHaveLength(2);
  });
});
