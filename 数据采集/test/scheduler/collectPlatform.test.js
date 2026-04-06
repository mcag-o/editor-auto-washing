import { describe, expect, test } from 'vitest';
import { collectPlatform } from '../../src/scheduler/collectPlatform.js';

describe('collectPlatform', () => {
  test('returns structured unsupported-platform errors', async () => {
    const result = await collectPlatform('unknown', { registry: {} });
    expect(result.success).toBe(false);
    expect(result.error.code).toBe('UNSUPPORTED_PLATFORM');
  });
});
