import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull } from './helpers.js';

const SOURCE_URL = 'https://www.zhihu.com/api/v3/explore/guest/feeds?limit=30&ws_qiangzhisafe=0';

export const createZhihuCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: (payload) => payload?.data ?? [],
  mapItem: (entry, index, platform) => {
    const target = entry?.target ?? {};
    const question = target?.question ?? {};
    if (!question?.title || !question?.id) return null;
    return createItem(platform, index + 1, question.title, `https://www.zhihu.com/question/${question.id}`, {
      summary: stringOrNull(target.excerpt),
      author: stringOrNull(target.author?.name),
      hot: stringOrNull(target.voteup_count),
      raw: entry
    });
  }
});
