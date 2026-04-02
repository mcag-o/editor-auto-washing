import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull } from './helpers.js';

const SOURCE_URL = 'https://api.stackexchange.com/2.3/questions?order=desc&sort=hot&site=stackoverflow';

export const createStackoverflowCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: (payload) => payload?.items ?? [],
  mapItem: (entry, index, platform) => {
    if (!entry?.title || !entry?.link) return null;
    return createItem(platform, index + 1, entry.title, entry.link, {
      hot: stringOrNull(entry?.score),
      author: stringOrNull(entry?.owner?.display_name),
      summary: stringOrNull(entry?.tags?.join(', ')),
      tags: entry?.tags ?? [],
      raw: entry
    });
  }
});
