import { readTextFile } from './io.js';
import { ensureMetaDefaults, normalizeParagraphs, normalizeText } from './article.js';
import { TEMPLATE_DIR } from '../constants/templates.js';
import { ArticleError } from './errors.js';

function templateUrl(templateName) {
  return new URL(`${templateName}.html`, TEMPLATE_DIR);
}

function isStudioBriefTemplate(templateName = '') {
  return templateName === 'studio-brief';
}

function isBrutalTemplate(templateName = '') {
  return templateName === 'neo-brutalism';
}

async function readTemplate(templateName) {
  try {
    return await readTextFile(templateUrl(templateName));
  } catch {
    throw new ArticleError(`Template not found: ${templateName}`);
  }
}

export function renderImageBlock(image, { templateName = '' } = {}) {
  if (!image) {
    return '';
  }

  const url = normalizeText(image.url);
  if (!url) {
    return '';
  }

  const caption = normalizeText(image.caption || '配图');

  if (isStudioBriefTemplate(templateName)) {
    return (
      '<section style="background: #ffffff; padding: 8px 22px 24px; text-align: center;">' +
      '<section style="background: #f6f2ea; border-radius: 14px; overflow: hidden; border: 1px solid rgba(23,22,20,0.07);">' +
      `<img src="${url}" style="width: 100%; display: block; margin: 0;" />` +
      '<section style="padding: 10px 14px 12px;">' +
      `<p style="font-size: 11px; color: #7f766b; text-align: left; margin: 0; line-height: 1.6;">${caption} · AI 生成</p>` +
      '</section></section></section>'
    );
  }

  if (isBrutalTemplate(templateName)) {
    return (
      '<section style="background: #fffdf7; padding: 10px 18px 0; text-align: center;">' +
      '<section style="background: #ffffff; border: 4px solid #111111; box-shadow: 8px 8px 0 #7dff6b; overflow: hidden;">' +
      `<img src="${url}" style="width: 100%; display: block; margin: 0;" />` +
      '<section style="padding: 10px 12px 12px; background: #7dff6b; border-top: 4px solid #111111;">' +
      `<p style="font-size: 11px; color: #111111; text-align: left; margin: 0; line-height: 1.55; font-weight: 900; letter-spacing: 0.3px;">${caption}</p>` +
      '</section></section></section>'
    );
  }

  return (
    '<section style="background: #ffffff; padding: 15px 20px; text-align: center;">' +
    `<img src="${url}" style="width: 100%; border-radius: 4px; margin: 0;" />` +
    '<p style="font-size: 11px; color: #999; text-align: center; margin: 5px 0 0; font-style: italic;">' +
    `${caption} | AI 生成` +
    '</p></section>'
  );
}

export function renderSectionHeading(section, { templateName = '' } = {}) {
  const sectionEn = normalizeText(section.en || section.title_en || 'SECTION');
  const sectionCn = normalizeText(section.cn || section.title || '分区');

  if (isStudioBriefTemplate(templateName)) {
    return (
      '<section style="background: #ffffff; padding: 26px 22px 10px;">' +
      `<p style="font-size: 11px; color: #887d70; letter-spacing: 3px; text-transform: uppercase; margin: 0 0 8px;">${sectionEn}</p>` +
      `<p style="font-size: 21px; font-weight: 700; color: #171614; line-height: 1.3; margin: 0;">${sectionCn}</p>` +
      '</section>'
    );
  }

  if (isBrutalTemplate(templateName)) {
    return (
      '<section style="background: #fffdf7; padding: 18px 18px 10px;">' +
      '<section style="display: inline-block; padding: 10px 12px 8px; background: #00c2ff; border: 4px solid #111111; box-shadow: 6px 6px 0 #111111;">' +
      `<p style="font-size: 11px; color: #111111; letter-spacing: 2.6px; text-transform: uppercase; margin: 0 0 6px; font-weight: 900;">${sectionEn}</p>` +
      `<p style="font-size: 22px; font-weight: 900; color: #111111; line-height: 1.15; margin: 0;">${sectionCn}</p>` +
      '</section></section>'
    );
  }

  return (
    '<section style="background: #f8f7f4; padding: 20px 20px 5px;">' +
    `<p style="font-size: 10px; color: #e94560; letter-spacing: 5px; text-transform: uppercase; margin: 0 0 15px; font-weight: 700;">${sectionEn} · ${sectionCn}</p>` +
    '</section>'
  );
}

