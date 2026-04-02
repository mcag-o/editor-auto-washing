import { describe, expect, test } from 'vitest';
import { normalizePlatformId, getPlatformMeta } from '../../src/platforms/aliases/platformRegistry.js';

describe('platform registry', () => {
  test('maps aliases to canonical ids', () => {
    expect(normalizePlatformId('sspai')).toBe('shaoshupai');
    expect(normalizePlatformId('tskr')).toBe('36kr');
    expect(normalizePlatformId('ftpojie')).toBe('52pojie');
    expect(normalizePlatformId('vtex')).toBe('v2ex');
    expect(getPlatformMeta('weibo').displayName).toBe('微博热搜');
  });
});
