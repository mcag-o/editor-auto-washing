import { availableTemplates, ensureMetaDefaults, normalizeParagraphs } from './article.js';
import { findContentImages, stripHtmlComments } from './render.js';

const WECHAT_IMAGE_HOST_PATTERNS = [
  'mmbiz.qpic.cn',
  'mmbiz.qlogo.cn',
  'wx.qlogo.cn'
];

function isWechatImageUrl(url) {
  if (!url) {
    return false;
  }

  try {
    const parsed = new URL(url);
    const host = parsed.host.toLowerCase();
    return WECHAT_IMAGE_HOST_PATTERNS.some((pattern) => host.includes(pattern));
  } catch {
    return false;
  }
}

export async function validateArticle(article, { htmlText = null } = {}) {
  const errors = [];
  const warnings = [];

  const templateName = String(article.template || '').trim();
  const templates = [...availableTemplates()].sort();
  if (!templates.includes(templateName)) {
    errors.push(`template must be one of: ${templates.join(', ')}`);
  }

  const meta = ensureMetaDefaults(article);
  const title = String(meta.title || '').trim();
  const digest = String(meta.digest || '').trim();

  if (!title) {
    errors.push('meta.title is required');
  } else if (title.length > 32) {
    errors.push(`meta.title must be <= 32 characters, got ${title.length}`);
  }

  if (!digest) {
    errors.push('meta.digest is required');
  } else if (digest.length > 128) {
    errors.push(`meta.digest must be <= 128 characters, got ${digest.length}`);
  }

  if (!String(meta.author || '').trim()) {
    errors.push('meta.author is required');
  }

  const headline = article.headline || {};
  if (!String(headline.title || '').trim()) {
    errors.push('headline.title is required');
  }
  if (normalizeParagraphs(headline.body).length === 0) {
    errors.push('headline.body is required');
  }

  const sections = article.sections;
  if (!Array.isArray(sections) || sections.length === 0) {
    errors.push('sections must be a non-empty array');
  } else {
    sections.forEach((section, sectionIndex) => {
      if (!String(section.cn || section.title || '').trim()) {
        errors.push(`sections[${sectionIndex}] is missing cn/title`);
      }

      const blocks = section.blocks || [];
      if (blocks.length === 0) {
        warnings.push(`sections[${sectionIndex}] has no blocks`);
      }

      blocks.forEach((block, blockIndex) => {
        const blockType = block.type || 'card';
        if (['card', 'opinion'].includes(blockType) && !String(block.title || '').trim()) {
          errors.push(`sections[${sectionIndex}].blocks[${blockIndex}] is missing title`);
        }
        if (blockType === 'week-ahead' && !(block.days || []).length) {
          errors.push(`sections[${sectionIndex}].blocks[${blockIndex}] needs days`);
        }
        if (blockType === 'image') {
          const url = String(block.url || '').trim();
          if (url && !isWechatImageUrl(url)) {
            warnings.push(`sections[${sectionIndex}].blocks[${blockIndex}] image URL does not look like WeChat CDN`);
          }
        }
      });
    });
  }

  const coverMediaId = String(meta.thumb_media_id || meta.cover_media_id || '').trim();
  if (!coverMediaId) {
    warnings.push('meta.thumb_media_id is missing; draft creation will need upload-image first');
  }

  findContentImages(article).forEach((image, imageIndex) => {
    const url = String(image.url || '').trim();
    if (url && !isWechatImageUrl(url)) {
      warnings.push(`content image #${imageIndex + 1} does not look like WeChat CDN: ${url}`);
    }
  });

  if (htmlText != null) {
    const htmlWithoutComments = stripHtmlComments(htmlText);
    if (htmlWithoutComments.includes('{{') || htmlWithoutComments.includes('}}')) {
      errors.push('rendered HTML still contains unresolved placeholders');
    }

    const htmlSize = Buffer.byteLength(htmlText, 'utf8');
    if (htmlSize > 1024 * 1024) {
      errors.push(`rendered HTML exceeds 1MB limit: ${htmlSize} bytes`);
    }
  }

  return {
    ok: errors.length === 0,
    errors,
    warnings
  };
}
