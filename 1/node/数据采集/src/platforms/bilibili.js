import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull } from './helpers.js';

const SOURCE_URL = 'https://api.bilibili.com/x/web-interface/popular';

export const createBilibiliCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  requestOptions: { headers: { referer: 'https://www.bilibili.com/' } },
  mapEntries: (payload) => payload?.data?.list ?? [],
  mapItem: (entry, index, platform) => {
    const bvid = entry?.bvid;
    if (!entry?.title || !bvid) return null;
    return createItem(platform, index + 1, entry.title, `https://www.bilibili.com/video/${bvid}`, {
      summary: stringOrNull(entry.desc),
      author: stringOrNull(entry.owner?.name),
      hot: stringOrNull(entry.stat?.view),
      raw: entry
    });
  }
});
