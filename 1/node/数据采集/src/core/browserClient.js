import { chromium } from 'playwright';

/**
 * Render a page in Chromium and return HTML content.
 * @param {string} pageUrl
 * @param {{ timeoutMs?: number, userAgent?: string }} options
 * @returns {Promise<string>}
 */
export async function requestRenderedHtml(pageUrl, options = {}) {
  const browser = await chromium.launch({ headless: true });

  try {
    const page = await browser.newPage({ userAgent: options.userAgent });
    await page.goto(pageUrl, {
      waitUntil: 'domcontentloaded',
      timeout: options.timeoutMs ?? 15000
    });
    return await page.content();
  } finally {
    await browser.close();
  }
}
