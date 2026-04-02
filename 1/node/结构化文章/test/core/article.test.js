import test from 'node:test';
import assert from 'node:assert/strict';

import {
  ensureMetaDefaults,
  normalizeParagraphs,
  countSources,
  countNewsItems
} from '../../src/core/article.js';

test('ensureMetaDefaults fills meta defaults', () => {
  const article = {
    template: 'daily-intelligence',
    meta: { title: 'T', digest: 'D', date: '2026-03-31' },
    headline: { title: 'H', body: ['B'], source: 'Headline Source' },
    sections: [
      {
        cn: '要闻',
        en: 'BRIEFING',
        blocks: [
          { type: 'card', title: 'x', body: ['y'], source: 'Section Source' }
        ]
      }
    ]
  };

  const meta = ensureMetaDefaults(article);

  assert.equal(meta.author, '39Claw');
  assert.equal(meta.open_comment, 1);
  assert.equal(meta.source_count, 2);
  assert.equal(meta.news_count, 1);
  assert.equal(meta.date_short, '2026.03.31');
});

test('normalizeParagraphs splits string paragraphs and trims list items', () => {
  assert.deepEqual(normalizeParagraphs('第一段\n\n第二段'), ['第一段', '第二段']);
  assert.deepEqual(normalizeParagraphs([' A ', '', 'B ']), ['A', 'B']);
});

test('countSources and countNewsItems summarize article content', () => {
  const article = {
    headline: { source: 'Headline Source' },
    sections: [
      {
        blocks: [
          { type: 'card', source: 'A' },
          { type: 'opinion', source: 'A' },
          { type: 'week-ahead', source: 'B' },
          { type: 'paragraph', source: 'Ignored' }
        ]
      }
    ]
  };

  assert.equal(countSources(article), 4);
  assert.equal(countNewsItems(article), 3);
});
