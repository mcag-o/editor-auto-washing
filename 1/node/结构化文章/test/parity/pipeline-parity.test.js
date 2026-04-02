import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';

import { runPipeline } from '../../src/core/pipeline.js';

function runPythonPipeline() {
  return new Promise((resolve, reject) => {
    const child = spawn('python3', ['./scripts/run_pipeline.py', './1/examples/article.sample.json', '--output-dir', './1/.tmp/python-build', '--dry-run'], {
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
        reject(new Error(stderr || `python pipeline failed with ${code}`));
        return;
      }
      resolve(JSON.parse(stdout));
    });
  });
}

test('node dry-run pipeline matches python summary shape', async () => {
  const nodeSummary = await runPipeline({
    input: './1/examples/article.sample.json',
    outputDir: './1/.tmp/node-build',
    dryRun: true
  });
  const pythonSummary = await runPythonPipeline();

  for (const key of ['html', 'resolved_article']) {
    assert.equal(typeof nodeSummary[key], 'string');
    assert.equal(typeof pythonSummary[key], 'string');
  }
});
