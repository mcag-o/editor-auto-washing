import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull } from './helpers.js';

const SOURCE_URL = 'https://sspai.com/api/v1/article/index/page/get?limit=20&offset=0&created_at=0';

export const createShaoshupaiCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: (payload) => payload?.data ?? [],
  mapItem: (entry, index, platform) => {
    if (!entry?.title || !entry?.id) return null;
    return createItem(platform, index + 1, entry.title, `https://sspai.com/post/${entry.id}`, {
      summary: stringOrNull(entry.summary),
      raw: entry
    });
  }
});
