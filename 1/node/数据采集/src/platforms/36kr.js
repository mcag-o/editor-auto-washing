import { defineJsonCrawler } from './base.js';
import { createItem } from './helpers.js';

const SOURCE_URL = 'https://gateway.36kr.com/api/mis/nav/home/nav/rank/hot';

export const create36KrCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  requestOptions: {
    method: 'POST',
    headers: { 'content-type': 'application/json; charset=utf-8' },
    body: JSON.stringify({
      partner_id: 'wap',
      param: { siteId: 1, platformId: 2 },
      timestamp: Date.now()
    })
  },
  mapEntries: (payload) => payload?.data?.hotRankList ?? [],
  mapItem: (entry, index, platform) => {
    const itemId = entry?.itemId;
    const title = entry?.templateMaterial?.widgetTitle;
    if (!title || !itemId) return null;
    return createItem(platform, index + 1, title, `https://www.36kr.com/p/${itemId}`, {
      summary: title,
      raw: entry
    });
  }
});
