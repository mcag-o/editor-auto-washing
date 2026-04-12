import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull } from './helpers.js';

const SOURCE_URL = 'https://api.juejin.cn/content_api/v1/content/article_rank?category_id=1&type=hot';

export const createJuejinCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: (payload) => payload?.data ?? [],
  mapItem: (entry, index, platform) => {
    const content = entry?.content ?? {};
    if (!content?.title || !content?.content_id) return null;
    return createItem(platform, index + 1, content.title, `https://juejin.cn/post/${content.content_id}`, {
      summary: stringOrNull(content.title),
      author: stringOrNull(entry?.author_user_info?.user_name),
      raw: entry
    });
  }
});