export function renderBodyParagraphs(value, { marginBottom = '8px', templateName = '' } = {}) {
  const paragraphs = normalizeParagraphs(value);
  if (paragraphs.length === 0) {
    return '';
  }

  let fontSize = '14px';
  let textColor = '#555';
  let lineHeight = '1.9';

  if (isStudioBriefTemplate(templateName)) {
    fontSize = '15px';
    textColor = '#332f2a';
    lineHeight = '1.95';
  } else if (isBrutalTemplate(templateName)) {
    fontSize = '15px';
    textColor = '#111111';
    lineHeight = '1.82';
  }

  return paragraphs
    .map(
      (paragraph) =>
        `<p style="font-size: ${fontSize}; color: ${textColor}; line-height: ${lineHeight}; margin: 0 0 ${marginBottom};">${normalizeText(paragraph, { allowHtml: true })}</p>`
    )
    .join('');
}

export function renderCardBlock(block, { highlight = false, templateName = '' } = {}) {
  const rawNumber = String(block.number || '');
  const number = normalizeText(/^\d+$/.test(rawNumber) ? rawNumber.padStart(2, '0') : rawNumber) || '&nbsp;';
  const color = highlight ? '#e94560' : '#1a1a2e';
  const title = normalizeText(block.title);
  const source = normalizeText(block.source || '');
  const body = renderBodyParagraphs(block.body, { templateName });

  if (isStudioBriefTemplate(templateName)) {
    const numberBg = highlight ? '#c96d44' : '#d9cfbf';
    const numberFg = highlight ? '#fffaf4' : '#3d372f';
    return (
      '<section style="background: #ffffff; padding: 0 22px 10px;">' +
      '<section style="background: #fbf8f1; border: 1px solid rgba(23,22,20,0.08); border-radius: 16px; padding: 18px 18px 16px;">' +
      '<section style="display: flex; align-items: flex-start;">' +
      `<section style="min-width: 34px; width: 34px; height: 34px; background: ${numberBg}; color: ${numberFg}; font-size: 12px; font-weight: 700; line-height: 34px; text-align: center; border-radius: 999px; margin-right: 14px; flex-shrink: 0; letter-spacing: 0.3px;">${number}</section>` +
      '<section style="flex: 1;">' +
      `<p style="font-size: 18px; font-weight: 700; color: #171614; margin: 0 0 10px; line-height: 1.4;">${title}</p>` +
      `${body}` +
      `<p style="font-size: 11px; color: #8b8175; margin: 10px 0 0; letter-spacing: 0.4px;">${source}</p>` +
      '</section></section></section></section>'
    );
  }

  if (isBrutalTemplate(templateName)) {
    const numberBg = highlight ? '#ff5fa2' : '#ffffff';
    return (
      '<section style="background: #ffffff; padding: 0 18px 12px;">' +
      '<section style="background: #ffffff; border: 4px solid #111111; box-shadow: 8px 8px 0 #111111; padding: 16px 14px 14px;">' +
      '<section style="display: flex; align-items: flex-start;">' +
      `<section style="min-width: 38px; width: 38px; height: 38px; background: ${numberBg}; color: #111111; font-size: 13px; font-weight: 900; line-height: 38px; text-align: center; margin-right: 12px; flex-shrink: 0; border: 3px solid #111111;">${number}</section>` +
      '<section style="flex: 1;">' +
      `<p style="font-size: 18px; font-weight: 900; color: #111111; margin: 0 0 10px; line-height: 1.28;">${title}</p>` +
      `${body}` +
      `<p style="font-size: 11px; color: #111111; margin: 10px 0 0; font-weight: 800; letter-spacing: 0.2px;">${source}</p>` +
      '</section></section></section></section>'
    );
  }

  return (
    '<section style="background: #ffffff; margin: 0 0 1px; padding: 20px;">' +
    '<section style="display: flex; align-items: flex-start;">' +
    `<section style="min-width: 36px; width: 36px; height: 36px; background: ${color}; color: #fff; font-size: 14px; font-weight: 900; line-height: 36px; text-align: center; border-radius: 4px; margin-right: 14px; flex-shrink: 0;">${number}</section>` +
    '<section style="flex: 1;">' +
    `<p style="font-size: 17px; font-weight: 800; color: #1a1a2e; margin: 0 0 8px; line-height: 1.4;">${title}</p>` +
    `${body}` +
    `<p style="font-size: 11px; color: #bbb; margin: 8px 0 0;">${source}</p>` +
    '</section></section></section>'
  );
}

