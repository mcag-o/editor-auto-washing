import test from 'node:test';
import assert from 'node:assert/strict';
import { rm } from 'node:fs/promises';
import { spawn } from 'node:child_process';

function runCli(args) {
  return new Promise((resolve) => {
    const child = spawn('node', ['./1/src/cli/index.js', ...args], {
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
      resolve({ code, stdout, stderr });
    });
  });
}

test('pipeline command prints summary json in dry-run mode', async () => {
  await rm('./1/.tmp/cli-pipeline', { recursive: true, force: true });

  const result = await runCli([
    'pipeline',
    './1/examples/article.sample.json',
    '--output-dir',
    './1/.tmp/cli-pipeline',
    '--dry-run'
  ]);

  assert.equal(result.code, 0);
  assert.match(result.stdout, /"html"/);
  assert.match(result.stdout, /"resolved_article"/);
});
