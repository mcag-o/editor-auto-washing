import { env } from '../src/config/env.js';
import { createPlatformRegistry } from '../src/platforms/index.js';
import { collectMany } from '../src/scheduler/collectMany.js';

const registry = createPlatformRegistry(env);
const result = await collectMany(['baidu', 'hackernews'], { registry, env });
process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
