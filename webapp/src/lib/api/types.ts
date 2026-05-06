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
  service?: string;
  version?: string;
};

export type SummaryMetric = {
  key: string;
  label: string;
  value: number;
};

export type DashboardSummary = {
  metrics: SummaryMetric[];
  updatedAt?: string;
};
