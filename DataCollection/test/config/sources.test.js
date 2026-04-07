import { describe, expect, test } from 'vitest';
import { listEnabledSources, loadSourcesConfig } from '../../src/config/sources.js';

describe('sources config', () => {
  test('loads sources and returns enabled subset', () => {
    const config = loadSourcesConfig();
    expect(Array.isArray(config.sources)).toBe(true);
    expect(config.sources.length).toBeGreaterThan(0);

    const enabled = listEnabledSources(config);
    expect(enabled.length).toBeGreaterThan(0);
    expect(enabled.every((item) => item.enabled === true)).toBe(true);
  });

  test('supports custom output path in config', () => {
    const config = loadSourcesConfig();
    expect(typeof config.output.outputDir).toBe('string');
    expect(config.output.outputDir.length).toBeGreaterThan(0);
  });
});
