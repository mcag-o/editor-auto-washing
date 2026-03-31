import { ensureMetaDefaults, normalizeParagraphs } from './article.js';

function safeFragment(value) {
  const cleaned = String(value)
    .split('')
    .map((char) => (/[a-z0-9]/i.test(char) ? char.toLowerCase() : '-'))
    .join('');

  const normalized = cleaned
    .split('-')
    .filter(Boolean)
    .join('-');

  return normalized || 'image';
}

function coverPrompt(article, meta = ensureMetaDefaults(article)) {
  const title = meta.title || '微信公众号封面';
  const dateShort = meta.date_short;
  const template = article.template;

  if (template === 'daily-intelligence') {
    return `Futuristic AI daily news cover for ${dateShort}. Theme: ${title}. Dark blue gradient, neural network nodes, holographic interface, editorial magazine cover, 16:9.`;
  }

  if (template === 'weekly-financial') {
    return `Dramatic financial weekly cover for ${dateShort}. Theme: ${title}. Dark red and black gradient, stock charts, commodities, Bloomberg or Economist style, 16:9.`;
  }

  return `Editorial deep analysis cover for ${dateShort}. Theme: ${title}. Deep navy palette, analytical charts, serious longform magazine style, cinematic light, 16:9.`;
}

function contentPrompt(article, title, detail) {
  const template = article.template;

  if (template === 'daily-intelligence') {
    return `Futuristic AI news illustration about ${title}. ${detail}. Modern research lab, holographic data panels, blue and white palette, professional editorial image, 16:9.`;
  }

  if (template === 'weekly-financial') {
    return `Financial news illustration about ${title}. ${detail}. Institutional market screens, macroeconomic tension, professional media photography style, 16:9.`;
  }

  return `Editorial analysis illustration about ${title}. ${detail}. Longform magazine style, layered charts and symbolic objects, restrained dramatic lighting, 16:9.`;
}

function sectionDetail(section, block) {
  const textBits = [];

  if (section.cn) {
    textBits.push(String(section.cn));
  }

  if (block.body) {
    const paragraphs = normalizeParagraphs(block.body);
    if (paragraphs.length > 0) {
      textBits.push(paragraphs[0].slice(0, 120));
    }
  }

  if (textBits.length === 0 && Array.isArray(block.days)) {
    const labels = block.days.slice(0, 2).map((row) => String(row?.label || '').trim()).filter(Boolean);
    textBits.push(labels.join(' / '));
  }

  return textBits.filter(Boolean).join('. ');
}

function chooseSectionSubject(section) {
  for (const block of section.blocks || []) {
    if (['card', 'opinion', 'week-ahead'].includes(block.type || 'card')) {
      return block;
    }
  }
  return null;
}

export function attachMissingImagePlans(article, { outputDir, maxContentImages = 3 } = {}) {
  const meta = ensureMetaDefaults(article);
  const dateShort = String(meta.date_short).replaceAll('.', '-');

  const cover = { ...(meta.cover_image || {}) };
  if (!cover.prompt) {
    cover.prompt = coverPrompt(article, meta);
  }
  if (!cover.local_path) {
    cover.local_path = `${outputDir}/cover-${dateShort}.png`;
  }
  meta.cover_image = cover;

  const plans = [{ target: 'cover', ...cover }];

  const headline = article.headline || {};
  let plannedSlots = 0;

  if (maxContentImages > 0) {
    const headlineImage = { ...(headline.image || {}) };
    if (!headlineImage.prompt) {
      const headlineParagraphs = normalizeParagraphs(headline.body);
      headlineImage.prompt = contentPrompt(
        article,
        String(headline.title || meta.title || '头条'),
        headlineParagraphs.length > 0 ? headlineParagraphs[0].slice(0, 120) : ''
      );
    }
    if (!headlineImage.caption) {
      headlineImage.caption = String(headline.title || '头条配图');
    }
    if (!headlineImage.local_path) {
      headlineImage.local_path = `${outputDir}/headline-${dateShort}.png`;
    }
    headline.image = headlineImage;
    article.headline = headline;
    plans.push({ target: 'headline', ...headlineImage });
    plannedSlots += 1;
  }

  let sectionIndex = 0;
  while (plannedSlots < maxContentImages && sectionIndex < (article.sections || []).length) {
    const section = article.sections[sectionIndex];
    sectionIndex += 1;

    if (section.image) {
      continue;
    }

    const subject = chooseSectionSubject(section);
    if (!subject) {
      continue;
    }

    const title = String(subject.title || section.cn || '正文配图');
    section.image = {
      prompt: contentPrompt(article, title, sectionDetail(section, subject)),
      caption: title,
      local_path: `${outputDir}/section-${String(plannedSlots).padStart(2, '0')}-${safeFragment(title).slice(0, 24)}.png`
    };

    plans.push({ target: `section[${sectionIndex - 1}]`, ...section.image });
    plannedSlots += 1;
  }

  article._plans ??= {};
  article._plans.images = plans;
  return article;
}
