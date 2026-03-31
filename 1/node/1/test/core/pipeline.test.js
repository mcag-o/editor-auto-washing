import test from 'node:test';
import assert from 'node:assert/strict';

import { runPipeline } from '../../src/core/pipeline.js';

test('runPipeline supports dry-run and returns summary', async () => {
  const summary = await runPipeline({
    input: './1/examples/article.sample.json',
    outputDir: './1/.tmp/build',
    dryRun: true
  });

  assert.equal(typeof summary.html, 'string');
  assert.equal(typeof summary.resolved_article, 'string');
  assert.equal(summary.html.endsWith('.html'), true);
});
