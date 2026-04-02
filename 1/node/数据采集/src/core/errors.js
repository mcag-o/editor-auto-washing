/**
 * Base collector error for transport and parsing failures.
 */
export class CollectorError extends Error {
  constructor(code, message, options = {}) {
    super(message);
    this.name = 'CollectorError';
    this.code = code;
    this.retryable = Boolean(options.retryable);
    this.cause = options.cause;
    this.statusCode = options.statusCode ?? null;
  }
}

/**
 * Raised when an upstream returns a non-successful HTTP status code.
 */
export class UpstreamHttpError extends CollectorError {
  constructor(message, statusCode, options = {}) {
    super('UPSTREAM_HTTP_ERROR', message, {
      ...options,
      retryable: options.retryable ?? statusCode >= 500,
      statusCode
    });
  }
}

/**
 * Raised when parsing upstream data fails.
 */
export class ParseError extends CollectorError {
  constructor(message, options = {}) {
    super('PARSE_ERROR', message, { ...options, retryable: options.retryable ?? false });
  }
}

/**
 * Raised when a requested platform is not supported.
 */
export class UnsupportedPlatformError extends CollectorError {
  constructor(platform) {
    super('UNSUPPORTED_PLATFORM', `Unsupported platform: ${platform}`, { retryable: false });
  }
}
