import test from 'node:test';
import assert from 'node:assert/strict';

import { renderArticle } from '../../src/core/render.js';

test('renderArticle renders html with headline and section', async () => {
  const article = {
    template: 'daily-intelligence',
    meta: { title: 'AI 日报', digest: '摘要', date: '2026-03-31' },
    headline: { title: '头条', body: ['第一段'], source: 'CNBC' },
    sections: [
      {
        cn: '要闻',
        en: 'BRIEFING',
        blocks: [{ type: 'card', number: 1, title: '新闻一', body: ['正文'], source: 'CNBC' }]
      }
    ]
  };

  const html = await renderArticle(article);

  assert.match(html, /头条/);
  assert.match(html, /新闻一/);
  assert.doesNotMatch(html, /{{/);
});
