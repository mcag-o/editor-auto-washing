import { mkdir, writeFile } from 'node:fs/promises';
import { dirname, join, parse } from 'node:path';

import { uploadCover, uploadContentImage, createDraft, publishDraft } from '../adapters/wechat-publisher.js';
import { generateImage } from '../adapters/image-generator.js';
import { loadJsonFile, writeJsonFile } from './io.js';
import { ensureMetaDefaults } from './article.js';
import { attachMissingImagePlans } from './image-plan.js';
import { findContentImages, renderArticle } from './render.js';
import { validateArticle } from './validate.js';
import { ArticleError } from './errors.js';

export async function runPipeline({
  input,
  outputDir = 'build',
  resolvedArticle = null,
  skipPlanImages = false,
  maxContentImages = 3,
  imageScript = null,
  wechatScript = null,
  createDraft: shouldCreateDraft = false,
  publish = false,
  dryRun = false
} = {}) {
  const article = await loadJsonFile(input);

  if (publish && !shouldCreateDraft) {
    throw new ArticleError('--publish requires --create-draft in the current pipeline');
  }

  await mkdir(outputDir, { recursive: true });

  let resolved = article;
  if (!skipPlanImages) {
    resolved = attachMissingImagePlans(article, {
      outputDir: join(outputDir, 'images'),
      maxContentImages
    });
  }

  const meta = ensureMetaDefaults(resolved);
  const cover = { ...(meta.cover_image || {}) };

  if (imageScript && !dryRun && cover.prompt && cover.local_path) {
    await generateImage('python3', [imageScript, '--prompt', cover.prompt, '--filename', cover.local_path, '--resolution', '1K']);
  }

  for (const image of findContentImages(resolved)) {
    if (imageScript && !dryRun && image.prompt && image.local_path) {
      await generateImage('python3', [imageScript, '--prompt', image.prompt, '--filename', image.local_path, '--resolution', '1K']);
    }
  }

  if (wechatScript && !dryRun) {
    if (cover.local_path && !meta.thumb_media_id) {
      const { mediaId } = await uploadCover('python3', [wechatScript, 'upload-image', cover.local_path]);
      if (mediaId) {
        meta.thumb_media_id = mediaId;
      }
    }

    for (const image of findContentImages(resolved)) {
      if (image.local_path && !image.url) {
        const { url } = await uploadContentImage('python3', [wechatScript, 'upload-content-image', image.local_path]);
        if (url) {
          image.url = url;
        }
      }
    }
  }

  const htmlText = await renderArticle(resolved);
  const validation = await validateArticle(resolved, { htmlText });
  if (!validation.ok) {
    throw new ArticleError(validation.errors.join('; '));
  }

  const stem = parse(input).name;
  const htmlPath = join(outputDir, `${stem}.html`);
  const resolvedPath = resolvedArticle || join(outputDir, `${stem}.resolved.json`);

  await writeFile(htmlPath, htmlText, 'utf8');
  await writeJsonFile(resolvedPath, resolved);

  let draftMediaId = null;
  let publishId = null;

  if (shouldCreateDraft) {
    if (dryRun) {
      // ignore during dry-run
    } else if (!wechatScript) {
      throw new ArticleError('--wechat-script is required for --create-draft');
    } else if (!meta.thumb_media_id) {
      throw new ArticleError('meta.thumb_media_id is missing; upload-image must finish before draft creation');
    } else {
      const draft = await createDraft('python3', [wechatScript, 'draft-add']);
      draftMediaId = draft.draftMediaId;

      if (publish) {
        const published = await publishDraft('python3', [wechatScript, 'publish', draftMediaId]);
        publishId = published.publishId;
      }
    }
  }

  return {
    html: htmlPath,
    resolved_article: resolvedPath,
    draft_media_id: draftMediaId,
    publish_id: publishId,
    cover_media_id: meta.thumb_media_id || null
  };
}
