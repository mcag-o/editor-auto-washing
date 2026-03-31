import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';

import { loadJsonFile, writeJsonFile } from '../../src/core/io.js';

test('loadJsonFile throws when payload is not an object', async () => {
  const fixtureUrl = new URL('../fixtures/list.json', import.meta.url);
  await assert.rejects(() => loadJsonFile(fixtureUrl), /JSON object/);
});

test('writeJsonFile writes pretty json with trailing newline', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'wechat-claw-io-'));
  const target = join(dir, 'payload.json');

  await writeJsonFile(target, { ok: true, count: 1 });

  const content = await readFile(target, 'utf8');
  assert.match(content, /"ok": true/);
  assert.equal(content.endsWith('\n'), true);
});
