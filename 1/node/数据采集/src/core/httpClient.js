import { fetch } from 'undici';
import { sleep, getBackoffDelay } from './backoff.js';
import { CollectorError, UpstreamHttpError } from './errors.js';

function buildHeaders(headers = {}) {
  return Object.fromEntries(
    Object.entries(headers).filter(([, value]) => value !== undefined && value !== null && value !== '')
  );
}

/**
 * Execute a request with timeout and retry support.
 * @param {string} url
 * @param {object} options
 * @returns {Promise<Response>}
 */
export async function request(url, options = {}) {
  const {
    fetchImpl = fetch,
    method = 'GET',
    headers = {},
    body,
    retryCount = 0,
    retryBaseMs = 100,
    timeoutMs = 10000
  } = options;

  for (let attempt = 0; attempt <= retryCount; attempt += 1) {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), timeoutMs);

    try {
      const response = await fetchImpl(url, {
        method,
        headers: buildHeaders(headers),
        body,
        signal: controller.signal
      });

      if (!response.ok) {
        throw new UpstreamHttpError(`Request failed with status ${response.status}`, response.status);
      }

      return response;
    } catch (error) {
      const wrapped =
        error instanceof CollectorError
          ? error
          : new CollectorError('REQUEST_FAILED', error.message || 'Request failed', {
              retryable: error.name === 'AbortError',
              cause: error
            });

      if (attempt === retryCount || wrapped.retryable === false) {
        throw wrapped;
      }

      await sleep(getBackoffDelay(attempt, retryBaseMs));
    } finally {
      clearTimeout(timeout);
    }
  }

  throw new CollectorError('REQUEST_FAILED', 'Request failed after retries', { retryable: false });
}

/**
 * Request and parse JSON.
 * @param {string} url
 * @param {object} options
 * @returns {Promise<any>}
 */
export async function requestJson(url, options = {}) {
  const response = await request(url, options);
  return response.json();
}

/**
 * Request and return text.
 * @param {string} url
 * @param {object} options
 * @returns {Promise<string>}
 */
export async function requestText(url, options = {}) {
  const response = await request(url, options);
  return response.text();
}
