import type {
  ApiEnvelope,
  ApiErrorPayload,
  ApiRequestOptions,
  DashboardSummary,
  HealthResponse,
} from './types';

export class ApiError extends Error {
  readonly status: number;
  readonly code?: string;

  constructor(status: number, message: string, code?: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

const API_BASE_URL = '/api';

async function readJson<T>(response: Response): Promise<T> {
  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

async function request<T>(path: string, init: RequestInit = {}, options: ApiRequestOptions = {}) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    signal: options.signal,
    headers: {
      Accept: 'application/json',
      ...options.headers,
      ...init.headers,
    },
  });

  if (!response.ok) {
    const payload = await readJson<ApiErrorPayload>(response).catch(() => undefined);
    throw new ApiError(
      response.status,
      payload?.message ?? payload?.error ?? `API request failed: ${response.status}`,
      payload?.code,
    );
  }

  return readJson<T>(response);
}

export const apiClient = {
  get<T>(path: string, options?: ApiRequestOptions) {
    return request<T>(path, { method: 'GET' }, options);
  },
  post<TResponse, TBody>(path: string, body: TBody, options?: ApiRequestOptions) {
    return request<TResponse>(
      path,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
      },
      options,
    );
  },
};

export function unwrapEnvelope<T>(payload: T | ApiEnvelope<T>): T {
  if (payload && typeof payload === 'object' && 'data' in payload) {
    return payload.data;
  }

  return payload as T;
}

export function getHealth(options?: ApiRequestOptions) {
  return apiClient.get<HealthResponse | ApiEnvelope<HealthResponse>>('/health', options);
}

export function getDashboardSummary(options?: ApiRequestOptions) {
  return apiClient.get<DashboardSummary | ApiEnvelope<DashboardSummary>>('/dashboard/summary', options);
}
