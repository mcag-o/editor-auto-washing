import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { env } from '../config/env.js';
import { buildMetaById, listEnabledSources, loadSourcesConfig } from '../config/sources.js';
import { createPlatformRegistry } from '../platforms/index.js';
import { collectMany } from '../scheduler/collectMany.js';

function toTimestampFragment(value) {
  return String(value).replace(/[-:]/g, '').replace(/\./g, '').replace(/\s+/g, '');
}

function normalizeOutputDirectory(outputDir) {
  return outputDir;
}

/**
 * Parse CLI arguments.
 * @param {string[]} argv
 * @returns {{ platforms: string[], outputDir: string | null, configPath: string | null, noOutput: boolean, useEnabledAll: boolean }}
 */
export function parseArgs(argv) {
  const args = {
    platforms: [],
    outputDir: null,
    configPath: null,
    noOutput: false,
    useEnabledAll: false
  };

  for (let index = 0; index < argv.length; index += 1) {
    if (argv[index] === '--platform' && argv[index + 1]) {
      args.platforms = [argv[index + 1]];
    }

    if (argv[index] === '--platforms' && argv[index + 1]) {
      args.platforms = argv[index + 1].split(',').map((item) => item.trim()).filter(Boolean);
    }

    if (argv[index] === '--all') {
      args.useEnabledAll = true;
    }

    if ((argv[index] === '--output-dir' || argv[index] === '--output' || argv[index] === '--bundle-out') && argv[index + 1]) {
      args.outputDir = argv[index + 1];
    }

    if (argv[index] === '--config' && argv[index + 1]) {
      args.configPath = argv[index + 1];
    }

    if (argv[index] === '--no-output') {
      args.noOutput = true;
    }
  }

  return args;
}

/**
 * Write per-source json output files.
 * @param {string} outputDir
 * @param {object} collectManyResult
 * @param {{ writeFileImpl?: typeof writeFile, mkdirImpl?: typeof mkdir }} [dependencies]
 * @returns {Promise<string[]>}
 */
export async function writeSourceOutputs(outputDir, collectManyResult, dependencies = {}) {
  const { writeFileImpl = writeFile, mkdirImpl = mkdir } = dependencies;
  const normalizedOutputDir = normalizeOutputDirectory(outputDir);
  await mkdirImpl(normalizedOutputDir, { recursive: true });
  const finishedAt = collectManyResult?.finishedAt ?? new Date().toISOString();
  const timestamp = toTimestampFragment(finishedAt);
  const writtenFiles = [];

  for (const result of collectManyResult?.results ?? []) {
    const sourceId = result?.canonicalPlatform ?? result?.platform ?? 'unknown';
    const fileName = `${sourceId}-${timestamp}.json`;
    const filePath = path.join(normalizedOutputDir, fileName);
    await writeFileImpl(filePath, `${JSON.stringify(result, null, 2)}\n`, 'utf8');
    writtenFiles.push(filePath);
  }

  return writtenFiles;
}

/**
 * Run the collector CLI.
 * @param {string[]} argv
 * @returns {Promise<object>}
 */
export async function run(argv = process.argv.slice(2)) {
  const args = parseArgs(argv);
  const sourcesConfig = loadSourcesConfig(args.configPath ?? undefined);
  const enabledSources = listEnabledSources(sourcesConfig);
  const platforms = args.platforms.length
    ? args.platforms
    : args.useEnabledAll
      ? enabledSources.map((item) => item.id)
      : enabledSources.map((item) => item.id);
  const registry = createPlatformRegistry(env);
  const metaById = buildMetaById(sourcesConfig);
  const result = await collectMany(platforms, { registry, env, metaById });

  if (!args.noOutput) {
    const outputDir = args.outputDir ?? sourcesConfig.output.outputDir;
    await writeSourceOutputs(outputDir, result);
  }

  process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
  return result;
}

if (import.meta.url === `file://${process.argv[1]}`) {
  await run();
}