export function renderOpinionBlock(block, { templateName = '' } = {}) {
  const opinion = {
    ...block,
    title: `编辑观点：${block.title || ''}`,
    source: block.source || '39Claw 编辑部'
  };

  return renderCardBlock(opinion, { highlight: true, templateName });
}

export function renderWeekAheadBlock(block, { templateName = '' } = {}) {
  const number = normalizeText(block.number || '');
  const title = normalizeText(block.title || '下周前瞻');
  const source = normalizeText(block.source || '');
  const rowsHtml = (block.days || [])
    .map((row) => {
      const label = normalizeText(row.label || '');
      const events = normalizeText(row.events || '', { allowHtml: true });

      if (isBrutalTemplate(templateName)) {
        return '<p style="font-size: 14px; color: #111111; line-height: 1.78; margin: 0 0 6px; font-weight: 700;">' +
          `<strong style="color: #111111;">${label}</strong> // ${events}</p>`;
      }

      return '<p style="font-size: 14px; color: #555; line-height: 1.9; margin: 0 0 5px;">' +
        `🔴 <strong>${label}</strong>：${events}</p>`;
    })
    .join('');

  if (isBrutalTemplate(templateName)) {
    return (
      '<section style="background: #ffffff; padding: 0 18px 12px;">' +
      '<section style="background: #ffffff; border: 4px solid #111111; box-shadow: 8px 8px 0 #00c2ff; padding: 16px 14px 14px;">' +
      '<section style="display: flex; align-items: flex-start;">' +
      `<section style="min-width: 38px; width: 38px; height: 38px; background: #ffd84d; color: #111111; font-size: 13px; font-weight: 900; line-height: 38px; text-align: center; margin-right: 12px; flex-shrink: 0; border: 3px solid #111111;">${number}</section>` +
      '<section style="flex: 1;">' +
      `<p style="font-size: 18px; font-weight: 900; color: #111111; margin: 0 0 10px; line-height: 1.28;">${title}</p>` +
      rowsHtml +
      `<p style="font-size: 11px; color: #111111; margin: 10px 0 0; font-weight: 800;">${source}</p>` +
      '</section></section></section></section>'
    );
  }

  return (
    '<section style="background: #ffffff; margin: 0 0 1px; padding: 20px;">' +
    '<section style="display: flex; align-items: flex-start;">' +
    `<section style="min-width: 36px; width: 36px; height: 36px; background: #e94560; color: #fff; font-size: 14px; font-weight: 900; line-height: 36px; text-align: center; border-radius: 4px; margin-right: 14px; flex-shrink: 0;">${number}</section>` +
    '<section style="flex: 1;">' +
    `<p style="font-size: 17px; font-weight: 800; color: #1a1a2e; margin: 0 0 8px; line-height: 1.4;">${title}</p>` +
    rowsHtml +
    `<p style="font-size: 11px; color: #bbb; margin: 8px 0 0;">${source}</p>` +
    '</section></section></section>'
  );
}

