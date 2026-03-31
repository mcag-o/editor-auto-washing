import test from 'node:test';
import assert from 'node:assert/strict';

import { TEMPLATE_NAMES } from '../../src/constants/templates.js';

test('all expected templates are registered', () => {
  assert.deepEqual([...TEMPLATE_NAMES].sort(), [
    'breaking-watch',
    'daily-intelligence',
    'deep-analysis',
    'industry-radar',
    'neo-brutalism',
    'product-release',
    'studio-brief',
    'weekly-financial'
  ]);
});
