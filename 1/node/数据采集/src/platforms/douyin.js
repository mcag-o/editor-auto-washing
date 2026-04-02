import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull } from './helpers.js';

const SOURCE_URL = 'https://www.douyin.com/aweme/v1/web/hot/search/list/';

export const createDouyinCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  requestOptions: {
    headers: { referer: 'https://www.douyin.com/' }
  },
  mapEntries: (payload) => payload?.data?.word_list ?? [],
  mapItem: (entry, index, platform) => {
    if (!entry?.word) return null;
    const topic = encodeURIComponent(entry.word);
    return createItem(
      platform,
      index + 1,
      entry.word,
      `https://www.douyin.com/hot/${entry.sentence_id}?trending_topic=${topic}&hotValue=${entry.hot_value}`,
      {
        hot: stringOrNull(entry.hot_value),
        summary: stringOrNull(entry.word),
        raw: entry
      }
    );
  }
});
