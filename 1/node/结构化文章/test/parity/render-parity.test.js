import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { spawn } from 'node:child_process';

import { renderArticle } from '../../src/core/render.js';
import { loadJsonFile } from '../../src/core/io.js';

function runPythonRender() {
  return new Promise((resolve, reject) => {
    const child = spawn('python3', ['./scripts/render_article.py', './1/examples/article.sample.json', '-o', './1/.tmp/parity-python.html', '--check'], {
      cwd: process.cwd(),
      stdio: ['ignore', 'pipe', 'pipe']
    });

    let stderr = '';
    child.stderr.on('data', (chunk) => {
      stderr += chunk.toString();
    });

    child.on('close', async (code) => {
      if (code !== 0) {
        reject(new Error(stderr || `python render failed with ${code}`));
        return;
      }
      resolve(await readFile('./1/.tmp/parity-python.html', 'utf8'));
    });
  });
}

test('node render matches key python render fragments', async () => {
  const article = await loadJsonFile('./1/examples/article.sample.json');
  const nodeHtml = await renderArticle(article);
  const pythonHtml = await runPythonRender();

  for (const fragment of ['头条标题', '新闻标题', 'THE DAILY INTELLIGENCE']) {
    assert.equal(nodeHtml.includes(fragment), true);
    assert.equal(pythonHtml.includes(fragment), true);
  }
});