export function renderQuoteBlock(block, { templateName = '' } = {}) {
  const text = normalizeText(block.text || '', { allowHtml: true });
  const attribution = normalizeText(block.attribution || '');

  if (isBrutalTemplate(templateName)) {
    return (
      '<section style="background: #ffffff; padding: 0 18px 12px;">' +
      '<section style="background: #f5f5f5; border: 4px solid #111111; box-shadow: 8px 8px 0 #ffd84d; padding: 14px 14px 12px;">' +
      `<p style="font-size: 16px; color: #111111; line-height: 1.8; margin: 0 0 8px; font-weight: 800;">${text}</p>` +
      `<p style="font-size: 11px; color: #111111; margin: 0; font-weight: 800;">${attribution}</p>` +
      '</section></section>'
    );
  }

  return (
    '<section style="background: #ffffff; padding: 10px 20px 20px;">' +
    '<section style="border-left: 3px solid #e94560; padding: 6px 0 6px 14px;">' +
    `<p style="font-size: 15px; color: #3f3f3f; line-height: 1.9; margin: 0 0 8px;">${text}</p>` +
    `<p style="font-size: 11px; color: #999; margin: 0;">${attribution}</p>` +
    '</section></section>'
  );
}

export function renderTakeawaysBlock(block, { templateName = '' } = {}) {
  const title = normalizeText(block.title || '核心结论');
  const itemsHtml = (block.items || [])
    .map((item) => `<li style="margin: 0 0 8px;">${normalizeText(item, { allowHtml: true })}</li>`)
    .join('');

  if (isBrutalTemplate(templateName)) {
    return (
      '<section style="background: #ffffff; padding: 0 18px 12px;">' +
      '<section style="background: #ffd84d; border: 4px solid #111111; box-shadow: 8px 8px 0 #111111; padding: 16px 14px 8px;">' +
      `<p style="font-size: 16px; font-weight: 900; color: #111111; margin: 0 0 12px;">${title}</p>` +
      `<ul style="margin: 0; padding-left: 20px; color: #111111; font-size: 15px; line-height: 1.82; font-weight: 800;">${itemsHtml}</ul>` +
      '</section></section>'
    );
  }

  return (
    '<section style="background: #ffffff; padding: 20px;">' +
    '<section style="background: #f8f7f4; border: 1px solid rgba(233,69,96,0.12); border-radius: 8px; padding: 18px 18px 10px;">' +
    `<p style="font-size: 16px; font-weight: 800; color: #1a1a2e; margin: 0 0 12px;">${title}</p>` +
    `<ul style="margin: 0; padding-left: 20px; color: #555; font-size: 14px; line-height: 1.9;">${itemsHtml}</ul>` +
    '</section></section>'
  );
}

export function renderParagraphBlock(block, { templateName = '' } = {}) {
  const body = renderBodyParagraphs(block.text || block.body, {
    marginBottom: '12px',
    templateName
  });

  if (isBrutalTemplate(templateName)) {
    return `<section style="background: #fffdf7; padding: 0 18px 8px;">${body}</section>`;
  }

  if (isStudioBriefTemplate(templateName)) {
    return `<section style="background: #ffffff; padding: 0 22px 8px;">${body}</section>`;
  }

  return `<section style="background: #ffffff; padding: 0 20px 8px;">${body}</section>`;
}

export function renderBlock(block, { templateName = '' } = {}) {
  const blockType = block.type || 'card';

  if (blockType === 'card') {
    return renderCardBlock(block, {
      highlight: block.style === 'highlight',
      templateName
    });
  }

  if (blockType === 'opinion') {
    return renderOpinionBlock(block, { templateName });
  }

  if (blockType === 'week-ahead') {
    return renderWeekAheadBlock(block, { templateName });
  }

  if (blockType === 'quote') {
    return renderQuoteBlock(block, { templateName });
  }

  if (blockType === 'takeaways') {
    return renderTakeawaysBlock(block, { templateName });
  }

  if (blockType === 'paragraph') {
    return renderParagraphBlock(block, { templateName });
  }

  if (blockType === 'image') {
    return renderImageBlock(block, { templateName });
  }

  throw new ArticleError(`Unsupported block type: ${blockType}`);
}

