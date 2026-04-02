import { resultSchema } from '../schemas/resultSchema.js';
import { getPlatformMeta, normalizePlatformId } from '../platforms/aliases/platformRegistry.js';
import { createFailureResult } from '../platforms/helpers.js';
import { UnsupportedPlatformError } from '../core/errors.js';

/**
 * Collect one platform and always return a structured result.
 * @param {string} platformId
 * @param {{ registry: Record<string, { collect(input: object): Promise<object> }>, env?: object }} options
 * @returns {Promise<object>}
 */
export async function collectPlatform(platformId, options = {}) {
  const canonical = normalizePlatformId(platformId);
  const meta = canonical ? getPlatformMeta(canonical) : null;
  const crawler = canonical ? options.registry?.[canonical] : null;

  if (!canonical || !meta || !crawler) {
    return createFailureResult({
      platform: String(platformId),
      canonicalPlatform: canonical ?? String(platformId),
      aliases: meta?.aliases ?? [],
      displayName: meta?.displayName ?? String(platformId),
      sourceType: meta?.sourceType === 'html' ? 'html' : 'json-api',
      sourceUrl: meta?.sourceUrl ?? 'https://example.invalid/',
      error: new UnsupportedPlatformError(platformId)
    });
  }

  try {
    const result = await crawler.collect({
      platform: String(platformId),
      canonicalPlatform: canonical,
      meta,
      env: options.env
    });
    return resultSchema.parse(result);
  } catch (error) {
    return createFailureResult({
      platform: String(platformId),
      canonicalPlatform: canonical,
      aliases: meta.aliases,
      displayName: meta.displayName,
      sourceType: meta.sourceType === 'html' ? 'html' : meta.sourceType === 'browser' ? 'browser' : 'json-api',
      sourceUrl: meta.sourceUrl,
      error: {
        code: error.code ?? 'COLLECT_FAILED',
        message: error.message ?? 'Collection failed',
        retryable: Boolean(error.retryable)
      }
    });
  }
}
