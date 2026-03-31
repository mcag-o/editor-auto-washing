import test from 'node:test';
import assert from 'node:assert/strict';

import * as api from '../../src/index.js';

test('package entry exports an object', () => {
  assert.equal(typeof api, 'object');
});
