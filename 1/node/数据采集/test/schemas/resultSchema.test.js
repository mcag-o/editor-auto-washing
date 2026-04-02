import { describe, expect, test } from 'vitest';
import { resultSchema } from '../../src/schemas/resultSchema.js';

describe('resultSchema', () => {
  test('accepts normalized result payloads', () => {
    const parsed = resultSchema.parse({
      platform: 'baidu',
      canonicalPlatform: 'baidu',
      aliases: ['baidu'],
      displayName: '百度热搜',
      sourceType: 'json-api',
      sourceUrl: 'https://top.baidu.com/api/board?platform=wise&tab=realtime',
      fetchedAt: '2026-04-02T21:00:00.000Z',
      success: true,
      items: [],
      warnings: []
    });

    expect(parsed.success).toBe(true);
  });
});
