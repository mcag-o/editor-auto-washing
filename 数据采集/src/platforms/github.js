import { defineJsonCrawler } from './base.js';
import { createItem, stringOrNull } from './helpers.js';

const SOURCE_URL = 'https://api.github.com/search/repositories?q=stars:%3E1&sort=stars';

export const createGithubCrawler = defineJsonCrawler({
  sourceUrl: SOURCE_URL,
  requestOptions: { headers: { accept: 'application/vnd.github+json' } },
  mapEntries: (payload) => payload?.items ?? [],
  mapItem: (entry, index, platform) => {
    if (!entry?.full_name || !entry?.html_url) return null;
    return createItem(platform, index + 1, entry.full_name, entry.html_url, {
      hot: stringOrNull(entry?.stargazers_count),
      summary: stringOrNull(entry?.description),
      author: stringOrNull(entry?.owner?.login),
      raw: entry
    });
  }
});
