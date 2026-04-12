import { defineHtmlCrawler } from './base.js';
import { createItem, stringOrNull, toAbsoluteUrl } from './helpers.js';

const SOURCE_URL = 'https://www.v2ex.com/?tab=hot';

export const createV2exCrawler = defineHtmlCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: ($) => $('div.cell.item').toArray(),
  mapItem: (entry, index, platform, $) => {
    const link = $(entry).find('span.item_title a').first();
    const title = link.text().trim();
    const url = toAbsoluteUrl(link.attr('href'), SOURCE_URL);
    if (!title || !url) return null;
    return createItem(platform, index + 1, title, url, {
      summary: stringOrNull($(entry).find('span.topic_info').first().text().trim()),
      raw: { title, url }
    });
  }
});
