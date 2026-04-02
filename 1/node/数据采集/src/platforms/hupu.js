import { defineHtmlCrawler } from './base.js';
import { createItem, stringOrNull, toAbsoluteUrl } from './helpers.js';

const SOURCE_URL = 'https://bbs.hupu.com/all-gambia';

export const createHupuCrawler = defineHtmlCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: ($) => $('div.t-info').toArray(),
  mapItem: (entry, index, platform, $) => {
    const link = $(entry).find('a').first();
    const title = $(entry).find('span.t-title').first().text().trim();
    const url = toAbsoluteUrl(link.attr('href'), SOURCE_URL);
    if (!title || !url) return null;
    return createItem(platform, index + 1, title, url, {
      summary: stringOrNull($(entry).find('span.t-replies').first().text().trim()),
      raw: { title, url }
    });
  }
});
