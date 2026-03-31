import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

import { ArticleError } from './errors.js';

function normalizePathLike(pathLike) {
  return pathLike instanceof URL ? fileURLToPath(pathLike) : pathLike;
}

export async function ensureDir(pathLike) {
  const path = normalizePathLike(pathLike);
  await mkdir(path, { recursive: true });
}

export async function readTextFile(pathLike) {
  const path = normalizePathLike(pathLike);
  return readFile(path, 'utf8');
}

export async function loadJsonFile(pathLike) {
  const path = normalizePathLike(pathLike);
  const content = await readTextFile(path);
  const payload = JSON.parse(content);

  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
    throw new ArticleError('Article input must be a JSON object.');
  }

  return payload;
}

export async function writeJsonFile(pathLike, payload) {
  const path = normalizePathLike(pathLike);
  await ensureDir(dirname(path));
  await writeFile(path, `${JSON.stringify(payload, null, 2)}\n`, 'utf8');
}
