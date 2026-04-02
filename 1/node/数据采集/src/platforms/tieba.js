import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull, toAbsoluteUrl } from './helpers.js';

const SOURCE_URL = 'http://tieba.baidu.com/hottopic/browse/topicList';

export const createTiebaCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: (payload) => payload?.data?.bang_topic?.topic_list ?? [],
  mapItem: (entry, index, platform) => {
    const url = toAbsoluteUrl(entry?.topic_url, 'http://tieba.baidu.com');
    if (!entry?.topic_name || !url) return null;
    return createItem(platform, index + 1, entry.topic_name, url, {
      summary: stringOrNull(entry.topic_desc),
      raw: entry
    });
  }
});
