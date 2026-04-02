import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull } from './helpers.js';

const SOURCE_URL = 'https://www.cls.cn/featured/v1/column/list';

export const createClsCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: (payload) => payload?.data?.column_list ?? [],
  mapItem: (entry, index, platform) => {
    const article = entry?.article_list ?? {};
    const title = article?.title ? `[${entry.title}] ${article.title}` : entry?.title;
    if (!title) return null;
    return createItem(platform, index + 1, title, article?.jump_url || 'https://www.cls.cn/telegraph', {
      summary: stringOrNull(article?.brief || entry?.brief || article?.title),
      raw: entry
    });
  }
});
