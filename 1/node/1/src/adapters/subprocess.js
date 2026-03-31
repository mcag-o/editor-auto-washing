import { spawn } from 'node:child_process';

import { ArticleError } from '../core/errors.js';

export async function runCommand(command, args = [], options = {}) {
  return await new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      stdio: ['ignore', 'pipe', 'pipe'],
      ...options
    });

    let stdout = '';
    let stderr = '';

    child.stdout.on('data', (chunk) => {
      stdout += chunk.toString();
    });

    child.stderr.on('data', (chunk) => {
      stderr += chunk.toString();
    });

    child.on('error', (error) => {
      reject(new ArticleError(`Command failed: ${error.message}`));
    });

    child.on('close', (code) => {
      if (code === 0) {
        resolve(stdout.trim() || stderr.trim());
        return;
      }

      reject(new ArticleError(`Command failed with exit code ${code}: ${stderr.trim() || stdout.trim()}`));
    });
  });
}
