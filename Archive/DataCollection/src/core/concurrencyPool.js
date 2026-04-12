import pLimit from 'p-limit';

/**
 * Build a bounded concurrency pool.
 * @param {number} limit
 * @returns {import('p-limit').LimitFunction}
 */
export function createConcurrencyPool(limit) {
  return pLimit(limit);
}
