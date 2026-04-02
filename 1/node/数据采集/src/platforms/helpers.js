import { resultSchema } from '../schemas/resultSchema.js';

/**
 * Create a normalized item.
 * @param {string} platform
 * @param {number} rank
 * @param {string} title
 * @param {string} url
 * @param {object} extra
 * @returns {object}
 */
export function createItem(platform, rank, title, url, extra = {}) {
  return {
    id: `${platform}-${rank}`,
    rank,
    title,
    url,
    mobileUrl: extra.mobileUrl ?? null,
    hot: extra.hot ?? null,
    summary: extra.summary ?? null,
    author: extra.author ?? null,
    category: extra.category ?? null,
    tags: extra.tags ?? [],
    publishTime: extra.publishTime ?? null,
    metadata: extra.metadata ?? {},
    raw: extra.raw ?? null
  };
}

/**
 * Build a normalized crawler result and validate it.
 * @param {object} params
 * @returns {object}
 */
export function createSuccessResult(params) {
  return resultSchema.parse({
    platform: params.platform,
    canonicalPlatform: params.canonicalPlatform,
    aliases: params.aliases,
    displayName: params.displayName,
    sourceType: params.sourceType,
    sourceUrl: params.sourceUrl,
    fetchedAt: new Date().toISOString(),
    success: true,
    items: params.items,
    warnings: params.warnings ?? []
  });
}

/**
 * Build a normalized failure result and validate it.
 * @param {object} params
 * @returns {object}
 */
export function createFailureResult(params) {
  return resultSchema.parse({
    platform: params.platform,
    canonicalPlatform: params.canonicalPlatform,
    aliases: params.aliases ?? [],
    displayName: params.displayName,
    sourceType: params.sourceType,
    sourceUrl: params.sourceUrl,
    fetchedAt: new Date().toISOString(),
    success: false,
    items: [],
    warnings: params.warnings ?? [params.error.message],
    error: {
      code: params.error.code,
      message: params.error.message,
      retryable: Boolean(params.error.retryable)
    }
  });
}

/**
 * Ensure URLs are absolute.
 * @param {string | undefined | null} value
 * @param {string} base
 * @returns {string | null}
 */
export function toAbsoluteUrl(value, base) {
  if (!value) {
    return null;
  }

  try {
    return new URL(value, base).toString();
  } catch {
    return null;
  }
}

/**
 * Convert values to a nullable string.
 * @param {unknown} value
 * @returns {string | null}
 */
export function stringOrNull(value) {
  if (value == null || value === '') {
    return null;
  }

  return String(value);
}
