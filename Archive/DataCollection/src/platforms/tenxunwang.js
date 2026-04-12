import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull } from './helpers.js';

const SOURCE_URL = 'https://i.news.qq.com/gw/event/pc_hot_ranking_list?ids_hash=&offset=0&page_size=51&appver=15.5_qqnews_7.1.60&rank_id=hot';

export const createTenxunwangCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: (payload) => payload?.idlist?.[0]?.newslist ?? payload?.data?.newslist ?? [],
  mapItem: (entry, index, platform) => {
    const title = entry?.title;
    const url = entry?.url || entry?.vurl || entry?.open_url;
    if (!title || !url) return null;
    return createItem(platform, index + 1, title, url, {
      hot: stringOrNull(entry?.hotEventScore),
      summary: stringOrNull(entry?.abstract),
      raw: entry
    });
  }
});
