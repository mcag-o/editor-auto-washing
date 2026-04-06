import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';

import { loadJsonFile } from '../../src/core/io.js';
import { validateArticle } from '../../src/core/validate.js';

function runPythonValidate() {
  return new Promise((resolve, reject) => {
    const child = spawn('python3', ['./scripts/validate_article.py', './1/examples/article.sample.json', '--json'], {
      cwd: process.cwd(),
      stdio: ['ignore', 'pipe', 'pipe']
    });

    let stdout = '';
    let stderr = '';

    child.stdout.on('data', (chunk) => {
      stdout += chunk.toString();
    });

    child.stderr.on('data', (chunk) => {
      stderr += chunk.toString();
    });

    child.on('close', (code) => {
      if (code !== 0) {
        reject(new Error(stderr || `python validate failed with ${code}`));
        return;
      }
      resolve({ stdout, stderr });
    });
  });
}

test('node validation matches python validation semantics for sample article', async () => {
  const article = await loadJsonFile('./1/examples/article.sample.json');
  const nodeResult = await validateArticle(article);
  const pythonResult = await runPythonValidate();

  assert.equal(nodeResult.ok, true);
  assert.equal(pythonResult.stderr.includes('ERROR:'), false);
});
