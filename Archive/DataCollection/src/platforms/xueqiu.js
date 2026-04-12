import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull } from './helpers.js';

const SOURCE_URL = 'https://xueqiu.com/hot_event/list.json?count=10';

export const createXueqiuCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: (payload) => payload?.list ?? [],
  mapItem: (entry, index, platform) => {
    const title = String(entry?.tag || '').replace(/^#|#$/g, '').trim();
    if (!title) return null;
    return createItem(platform, index + 1, title, 'https://xueqiu.com/hot_event', {
      hot: stringOrNull(entry?.hot || entry?.status_count),
      summary: stringOrNull(entry?.content),
      raw: entry
    });
  }
});
