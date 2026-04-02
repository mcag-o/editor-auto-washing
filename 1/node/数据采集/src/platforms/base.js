import * as cheerio from 'cheerio';
import { ParseError } from '../core/errors.js';
import { createSuccessResult } from './helpers.js';

/**
 * Build a normalized JSON API crawler.
 * @param {{ sourceUrl: string, mapEntries(payload: any): any[], mapItem(entry: any, index: number, platform: string): object }} config
 * @returns {(deps: { requestJson(url: string, options?: object): Promise<any> }) => { collect(input: object): Promise<object> }}
 */
export function defineJsonCrawler(config) {
  return function createCrawler(deps) {
    return {
      async collect({ platform, canonicalPlatform, meta }) {
        const payload = await deps.requestJson(config.sourceUrl, config.requestOptions ?? {});
        const entries = config.mapEntries(payload);

        if (!Array.isArray(entries)) {
          throw new ParseError(`Expected array entries for ${canonicalPlatform}`);
        }

        return createSuccessResult({
          platform,
          canonicalPlatform,
          aliases: meta.aliases,
          displayName: meta.displayName,
          sourceType: 'json-api',
          sourceUrl: config.sourceUrl,
          items: entries
            .map((entry, index) => config.mapItem(entry, index, canonicalPlatform))
            .filter(Boolean)
        });
      }
    };
  };
}

/**
 * Build a normalized HTML crawler.
 * @param {{ sourceUrl: string, mapEntries($: cheerio.CheerioAPI): any[], mapItem(entry: any, index: number, platform: string, $: cheerio.CheerioAPI): object }} config
 * @returns {(deps: { requestText(url: string, options?: object): Promise<string>, requestRenderedHtml?: (url: string, options?: object) => Promise<string>, browserEnabled?: boolean }) => { collect(input: object): Promise<object> }}
 */
export function defineHtmlCrawler(config) {
  return function createCrawler(deps) {
    return {
      async collect({ platform, canonicalPlatform, meta }) {
        let html;

        try {
          html = await deps.requestText(config.sourceUrl, config.requestOptions ?? {});
        } catch (error) {
          if (!deps.browserEnabled || !deps.requestRenderedHtml) {
            throw error;
          }
          html = await deps.requestRenderedHtml(config.sourceUrl, { timeoutMs: 15000 });
        }

        const $ = cheerio.load(html);
        const entries = config.mapEntries($);

        if (!Array.isArray(entries)) {
          throw new ParseError(`Expected array entries for ${canonicalPlatform}`);
        }

        return createSuccessResult({
          platform,
          canonicalPlatform,
          aliases: meta.aliases,
          displayName: meta.displayName,
          sourceType: 'html',
          sourceUrl: config.sourceUrl,
          items: entries
            .map((entry, index) => config.mapItem(entry, index, canonicalPlatform, $))
            .filter(Boolean)
        });
      }
    };
  };
}
