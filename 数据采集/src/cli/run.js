import { env } from '../config/env.js';
import { createPlatformRegistry } from '../platforms/index.js';
import { listPlatforms } from '../platforms/aliases/platformRegistry.js';
import { collectMany } from '../scheduler/collectMany.js';

/**
 * Parse CLI arguments.
 * @param {string[]} argv
 * @returns {{ platforms: string[] }}
 */
export function parseArgs(argv) {
  const args = { platforms: [] };

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
  }

  return args;
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
  process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
  return result;
}

if (import.meta.url === `file://${process.argv[1]}`) {
  await run();
}
