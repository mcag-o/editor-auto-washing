import { TEMPLATE_NAMES } from '../constants/templates.js';
import { ArticleError } from './errors.js';

const NEWS_BLOCK_TYPES = new Set(['card', 'opinion', 'week-ahead']);
const WEEKDAYS = ['星期一', '星期二', '星期三', '星期四', '星期五', '星期六', '星期日'];

function escapeHtml(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function toDateParts(date) {
  return {
    year: date.getUTCFullYear(),
    month: date.getUTCMonth() + 1,
    day: date.getUTCDate(),
    weekday: WEEKDAYS[date.getUTCDay() === 0 ? 6 : date.getUTCDay() - 1]
  };
}

export function availableTemplates() {
  return new Set(TEMPLATE_NAMES);
}

export function normalizeText(value, { allowHtml = false } = {}) {
  const text = value == null ? '' : String(value).trim();
  return allowHtml ? text : escapeHtml(text);
}

export function normalizeParagraphs(value) {
  if (value == null) {
    return [];
  }

  if (typeof value === 'string') {
    const parts = value
      .split(/\n{2,}/)
      .map((part) => part.trim())
      .filter(Boolean);

    return parts.length > 0 ? parts : [value.trim()];
  }

  if (Array.isArray(value)) {
    return value
      .map((item) => String(item).trim())
      .filter(Boolean);
  }

  throw new ArticleError(`Expected string or list of strings, got: ${typeof value}`);
}

export function toIntString(value, fallback) {
  const parsed = Number.parseInt(value, 10);
  return Number.isNaN(parsed) ? fallback : String(parsed);
}

export function parseIsoDate(raw) {
  if (!raw) {
    const now = new Date();
    return new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
  }

  const match = String(raw).match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!match) {
    throw new ArticleError(`Invalid ISO date: ${raw}`);
  }

  const [, year, month, day] = match;
  return new Date(Date.UTC(Number(year), Number(month) - 1, Number(day)));
}

export function countSources(article) {
  const sources = new Set();
  const headline = article?.headline ?? {};

  if (headline.source) {
    sources.add(String(headline.source).trim());
  }

  for (const section of article?.sections ?? []) {
    for (const block of section?.blocks ?? []) {
      if (block?.source) {
        sources.add(String(block.source).trim());
      }
    }
  }

  return sources.size;
}

export function countNewsItems(article) {
  let count = 0;

  for (const section of article?.sections ?? []) {
    for (const block of section?.blocks ?? []) {
      if (NEWS_BLOCK_TYPES.has(block?.type ?? 'card')) {
        count += 1;
      }
    }
  }

  return count;
}

export function ensureMetaDefaults(article) {
  const rawMeta = article?.meta;
  const meta = rawMeta && typeof rawMeta === 'object' && !Array.isArray(rawMeta) ? rawMeta : { ...(rawMeta ?? {}) };
  article.meta = meta;

  const parsedDate = parseIsoDate(meta.date);
  const { year, month, day, weekday } = toDateParts(parsedDate);

  if (meta.date == null) {
    meta.date = `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
  }
  if (meta.date_cn == null) {
    meta.date_cn = `${year} 年 ${month} 月 ${day} 日 · ${weekday}`;
  }
  if (meta.date_short == null) {
    meta.date_short = `${year}.${String(month).padStart(2, '0')}.${String(day).padStart(2, '0')}`;
  }
  if (meta.author == null) {
    meta.author = '39Claw';
  }
  if (meta.open_comment == null) {
    meta.open_comment = 1;
  }
  if (meta.source_count == null) {
    meta.source_count = countSources(article);
  }
  if (meta.news_count == null) {
    meta.news_count = countNewsItems(article);
  }

  return meta;
}
