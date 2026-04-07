import { readFileSync } from 'node:fs';
import path from 'node:path';
import { parse } from 'yaml';
import {
  getPlatformMeta,
  listPlatforms,
  normalizePlatformId
} from '../platforms/aliases/platformRegistry.js';

const DEFAULT_CONFIG_PATH = path.resolve(process.cwd(), 'config/sources.yaml');

function normalizeSourceEntry(entry) {
  const source = entry && typeof entry === 'object' ? entry : {};
  const inputId = String(source.id ?? '').trim();
  const id = normalizePlatformId(inputId) ?? inputId;
  const meta = getPlatformMeta(id);

  if (!id || !meta) {
    throw new Error(`Unknown source id in config: ${inputId || '(empty)'}`);
  }

  return {
    id,
    enabled: source.enabled !== false,
    displayName: source.displayName ?? meta.displayName,
    aliases: Array.isArray(source.aliases) ? source.aliases : meta.aliases,
    sourceType: source.sourceType ?? meta.sourceType,
    sourceUrl: source.sourceUrl ?? meta.sourceUrl,
    notes: source.notes ?? '',
    timeoutMs: Number.isFinite(source.timeoutMs) ? source.timeoutMs : null,
    headers: source.headers && typeof source.headers === 'object' ? source.headers : {},
    requestOptions: {
      ...(source.headers && typeof source.headers === 'object'
        ? {
            headers: source.headers
          }
        : {}),
      ...(Number.isFinite(source.timeoutMs)
        ? {
            timeoutMs: Number(source.timeoutMs)
          }
        : {})
    }
  };
}

function normalizeConfig(raw, configPath) {
  const data = raw && typeof raw === 'object' ? raw : {};
  const baseDir = path.dirname(configPath);
  const sources = Array.isArray(data.sources)
    ? data.sources.map((entry) => normalizeSourceEntry(entry))
    : listPlatforms().map((id) => normalizeSourceEntry({ id, enabled: true }));

  const uniqueById = new Map();
  for (const source of sources) {
    uniqueById.set(source.id, source);
  }

  const outputRaw = String(data.output?.outputDir ?? data.output?.bundleOut ?? './output');
  const outputPath = path.isAbsolute(outputRaw) ? outputRaw : path.resolve(baseDir, outputRaw);

  return {
    configPath,
    output: {
      outputDir: outputPath
    },
    schedule: {
      everyMinutes: Number.isFinite(data.schedule?.everyMinutes)
        ? Number(data.schedule.everyMinutes)
        : 30
    },
    sources: [...uniqueById.values()]
  };
}

export function buildMetaById(config) {
  return Object.fromEntries(
    (config?.sources ?? []).map((source) => [
      source.id,
      {
        displayName: source.displayName,
        aliases: source.aliases,
        sourceType: source.sourceType,
        sourceUrl: source.sourceUrl,
        requestOptions: source.requestOptions
      }
    ])
  );
}

export function loadSourcesConfig(configPath = DEFAULT_CONFIG_PATH) {
  const resolvedPath = path.isAbsolute(configPath) ? configPath : path.resolve(process.cwd(), configPath);
  const content = readFileSync(resolvedPath, 'utf8');
  const raw = parse(content);
  return normalizeConfig(raw, resolvedPath);
}

export function listEnabledSources(config) {
  return (config?.sources ?? []).filter((entry) => entry.enabled);
}

export function listAllSources(config) {
  return [...(config?.sources ?? [])];
}
