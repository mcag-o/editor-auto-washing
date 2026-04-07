import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull } from './helpers.js';

const SOURCE_URL = 'https://weibo.com/ajax/side/hotSearch';

export const createWeiboCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: (payload) => payload?.data?.realtime ?? [],
  mapItem: (entry, index, platform) => {
    if (!entry?.word) return null;
    const query = encodeURIComponent(`#${entry.word}#`);
    return createItem(platform, index + 1, entry.word, `https://s.weibo.com/weibo?q=${query}`, {
      hot: stringOrNull(entry.raw_hot),
      summary: stringOrNull(entry.note || entry.word),
      raw: entry
    });
  }
});
