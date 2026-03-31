import test from 'node:test';
import assert from 'node:assert/strict';
import { rm } from 'node:fs/promises';
import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
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

test('render command writes html output', async () => {
  const outputPath = join(process.cwd(), '1/.tmp/cli-render/article.html');
  await rm(join(process.cwd(), '1/.tmp/cli-render'), { recursive: true, force: true });

  const result = await runCli([
    'render',
    './1/examples/article.sample.json',
    '-o',
    outputPath,
    '--check'
  ]);

  assert.equal(result.code, 0);
  assert.equal(existsSync(outputPath), true);
  assert.match(readFileSync(outputPath, 'utf8'), /头条标题/);
});
