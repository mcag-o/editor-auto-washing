import { createConcurrencyPool } from '../core/concurrencyPool.js';
import { collectPlatform } from './collectPlatform.js';

/**
 * Collect multiple platforms and aggregate results.
 * @param {string[]} platforms
 * @param {{ registry: Record<string, object>, env?: object }} options
 * @returns {Promise<object>}
 */
export async function collectMany(platforms, options = {}) {
  const startedAt = new Date().toISOString();
  const pool = createConcurrencyPool(options.env?.globalConcurrency ?? 4);
  const results = await Promise.all(
    platforms.map((platform) => pool(() => collectPlatform(platform, options)))
  );

  return {
    requestedPlatforms: platforms,
    resolvedPlatforms: results.map((entry) => entry.canonicalPlatform),
    startedAt,
    finishedAt: new Date().toISOString(),
    successCount: results.filter((entry) => entry.success).length,
    failureCount: results.filter((entry) => !entry.success).length,
    results
  };
}
