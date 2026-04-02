import dotenv from 'dotenv';

dotenv.config();

function toInt(value, fallback) {
  const parsed = Number.parseInt(value ?? '', 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function toBool(value, fallback = false) {
  if (value == null || value === '') {
    return fallback;
  }

  return ['1', 'true', 'yes', 'on'].includes(String(value).toLowerCase());
}

export function loadEnv(raw = process.env) {
  return Object.freeze({
    httpTimeoutMs: toInt(raw.HTTP_TIMEOUT_MS, 10000),
    httpRetryCount: toInt(raw.HTTP_RETRY_COUNT, 2),
    httpRetryBaseMs: toInt(raw.HTTP_RETRY_BASE_MS, 250),
    globalConcurrency: toInt(raw.GLOBAL_CONCURRENCY, 4),
    defaultUserAgent:
      raw.DEFAULT_USER_AGENT ||
      'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36',
    httpProxy: raw.HTTP_PROXY || '',
    httpsProxy: raw.HTTPS_PROXY || '',
    weiboCookie: raw.WEIBO_COOKIE || '',
    xueqiuCookie: raw.XUEQIU_COOKIE || '',
    enableBrowserFallback: toBool(raw.ENABLE_BROWSER_FALLBACK, false)
  });
}

export const env = loadEnv();
