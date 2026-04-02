import test from 'node:test';
import assert from 'node:assert/strict';

import {
  uploadCover,
  uploadContentImage,
  createDraft,
  publishDraft
} from '../../src/adapters/wechat-publisher.js';

test('uploadCover parses media_id from json output', async () => {
  const mediaId = await uploadCover('python3', [
    '-c',
    "print('{\"media_id\": \"MEDIA123\"}')"
  ]);
  assert.equal(mediaId.mediaId, 'MEDIA123');
});

test('uploadContentImage parses url from json output', async () => {
  const result = await uploadContentImage('python3', [
    '-c',
    "print('{\"url\": \"https://example.com/image.png\"}')"
  ]);
  assert.equal(result.url, 'https://example.com/image.png');
});

test('createDraft and publishDraft parse ids from json output', async () => {
  const draft = await createDraft('python3', ['-c', "print('{\"media_id\": \"DRAFT1\"}')"]);
  const publish = await publishDraft('python3', ['-c', "print('{\"publish_id\": \"PUB1\"}')"]);

  assert.equal(draft.draftMediaId, 'DRAFT1');
  assert.equal(publish.publishId, 'PUB1');
});
