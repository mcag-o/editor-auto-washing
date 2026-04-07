import { describe, expect, test } from 'vitest';
import { collectPlatform } from '../../src/scheduler/collectPlatform.js';

describe('collectPlatform', () => {
  test('returns structured unsupported-platform errors', async () => {
    const result = await collectPlatform('unknown', { registry: {} });
    expect(result.success).toBe(false);
    expect(result.error.code).toBe('UNSUPPORTED_PLATFORM');
  });

  test('uses configured source metadata overrides', async () => {
    const registry = {
      baidu: {
        collect: async ({ platform, canonicalPlatform, meta }) => ({
          platform,
          canonicalPlatform,
          aliases: meta.aliases,
          displayName: meta.displayName,
          sourceType: meta.sourceType,
          sourceUrl: meta.sourceUrl,
          fetchedAt: new Date().toISOString(),
          success: true,
          items: [],
          warnings: []
        })
      }
    };

    const result = await collectPlatform('baidu', {
      registry,
      metaById: {
        baidu: {
          displayName: '百度热搜(配置覆盖)',
          aliases: ['baidu'],
          sourceType: 'json-api',
          sourceUrl: 'https://example.com/custom-baidu-api'
        }
      }
    });

    expect(result.success).toBe(true);
    expect(result.displayName).toBe('百度热搜(配置覆盖)');
    expect(result.sourceUrl).toBe('https://example.com/custom-baidu-api');
  });
});
