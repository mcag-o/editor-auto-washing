import { writeFile } from 'node:fs/promises';
import { env } from '../config/env.js';
import { buildContentHubBundle } from '../integration/contentHubBundle.js';
import { createPlatformRegistry } from '../platforms/index.js';
import { listPlatforms } from '../platforms/aliases/platformRegistry.js';
import { collectMany } from '../scheduler/collectMany.js';

/**
 * Parse CLI arguments.
 * @param {string[]} argv
 * @returns {{ platforms: string[], bundleOut: string | null }}
 */
export function parseArgs(argv) {
  const args = { platforms: [], bundleOut: null };

  for (let index = 0; index < argv.length; index += 1) {
    if (argv[index] === '--platform' && argv[index + 1]) {
      args.platforms = [argv[index + 1]];
    }

    if (argv[index] === '--platforms' && argv[index + 1]) {
      args.platforms = argv[index + 1].split(',').map((item) => item.trim()).filter(Boolean);
    }

    if (argv[index] === '--all') {
      args.platforms = listPlatforms();
    }

    if (argv[index] === '--bundle-out' && argv[index + 1]) {
      args.bundleOut = argv[index + 1];
    }
  }

  return args;
}

/**
 * Build and write content-hub bundle file.
 * @param {string} filePath
 * @param {object} collectManyResult
 * @param {{ writeFileImpl?: typeof writeFile }} [dependencies]
 * @returns {Promise<object>}
 */
export async function writeBundleOutput(filePath, collectManyResult, dependencies = {}) {
  const { writeFileImpl = writeFile } = dependencies;
  const bundle = buildContentHubBundle(collectManyResult);
  await writeFileImpl(filePath, `${JSON.stringify(bundle, null, 2)}\n`, 'utf8');
  return bundle;
}

/**
 * Run the collector CLI.
 * @param {string[]} argv
 * @returns {Promise<object>}
 */
export async function run(argv = process.argv.slice(2)) {
  const args = parseArgs(argv);
  const platforms = args.platforms.length ? args.platforms : listPlatforms();
  const registry = createPlatformRegistry(env);
  const result = await collectMany(platforms, { registry, env });

  if (args.bundleOut) {
    await writeBundleOutput(args.bundleOut, result);
  }

  process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
  return result;
}

if (import.meta.url === `file://${process.argv[1]}`) {
  await run();
}
