import test from 'node:test';
import assert from 'node:assert/strict';

import { attachMissingImagePlans } from '../../src/core/image-plan.js';

test('attachMissingImagePlans adds cover and content image plans', () => {
  const article = {
    template: 'daily-intelligence',
    meta: { title: 'AI 日报', digest: '摘要', date: '2026-03-31' },
    headline: { title: '头条', body: ['第一段'] },
    sections: [
      {
        cn: '要闻',
        en: 'BRIEFING',
        blocks: [{ type: 'card', title: '新闻一', body: ['正文'], source: '来源' }]
      }
    ]
  };

  const updated = attachMissingImagePlans(article, { outputDir: 'build/images', maxContentImages: 3 });

  assert.ok(updated.meta.cover_image.prompt);
  assert.ok(updated.meta.cover_image.local_path.endsWith('cover-2026-03-31.png'));
  assert.ok(updated.headline.image.prompt);
  assert.ok(updated.sections[0].image.prompt);
  assert.equal(updated._plans.images.length, 3);
});
