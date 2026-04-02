import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull, toAbsoluteUrl } from './helpers.js';

const SOURCE_URL = 'https://top.baidu.com/api/board?platform=wise&tab=realtime';

export const createBaiduCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: (payload) => payload?.data?.cards?.[0]?.content?.[0]?.content ?? [],
  mapItem: (entry, index, platform) => {
    const url = toAbsoluteUrl(String(entry.url || '').replace('m.', 'www.'), 'https://top.baidu.com/');
    if (!entry.word || !url) return null;
    return createItem(platform, index + 1, entry.word, url, {
      hot: stringOrNull(entry.hotScore),
      summary: stringOrNull(entry.desc),
      raw: entry
    });
  }
});
