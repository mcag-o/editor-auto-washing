import { describe, expect, test } from 'vitest';
import { parseArgs } from '../../src/cli/run.js';

describe('parseArgs', () => {
  test('parses collect-many arguments', () => {
    const parsed = parseArgs(['--platforms', 'baidu,weibo']);
    expect(parsed.platforms).toEqual(['baidu', 'weibo']);
  });
});
