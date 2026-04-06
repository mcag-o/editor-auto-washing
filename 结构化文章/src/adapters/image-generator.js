import { runCommand } from './subprocess.js';

export async function generateImage(command, args = [], options = {}) {
  return await runCommand(command, args, options);
}
