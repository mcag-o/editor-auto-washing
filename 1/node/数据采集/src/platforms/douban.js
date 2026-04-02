import { defineHtmlCrawler } from './base.js';
import { createItem, stringOrNull, toAbsoluteUrl } from './helpers.js';

const SOURCE_URL = 'https://www.douban.com/group/explore';

export const createDoubanCrawler = defineHtmlCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: ($) => $('div.channel-item').toArray(),
  mapItem: (entry, index, platform, $) => {
    const link = $(entry).find('h3 a').first();
    const title = link.text().trim();
    const url = toAbsoluteUrl(link.attr('href'), SOURCE_URL);
    if (!title || !url) return null;
    return createItem(platform, index + 1, title, url, {
      summary: stringOrNull($(entry).find('div.content').text().trim()),
      raw: { title, url }
    });
  }
});
