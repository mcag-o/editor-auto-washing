/**
 * Tiny structured logger for CLI and debugging output.
 */
export const logger = {
  info(message, extra = {}) {
    console.log(JSON.stringify({ level: 'info', message, ...extra }));
  },
  warn(message, extra = {}) {
    console.warn(JSON.stringify({ level: 'warn', message, ...extra }));
  },
  error(message, extra = {}) {
    console.error(JSON.stringify({ level: 'error', message, ...extra }));
  }
};
