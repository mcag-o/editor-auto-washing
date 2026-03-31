import test from 'node:test';
import assert from 'node:assert/strict';

import { runCommand } from '../../src/adapters/subprocess.js';

test('runCommand returns stdout when command succeeds', async () => {
  const output = await runCommand('python3', ['-c', "print('ok')"]);
  assert.equal(output, 'ok');
});

test('runCommand throws when command exits non-zero', async () => {
  await assert.rejects(() => runCommand('python3', ['-c', 'import sys; sys.exit(2)']), /Command failed/);
});
