import test from 'node:test';
import assert from 'node:assert/strict';

import { loadJsonFile } from '../../src/core/io.js';
import { renderArticle } from '../../src/core/render.js';

test('example article can be loaded and rendered', async () => {
  const article = await loadJsonFile('./1/examples/article.sample.json');
  const html = await renderArticle(article);

  assert.match(html, /头条标题/);
  assert.match(html, /新闻标题/);
});
