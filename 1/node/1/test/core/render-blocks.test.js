import test from 'node:test';
import assert from 'node:assert/strict';

import { renderArticle } from '../../src/core/render.js';

test('renderArticle supports opinion week-ahead quote takeaways and image blocks', async () => {
  const article = {
    template: 'weekly-financial',
    meta: {
      title: '财经周报',
      digest: '摘要',
      date: '2026-03-31'
    },
    headline: {
      title: '本周头条',
      body: ['头条第一段'],
      source: 'Bloomberg'
    },
    sections: [
      {
        cn: '全球市场',
        en: 'GLOBAL MARKETS',
        blocks: [
          { type: 'opinion', number: 1, title: '观点标题', body: ['观点正文'] },
          {
            type: 'week-ahead',
            number: 2,
            title: '下周前瞻',
            days: [
              { label: '周一', events: 'CPI' },
              { label: '周二', events: '财报' }
            ],
            source: '日历'
          },
          { type: 'quote', text: '市场正在重估。', attribution: '分析师' },
          { type: 'takeaways', title: '核心结论', items: ['第一点', '第二点'] },
          { type: 'image', url: 'https://mmbiz.qpic.cn/example', caption: '配图说明' },
          { type: 'paragraph', text: '补充段落' }
        ]
      }
    ]
  };

  const html = await renderArticle(article);

  assert.match(html, /编辑观点：观点标题/);
  assert.match(html, /下周前瞻/);
  assert.match(html, /市场正在重估/);
  assert.match(html, /核心结论/);
  assert.match(html, /配图说明/);
  assert.match(html, /补充段落/);
});
