import test from 'node:test';
import assert from 'node:assert/strict';
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

test('validate command exits successfully for valid article', async () => {
  const result = await runCli([
    'validate',
    './1/examples/article.sample.json'
  ]);

  assert.equal(result.code, 0);
  assert.match(result.stdout, /"ok": true/);
});
