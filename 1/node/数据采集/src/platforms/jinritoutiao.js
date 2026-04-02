import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull } from './helpers.js';

const SOURCE_URL = 'https://www.toutiao.com/hot-event/hot-board/?origin=toutiao_pc';

export const createJinritoutiaoCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: (payload) => payload?.data ?? payload?.data?.list ?? [],
  mapItem: (entry, index, platform) => {
    const title = entry?.Title || entry?.title || entry?.word;
    const url = entry?.Url || entry?.url || entry?.ClusterId ? `https://www.toutiao.com/trending/${entry.ClusterId || entry.cluster_id}` : null;
    if (!title || !url) return null;
    return createItem(platform, index + 1, title, url, {
      hot: stringOrNull(entry?.HotValue || entry?.hot_value),
      summary: stringOrNull(entry?.Label || entry?.summary || title),
      raw: entry
    });
  }
});
