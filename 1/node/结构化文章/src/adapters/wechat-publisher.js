import { runCommand } from './subprocess.js';

function parseJsonLike(output) {
  try {
    return JSON.parse(output);
  } catch {
    return {};
  }
}

function extractFirst(patterns, output) {
  for (const pattern of patterns) {
    const match = output.match(pattern);
    if (match) {
      return match[1];
    }
  }
  return null;
}

export async function uploadCover(command, args = [], options = {}) {
  const output = await runCommand(command, args, options);
  const payload = parseJsonLike(output);
  const mediaId = payload.media_id || extractFirst([/media_id['": ]+([\w-]+)/], output);
  return { mediaId, output };
}

export async function uploadContentImage(command, args = [], options = {}) {
  const output = await runCommand(command, args, options);
  const payload = parseJsonLike(output);
  const url = payload.url || extractFirst([/(https?:\/\/\S+)/], output);
  return { url, output };
}

export async function createDraft(command, args = [], options = {}) {
  const output = await runCommand(command, args, options);
  const payload = parseJsonLike(output);
  const draftMediaId = payload.media_id || payload.draft_media_id || extractFirst([/media_id['": ]+([\w-]+)/], output);
  return { draftMediaId, output };
}

export async function publishDraft(command, args = [], options = {}) {
  const output = await runCommand(command, args, options);
  const payload = parseJsonLike(output);
  const publishId = payload.publish_id || extractFirst([/publish_id['": ]+([\w-]+)/], output);
  return { publishId, output };
}
