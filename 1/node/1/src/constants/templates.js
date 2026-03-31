export const TEMPLATE_NAMES = [
  'daily-intelligence',
  'weekly-financial',
  'deep-analysis',
  'industry-radar',
  'product-release',
  'breaking-watch',
  'studio-brief',
  'neo-brutalism'
];

export const TEMPLATE_DIR = new URL('../../templates/', import.meta.url);

export function isSpecialTemplate(templateName) {
  return templateName === 'studio-brief' || templateName === 'neo-brutalism';
}
