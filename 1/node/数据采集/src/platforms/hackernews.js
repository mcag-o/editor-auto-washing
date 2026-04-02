import { defineHtmlCrawler } from './base.js';
import { createItem, stringOrNull, toAbsoluteUrl } from './helpers.js';

const SOURCE_URL = 'https://news.ycombinator.com/';

export const createHackernewsCrawler = defineHtmlCrawler({
  sourceUrl: SOURCE_URL,
  mapEntries: ($) => $('tr.athing').toArray(),
  mapItem: (entry, index, platform, $) => {
    const item = $(entry);
    const link = item.find('.titleline a').first();
    const title = link.text().trim();
    const url = toAbsoluteUrl(link.attr('href'), SOURCE_URL);
    if (!title || !url) return null;
    const meta = item.next('tr');
    const score = meta.find('.score').first().text().trim();
    const author = meta.find('.hnuser').first().text().trim();
    const comments = meta.find('a').last().text().trim();
    const site = item.find('.sitestr').first().text().trim();
    return createItem(platform, index + 1, title, url, {
      hot: stringOrNull(score),
      author: stringOrNull(author),
      summary: stringOrNull(`来源: ${site} | 评论: ${comments}`),
      raw: { title, url, score, author, comments, site }
    });
  }
});
