import { defineHtmlCrawler } from './base.js';
import { createItem, stringOrNull, toAbsoluteUrl } from './helpers.js';

const SOURCE_URL = 'https://www.52pojie.cn/forum.php?mod=guide&view=hot';

export const create52pojieCrawler = defineHtmlCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: ($) => $('tbody[id^="normalthread_"]').toArray(),
  mapItem: (entry, index, platform, $) => {
    const link = $(entry).find('a.s.xst').first();
    const title = link.text().trim();
    const url = toAbsoluteUrl(link.attr('href'), SOURCE_URL);
    if (!title || !url) return null;
    return createItem(platform, index + 1, title, url, {
      summary: stringOrNull($(entry).find('.by cite').first().text().trim()),
      raw: { title, url }
    });
  }
});
