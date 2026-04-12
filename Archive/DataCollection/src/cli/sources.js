import { request } from '../core/httpClient.js';
import { listAllSources, loadSourcesConfig } from '../config/sources.js';

function formatRows(rows) {
  return `${JSON.stringify(rows, null, 2)}\n`;
}

export function createSourceStatusRows(config) {
  return listAllSources(config).map((source) => ({
    id: source.id,
    enabled: source.enabled ? 'on' : 'off',
    sourceType: source.sourceType,
    status: 'configured',
    sourceUrl: source.sourceUrl
  }));
}

async function checkOne(source, timeoutMs) {
  try {
    await request(source.sourceUrl, { timeoutMs, retryCount: 0, method: 'GET' });
    return {
      id: source.id,
      enabled: source.enabled ? 'on' : 'off',
      sourceType: source.sourceType,
      status: 'ok',
      sourceUrl: source.sourceUrl
    };
  } catch (error) {
    return {
      id: source.id,
      enabled: source.enabled ? 'on' : 'off',
      sourceType: source.sourceType,
      status: 'failed',
      sourceUrl: source.sourceUrl,
      error: error.message
    };
  }
}

export async function runSourcesCli(argv = process.argv.slice(2)) {
  const command = argv[0] ?? 'list';
  const configArgIndex = argv.findIndex((item) => item === '--config');
  const configPath = configArgIndex >= 0 ? argv[configArgIndex + 1] : undefined;
  const config = loadSourcesConfig(configPath);

  if (command === 'list') {
    process.stdout.write(formatRows(createSourceStatusRows(config)));
    return;
  }

  if (command === 'check') {
    const timeoutMs = 4000;
    const checks = await Promise.all(listAllSources(config).map((source) => checkOne(source, timeoutMs)));
    process.stdout.write(formatRows(checks));
    return;
  }

  throw new Error(`Unsupported sources command: ${command}`);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  await runSourcesCli();
}
