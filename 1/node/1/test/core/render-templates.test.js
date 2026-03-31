import test from 'node:test';
import assert from 'node:assert/strict';

import { renderArticle } from '../../src/core/render.js';

test('renderArticle applies studio brief and neo brutalism theme variations', async () => {
  const baseArticle = {
    meta: { title: '风格测试', digest: '摘要', date: '2026-03-31' },
    headline: { title: '头条', body: ['正文'], source: '来源' },
    conclusion: '结论',
    cta: '行动号召',
    sections: [
      {
        cn: '要闻',
        en: 'BRIEFING',
        image: { url: 'https://mmbiz.qpic.cn/example', caption: '配图' },
        blocks: [{ type: 'card', number: 1, title: '标题', body: ['正文'], source: '来源' }]
      }
    ]
  };

  const studioHtml = await renderArticle({ ...baseArticle, template: 'studio-brief' });
  const brutalHtml = await renderArticle({ ...baseArticle, template: 'neo-brutalism' });

  assert.match(studioHtml, /Studio Brief/);
  assert.match(studioHtml, /border-radius: 14px/);
  assert.match(brutalHtml, /NEO BRUTALISM ISSUE/);
  assert.match(brutalHtml, /box-shadow: 8px 8px 0/);
});