export function renderSections(article, { templateName = '' } = {}) {
  const rendered = [];

  for (const section of article.sections || []) {
    rendered.push(renderSectionHeading(section, { templateName }));

    if (section.image) {
      rendered.push(renderImageBlock(section.image, { templateName }));
    }

    for (const block of section.blocks || []) {
      rendered.push(renderBlock(block, { templateName }));
    }
  }

  return rendered.join('');
}

export function renderHeadlineBody(article, { templateName = '' } = {}) {
  const paragraphs = normalizeParagraphs((article.headline || {}).body);
  if (paragraphs.length === 0) {
    return '';
  }

  if (isStudioBriefTemplate(templateName)) {
    return paragraphs
      .map(
        (paragraph) =>
          `<p style="font-size: 16px; color: #2f2b27; line-height: 1.95; margin: 0 0 10px;">${normalizeText(paragraph, { allowHtml: true })}</p>`
      )
      .join('');
  }

  if (isBrutalTemplate(templateName)) {
    return paragraphs
      .map(
        (paragraph) =>
          `<p style="font-size: 16px; color: #111111; line-height: 1.82; margin: 0 0 10px; font-weight: 700;">${normalizeText(paragraph, { allowHtml: true })}</p>`
      )
      .join('');
  }

  return paragraphs
    .map(
      (paragraph) =>
        `<p style="font-size: 15px; color: #3f3f3f; line-height: 2; margin: 0 0 8px;">${normalizeText(paragraph, { allowHtml: true })}</p>`
    )
    .join('');
}

export function applyReplacements(templateText, replacements) {
  let rendered = templateText;
  for (const [key, value] of Object.entries(replacements)) {
    rendered = rendered.replaceAll(`{{${key}}}`, value);
  }
  return rendered;
}

export function stripHtmlComments(htmlText) {
  return htmlText.replace(/<!--[\s\S]*?-->/g, '');
}

export async function renderArticle(article) {
  const meta = ensureMetaDefaults(article);
  const templateName = String(article.template || '').trim();
  const templateText = stripHtmlComments(await readTemplate(templateName));
  const headline = article.headline || {};

  return applyReplacements(templateText, {
    DATE_CN: normalizeText(meta.date_cn),
    DATE_SHORT: normalizeText(meta.date_short),
    SOURCE_COUNT: normalizeText(meta.source_count),
    NEWS_COUNT: normalizeText(meta.news_count),
    TITLE: normalizeText(meta.title),
    SUBTITLE: normalizeText(meta.subtitle || ''),
    AUTHOR: normalizeText(meta.author),
    DIGEST: normalizeText(meta.digest || '', { allowHtml: true }),
    HEADLINE_TITLE: normalizeText(headline.title),
    HEADLINE_BODY: renderHeadlineBody(article, { templateName }),
    HEADLINE_SOURCE: normalizeText(headline.source || ''),
    HEADLINE_IMAGE: renderImageBlock(headline.image, { templateName }),
    BODY_SECTIONS: renderSections(article, { templateName }),
    CONCLUSION: normalizeText(article.conclusion || '', { allowHtml: true }),
    CTA: normalizeText(article.cta || '你最关注哪一点？欢迎留言讨论。', { allowHtml: true })
  });
}

export function findContentImages(article) {
  const images = [];
  const headline = article.headline || {};

  if (headline.image && typeof headline.image === 'object') {
    images.push(headline.image);
  }

  for (const section of article.sections || []) {
    if (section.image && typeof section.image === 'object') {
      images.push(section.image);
    }

    for (const block of section.blocks || []) {
      if ((block.type || 'card') === 'image') {
        images.push(block);
      }
    }
  }

  return images;
}
