export type ApiErrorPayload = {
  error?: string;
  message?: string;
  code?: string;
};

export type ApiEnvelope<T> = {
  data: T;
  message?: string;
};

export type ApiRequestOptions = {
  signal?: AbortSignal;
  headers?: HeadersInit;
};

export type HealthResponse = {
  status: string;
  running?: boolean;
  paused?: boolean;
  started_at?: string;
  updated_at?: string;
};
