import { describe, expect, test } from 'vitest';
import { loadEnv } from '../../src/config/env.js';

describe('loadEnv', () => {
  test('returns normalized defaults', () => {
    const current = loadEnv({});

    expect(current.httpTimeoutMs).toBe(10000);
    expect(current.httpRetryCount).toBe(2);
    expect(current.globalConcurrency).toBe(4);
    expect(current.enableBrowserFallback).toBe(false);
  });
});
