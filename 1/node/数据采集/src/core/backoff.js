/**
 * Sleep for a given amount of time.
 * @param {number} ms
 * @returns {Promise<void>}
 */
export async function sleep(ms) {
  await new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Compute an exponential backoff delay with no randomness by default.
 * @param {number} attempt
 * @param {number} baseMs
 * @returns {number}
 */
export function getBackoffDelay(attempt, baseMs) {
  return baseMs * (2 ** attempt);
}
