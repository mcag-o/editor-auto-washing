/**
 * Create a per-key rate limiter.
 * @param {{ minIntervalMs: number }} options
 * @returns {{ wait(key: string): Promise<void> }}
 */
export function createRateLimiter({ minIntervalMs }) {
  const timestamps = new Map();

  return {
    async wait(key) {
      const now = Date.now();
      const previous = timestamps.get(key) ?? 0;
      const delta = now - previous;

      if (delta < minIntervalMs) {
        await new Promise((resolve) => setTimeout(resolve, minIntervalMs - delta));
      }

      timestamps.set(key, Date.now());
    }
  };
}
