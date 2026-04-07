import { describe, expect, test, vi } from 'vitest';
import { requestJson } from '../../src/core/httpClient.js';

describe('requestJson', () => {
  test('retries retryable failures once before succeeding', async () => {
    const responses = [
      { ok: false, status: 502 },
      { ok: true, status: 200, json: async () => ({ ok: true }) }
    ];
    const fetchImpl = vi.fn().mockImplementation(async () => {
      const response = responses.shift();
      if (!response.ok) {
        return {
          ok: false,
          status: response.status,
          json: async () => ({})
        };
      }

      return response;
    });

    const result = await requestJson('https://example.com', {
      fetchImpl,
      retryCount: 1,
      retryBaseMs: 1,
      timeoutMs: 1000
    });

    expect(result).toEqual({ ok: true });
    expect(fetchImpl).toHaveBeenCalledTimes(2);
  });
});
