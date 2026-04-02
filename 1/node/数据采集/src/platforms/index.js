import { env as sharedEnv } from '../config/env.js';
import { requestJson, requestText } from '../core/httpClient.js';
import { requestRenderedHtml } from '../core/browserClient.js';
import { createBaiduCrawler } from './baidu.js';
import { createShaoshupaiCrawler } from './shaoshupai.js';
import { createWeiboCrawler } from './weibo.js';
import { createZhihuCrawler } from './zhihu.js';
import { create36KrCrawler } from './36kr.js';
import { create52pojieCrawler } from './52pojie.js';
import { createBilibiliCrawler } from './bilibili.js';
import { createDoubanCrawler } from './douban.js';
import { createHupuCrawler } from './hupu.js';
import { createTiebaCrawler } from './tieba.js';
import { createJuejinCrawler } from './juejin.js';
import { createDouyinCrawler } from './douyin.js';
import { createV2exCrawler } from './v2ex.js';
import { createJinritoutiaoCrawler } from './jinritoutiao.js';
import { createTenxunwangCrawler } from './tenxunwang.js';
import { createStackoverflowCrawler } from './stackoverflow.js';
import { createGithubCrawler } from './github.js';
import { createHackernewsCrawler } from './hackernews.js';
import { createSinaFinanceCrawler } from './sina_finance.js';
import { createEastmoneyCrawler } from './eastmoney.js';
import { createXueqiuCrawler } from './xueqiu.js';
import { createClsCrawler } from './cls.js';

function createRequestDeps(env = sharedEnv) {
  const defaultHeaders = { 'user-agent': env.defaultUserAgent };

  return {
    browserEnabled: env.enableBrowserFallback,
    requestJson: (url, options = {}) =>
      requestJson(url, {
        timeoutMs: env.httpTimeoutMs,
        retryCount: env.httpRetryCount,
        retryBaseMs: env.httpRetryBaseMs,
        ...options,
        headers: {
          ...defaultHeaders,
          ...(url.includes('weibo.com') && env.weiboCookie ? { cookie: env.weiboCookie } : {}),
          ...(url.includes('xueqiu.com') && env.xueqiuCookie ? { cookie: env.xueqiuCookie } : {}),
          ...(options.headers ?? {})
        }
      }),
    requestText: (url, options = {}) =>
      requestText(url, {
        timeoutMs: env.httpTimeoutMs,
        retryCount: env.httpRetryCount,
        retryBaseMs: env.httpRetryBaseMs,
        ...options,
        headers: {
          ...defaultHeaders,
          ...(options.headers ?? {})
        }
      }),
    requestRenderedHtml: (url, options = {}) =>
      requestRenderedHtml(url, {
        timeoutMs: options.timeoutMs ?? env.httpTimeoutMs,
        userAgent: env.defaultUserAgent
      })
  };
}

/**
 * Create the full platform registry.
 * @param {object} runtimeEnv
 * @returns {Record<string, { collect(input: object): Promise<object> }>}
 */
export function createPlatformRegistry(runtimeEnv = sharedEnv) {
  const deps = createRequestDeps(runtimeEnv);

  return {
    baidu: createBaiduCrawler(deps),
    shaoshupai: createShaoshupaiCrawler(deps),
    weibo: createWeiboCrawler(deps),
    zhihu: createZhihuCrawler(deps),
    '36kr': create36KrCrawler(deps),
    '52pojie': create52pojieCrawler(deps),
    bilibili: createBilibiliCrawler(deps),
    douban: createDoubanCrawler(deps),
    hupu: createHupuCrawler(deps),
    tieba: createTiebaCrawler(deps),
    juejin: createJuejinCrawler(deps),
    douyin: createDouyinCrawler(deps),
    v2ex: createV2exCrawler(deps),
    jinritoutiao: createJinritoutiaoCrawler(deps),
    tenxunwang: createTenxunwangCrawler(deps),
    stackoverflow: createStackoverflowCrawler(deps),
    github: createGithubCrawler(deps),
    hackernews: createHackernewsCrawler(deps),
    sina_finance: createSinaFinanceCrawler(deps),
    eastmoney: createEastmoneyCrawler(deps),
    xueqiu: createXueqiuCrawler(deps),
    cls: createClsCrawler(deps)
  };
}
