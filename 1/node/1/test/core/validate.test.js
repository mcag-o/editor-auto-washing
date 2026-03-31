import test from 'node:test';
import assert from 'node:assert/strict';

import { renderArticle } from '../../src/core/render.js';
import { validateArticle } from '../../src/core/validate.js';

test('validateArticle returns ok for valid article', async () => {
  const article = {
    template: 'daily-intelligence',
    meta: { title: 'AI 日报测试', digest: '这里是一段摘要', date: '2026-03-31' },
    headline: { title: '头条标题', body: ['正文'], source: '来源' },
    sections: [{ cn: '要闻', en: 'BRIEFING', blocks: [{ type: 'card', title: '新闻标题', body: ['正文'], source: '来源' }] }]
  };

  const result = await validateArticle(article);
  assert.equal(result.ok, true);
  assert.deepEqual(result.errors, []);
});

test('validateArticle returns error for missing required fields', async () => {
  const article = {
    template: 'missing-template',
    meta: { title: '', digest: '' },
    headline: { title: '', body: [] },
    sections: []
  };

  const result = await validateArticle(article);
  assert.equal(result.ok, false);
  assert.ok(result.errors.some((error) => error.includes('template')));
  assert.ok(result.errors.some((error) => error.includes('meta.title')));
});

test('validateArticle warns about missing cover media id and unresolved placeholders', async () => {
  const article = {
    template: 'daily-intelligence',
    meta: { title: 'AI 日报测试', digest: '这里是一段摘要', date: '2026-03-31' },
    headline: { title: '头条标题', body: ['正文'], source: '来源' },
    sections: [{ cn: '要闻', en: 'BRIEFING', blocks: [{ type: 'card', title: '新闻标题', body: ['正文'], source: '来源' }] }]
  };

  const html = await renderArticle(article);
  const result = await validateArticle(article, { htmlText: `${html} {{UNRESOLVED}}` });

  assert.ok(result.warnings.some((warning) => warning.includes('thumb_media_id')));
  assert.ok(result.errors.some((error) => error.includes('unresolved placeholders')));
});
