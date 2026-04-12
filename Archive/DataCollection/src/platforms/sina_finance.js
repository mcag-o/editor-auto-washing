import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull } from './helpers.js';

const SOURCE_URL = 'https://zhibo.sina.com.cn/api/zhibo/feed?page=1&page_size=20&zhibo_id=152&tag_id=0&dire=f&dpc=1&pagesize=20';

export const createSinaFinanceCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: (payload) => payload?.result?.data?.feed?.list ?? payload?.data?.feed?.list ?? [],
  mapItem: (entry, index, platform) => {
    const title = entry?.rich_text || entry?.content || entry?.title;
    const url = entry?.url || 'https://finance.sina.com.cn/';
    if (!title) return null;
    return createItem(platform, index + 1, String(title).replace(/<[^>]+>/g, '').trim(), url, {
      summary: stringOrNull(entry?.content),
      publishTime: stringOrNull(entry?.create_time),
      raw: entry
    });
  }
});
