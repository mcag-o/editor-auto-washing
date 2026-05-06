export type JsonPrimitive = string | number | boolean | null;

export type JsonValue = JsonPrimitive | JsonValue[] | { [key: string]: JsonValue };

export type JsonObject = { [key: string]: JsonValue };

export type SelectOption = {
  id: string;
  label: string;
};

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

export type SourceDocument = {
  id: string;
  source_type: string;
  original_filename: string;
  original_path: string;
  archived_path: string;
  file_type: string;
  title: string;
  body: string;
  summary: string;
  metadata: Record<string, unknown>;
  hash: string;
  imported_at: string | null;
  status: string;
  workspace_article_id: string;
  rewrite_run_id: string;
  claimed_by: string;
  claimed_at: string | null;
  processing_started_at: string | null;
  completed_at: string | null;
  error_summary: string;
};

export type RewritePipelineRun = {
  id: string;
  profile_id: string;
  profile_version: string;
  workspace_article_id: string;
  collector_article_id: string;
  target_type: string;
  source_profile: string;
  status: string;
  current_stage: string;
  started_at: string;
  completed_at: string | null;
  final_draft_id: string;
  error_summary: string;
  metadata: Record<string, unknown>;
};

export type RewriteStageRun = {
  id: string;
  pipeline_run_id: string;
  stage_name: string;
  stage_type: string;
  prompt_ref: string;
  llm_profile_ref: string;
  status: string;
  attempt: number;
  input_json: string;
  output_json: string;
  error_summary: string;
  metadata: Record<string, unknown>;
  started_at: string;
  completed_at: string | null;
};

export type ArticleStagesResponse = {
  article: SourceDocument;
  run: RewritePipelineRun | null;
  stages: RewriteStageRun[];
};

export type ArticleQueueActionResponse = {
  status: string;
  message: string;
  worker_running?: boolean;
  system_state?: string;
  requested_pause?: boolean;
  article: SourceDocument;
};

export type SystemControlState = {
  id: string;
  state: string;
  reason: string;
  metadata: Record<string, unknown>;
  updated_by: string;
  requested_at: string | null;
  updated_at: string;
};

export type ControlPlaneConfigPayload = {
  target_type?: string;
  source_profile?: string;
  render_platform?: string;
  default_workflow_template?: string;
  concurrency?: number;
  operator_name?: string;
  review_enabled?: boolean;
  draft_auto_render?: boolean;
  audit_retention_days?: number;
  notification_channel?: string;
  operator_note?: string;
};

export type AuditLog = {
  id: string;
  actor: string;
  action: string;
  resource: string;
  resource_id: string;
  result: string;
  message: string;
  metadata: Record<string, unknown>;
  created_at: string;
};

export type WorkflowNodeDefinition = {
  id: string;
  type: string;
  name: string;
  config_json: string;
};

export type WorkflowNodePosition = {
  x: number;
  y: number;
};

export type WorkflowNodeConfigPayload = {
  label?: string;
  type?: string;
  template?: string;
  model?: string;
  context?: string;
  position?: WorkflowNodePosition;
};

export type WorkflowEdgeDefinition = {
  from_node_id: string;
  to_node_id: string;
  condition: string;
  priority: number;
};

export type WorkflowDefinition = {
  id: string;
  name: string;
  description: string;
  version: string;
  enabled: boolean;
  entry_node_id: string;
  nodes: WorkflowNodeDefinition[];
  edges: WorkflowEdgeDefinition[];
  updated_by: string;
  updated_at: string;
};

export type WorkflowDefinitionInput = {
  id?: string;
  name: string;
  description: string;
  version: string;
  enabled: boolean;
  entry_node_id: string;
  nodes: WorkflowNodeDefinition[];
  edges: WorkflowEdgeDefinition[];
  updated_by: string;
};

export type TemplateDefinition = {
  id: string;
  name: string;
  type: string;
  version: string;
  enabled: boolean;
  content: string;
  variables_json: string | JsonValue;
  updated_by: string;
  updated_at: string;
};

export type TemplateStagePayload = {
  label: string;
  note: string;
};

export type TemplateVariablesPayload = {
  summary?: string;
  stages?: TemplateStagePayload[];
};

export type TemplateDefinitionInput = {
  id?: string;
  name: string;
  type: string;
  version: string;
  enabled: boolean;
  content: string;
  variables_json: JsonValue;
  updated_by: string;
};
