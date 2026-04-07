const BUNDLE_VERSION = '1.0';

function toArray(value) {
  return Array.isArray(value) ? value : [];
}

function normalizeItem(result, item) {
  return {
    sourceType: result?.sourceType ?? null,
    platform: result?.platform ?? result?.canonicalPlatform ?? null,
    canonicalPlatform: result?.canonicalPlatform ?? result?.platform ?? null,
    title: item?.title ?? '',
    url: item?.url ?? '',
    summary: item?.summary ?? null,
    author: item?.author ?? null,
    publishTime: item?.publishTime ?? null,
    tags: toArray(item?.tags),
    category: item?.category ?? null,
    rank: typeof item?.rank === 'number' ? item.rank : null,
    hot: item?.hot ?? null,
    metadata: item?.metadata && typeof item.metadata === 'object' ? item.metadata : {},
    raw: item?.raw ?? null
  };
}

function normalizeSource(result) {
  return {
    sourceType: result?.sourceType ?? null,
    platform: result?.platform ?? result?.canonicalPlatform ?? null,
    canonicalPlatform: result?.canonicalPlatform ?? result?.platform ?? null,
    displayName: result?.displayName ?? null,
    sourceUrl: result?.sourceUrl ?? null,
    fetchedAt: result?.fetchedAt ?? null,
    aliases: toArray(result?.aliases),
    success: Boolean(result?.success),
    itemCount: toArray(result?.items).length,
    warnings: toArray(result?.warnings),
    error: result?.error ?? null
  };
}

function normalizeFailure(result) {
  return {
    sourceType: result?.sourceType ?? null,
    platform: result?.platform ?? result?.canonicalPlatform ?? null,
    canonicalPlatform: result?.canonicalPlatform ?? result?.platform ?? null,
    displayName: result?.displayName ?? null,
    sourceUrl: result?.sourceUrl ?? null,
    fetchedAt: result?.fetchedAt ?? null,
    warnings: toArray(result?.warnings),
    error: result?.error ?? null
  };
}

/**
 * Build content-hub bundle payload from collectMany result.
 * @param {object} collectManyResult
 * @returns {object}
 */
export function buildContentHubBundle(collectManyResult) {
  const results = toArray(collectManyResult?.results);

  return {
    bundleVersion: BUNDLE_VERSION,
    generatedAt:
      typeof collectManyResult?.finishedAt === 'string'
        ? collectManyResult.finishedAt
        : new Date().toISOString(),
    sources: results.map((result) => normalizeSource(result)),
    items: results
      .filter((result) => Boolean(result?.success))
      .flatMap((result) => toArray(result.items).map((item) => normalizeItem(result, item))),
    failures: results
      .filter((result) => !result?.success)
      .map((result) => normalizeFailure(result))
  };
}
