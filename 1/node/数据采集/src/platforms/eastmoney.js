import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull } from './helpers.js';

const SOURCE_URL = 'https://np-weblist.eastmoney.com/comm/web/getFastNewsList';

export const createEastmoneyCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: (payload) => payload?.data?.fastNewsList ?? payload?.data?.list ?? [],
  mapItem: (entry, index, platform) => {
    const title = entry?.title || entry?.digest || entry?.infoTypeName;
    if (!title) return null;
    return createItem(platform, index + 1, title, entry?.url || 'https://www.eastmoney.com/', {
      summary: stringOrNull(entry?.digest),
      publishTime: stringOrNull(entry?.showTime),
      raw: entry
    });
  }
});
