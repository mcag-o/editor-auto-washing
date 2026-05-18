import type {
ApiEnvelope,
ApiErrorPayload,
ApiRequestOptions,
BrowserArticle,
ArticleQueueActionResponse,
ArticleStagesResponse,
AuditLog,
HealthResponse,
JsonObject,
SystemControlState,
TemplateDefinition,
TemplateDefinitionInput,
  WorkflowDefinition,
  WorkflowDefinitionInput,
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
  put<TResponse, TBody>(path: string, body: TBody, options?: ApiRequestOptions) {
    return request<TResponse>(
      path,
      {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
      },
      options,
    );
  },
  delete<T>(path: string, options?: ApiRequestOptions) {
    return request<T>(path, { method: 'DELETE' }, options);
  },
  upload<T>(path: string, formData: FormData, options?: ApiRequestOptions) {
    return request<T>(
      path,
      {
        method: 'POST',
        body: formData,
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
  return apiClient.get<HealthResponse | ApiEnvelope<HealthResponse>>('/system/status', options);
}

export function uploadIntake(file: File, options?: ApiRequestOptions) {
const formData = new FormData();
formData.set('file', file);
 return apiClient.upload<BrowserArticle>('/intake/upload', formData, options);
}

export function pasteIntake(body: { title: string; body: string }, options?: ApiRequestOptions) {
 return apiClient.post<BrowserArticle, { title: string; body: string }>('/intake/paste', body, options);
}

export function listArticles(options?: ApiRequestOptions) {
 return apiClient.get<ApiEnvelope<BrowserArticle[]>>('/articles', options).then(unwrapEnvelope);
}

export function getArticle(id: string, options?: ApiRequestOptions) {
 return apiClient.get<BrowserArticle>(`/articles/${id}`, options);
}

export function getArticleStages(id: string, options?: ApiRequestOptions) {
  return apiClient.get<ArticleStagesResponse>(`/articles/${id}/stages`, options);
}

export function retryArticle(id: string, options?: ApiRequestOptions) {
  return apiClient.post<ArticleQueueActionResponse, Record<string, never>>(`/articles/${id}/retry`, {}, options);
}

export function stopArticle(id: string, options?: ApiRequestOptions) {
  return apiClient.post<ArticleQueueActionResponse, Record<string, never>>(`/articles/${id}/stop`, {}, options);
}

export function resumeArticle(id: string, options?: ApiRequestOptions) {
  return apiClient.post<ArticleQueueActionResponse, Record<string, never>>(`/articles/${id}/resume`, {}, options);
}

export function deleteArticle(id: string, options?: ApiRequestOptions) {
  return apiClient.delete<void>(`/articles/${id}`, options);
}

export function getSystemStatus(options?: ApiRequestOptions) {
  return apiClient.get<SystemControlState>('/system/status', options);
}

export function startSystem(body: { concurrency_limit: number }, options?: ApiRequestOptions) {
  return apiClient.post<SystemControlState, { concurrency_limit: number }>('/system/start', body, options);
}

export function pauseSystem(options?: ApiRequestOptions) {
  return apiClient.post<SystemControlState, Record<string, never>>('/system/pause', {}, options);
}

export function resumeSystem(options?: ApiRequestOptions) {
  return apiClient.post<SystemControlState, Record<string, never>>('/system/resume', {}, options);
}

export function getConfig(options?: ApiRequestOptions) {
  return apiClient.get<JsonObject>('/config', options);
}

export function updateConfig(config: JsonObject, options?: ApiRequestOptions) {
  return apiClient.put<JsonObject, JsonObject>('/config', config, options);
}

export function listAudit(options?: ApiRequestOptions) {
  return apiClient.get<ApiEnvelope<AuditLog[]>>('/audit', options).then(unwrapEnvelope);
}

export function getAudit(id: string, options?: ApiRequestOptions) {
  return apiClient.get<AuditLog>(`/audit/${id}`, options);
}

export function listWorkflows(options?: ApiRequestOptions) {
  return apiClient.get<ApiEnvelope<WorkflowDefinition[]>>('/workflows', options).then(unwrapEnvelope);
}

export function getWorkflow(id: string, options?: ApiRequestOptions) {
  return apiClient.get<WorkflowDefinition>(`/workflows/${id}`, options);
}

export function createWorkflow(body: WorkflowDefinitionInput, options?: ApiRequestOptions) {
  return apiClient.post<WorkflowDefinition, WorkflowDefinitionInput>('/workflows', body, options);
}

export function updateWorkflow(id: string, body: WorkflowDefinitionInput, options?: ApiRequestOptions) {
  return apiClient.put<WorkflowDefinition, WorkflowDefinitionInput>(`/workflows/${id}`, body, options);
}

export function deleteWorkflow(id: string, options?: ApiRequestOptions) {
  return apiClient.delete<void>(`/workflows/${id}`, options);
}

export function listTemplates(options?: ApiRequestOptions) {
  return apiClient.get<ApiEnvelope<TemplateDefinition[]>>('/templates', options).then(unwrapEnvelope);
}

export function getTemplate(id: string, options?: ApiRequestOptions) {
  return apiClient.get<TemplateDefinition>(`/templates/${id}`, options);
}

export function createTemplate(body: TemplateDefinitionInput, options?: ApiRequestOptions) {
  return apiClient.post<TemplateDefinition, TemplateDefinitionInput>('/templates', body, options);
}

export function updateTemplate(id: string, body: TemplateDefinitionInput, options?: ApiRequestOptions) {
  return apiClient.put<TemplateDefinition, TemplateDefinitionInput>(`/templates/${id}`, body, options);
}

export function deleteTemplate(id: string, options?: ApiRequestOptions) {
  return apiClient.delete<void>(`/templates/${id}`, options);
}
