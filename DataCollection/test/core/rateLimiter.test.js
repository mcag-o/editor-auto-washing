import { describe, expect, test } from 'vitest';
import { createRateLimiter } from '../../src/core/rateLimiter.js';

describe('createRateLimiter', () => {
  test('waits between calls for the same key', async () => {
    const limiter = createRateLimiter({ minIntervalMs: 20 });
    const start = Date.now();

    await limiter.wait('baidu');
    await limiter.wait('baidu');

    expect(Date.now() - start).toBeGreaterThanOrEqual(20);
  });
});
