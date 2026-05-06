import type { ControlPlaneConfigPayload } from '../api/types';

export type ControlPlaneConfigForm = {
  targetType: string;
  sourceProfile: string;
  renderPlatform: string;
  defaultWorkflowTemplate: string;
  concurrency: string;
  operatorName: string;
  reviewEnabled: boolean;
  draftAutoRender: boolean;
  auditRetentionDays: string;
  notificationChannel: string;
  operatorNote: string;
};

export const defaultControlPlaneConfigForm: ControlPlaneConfigForm = {
  targetType: 'wechat-longform',
  sourceProfile: 'sspai',
  renderPlatform: 'wechat',
  defaultWorkflowTemplate: '',
  concurrency: '2',
  operatorName: 'local-admin',
  reviewEnabled: true,
  draftAutoRender: true,
  auditRetentionDays: '90',
  notificationChannel: '站内提醒',
  operatorNote: '',
};

function stringValue(value: unknown, fallback: string) {
  return typeof value === 'string' && value.trim() ? value : fallback;
}

function booleanValue(value: unknown, fallback: boolean) {
  return typeof value === 'boolean' ? value : fallback;
}

function numberString(value: unknown, fallback: string) {
  return typeof value === 'number' && Number.isFinite(value) ? String(value) : fallback;
}

export function mapApiConfigToForm(
  payload: ControlPlaneConfigPayload,
  defaults: Partial<Pick<ControlPlaneConfigForm, 'defaultWorkflowTemplate'>> = {},
): ControlPlaneConfigForm {
  return {
    targetType: stringValue(payload.target_type, defaultControlPlaneConfigForm.targetType),
    sourceProfile: stringValue(payload.source_profile, defaultControlPlaneConfigForm.sourceProfile),
    renderPlatform: stringValue(payload.render_platform, defaultControlPlaneConfigForm.renderPlatform),
    defaultWorkflowTemplate: stringValue(
      payload.default_workflow_template,
      defaults.defaultWorkflowTemplate ?? defaultControlPlaneConfigForm.defaultWorkflowTemplate,
    ),
    concurrency: numberString(payload.concurrency, defaultControlPlaneConfigForm.concurrency),
    operatorName: stringValue(payload.operator_name, defaultControlPlaneConfigForm.operatorName),
    reviewEnabled: booleanValue(payload.review_enabled, defaultControlPlaneConfigForm.reviewEnabled),
    draftAutoRender: booleanValue(payload.draft_auto_render, defaultControlPlaneConfigForm.draftAutoRender),
    auditRetentionDays: numberString(payload.audit_retention_days, defaultControlPlaneConfigForm.auditRetentionDays),
    notificationChannel: stringValue(payload.notification_channel, defaultControlPlaneConfigForm.notificationChannel),
    operatorNote: stringValue(payload.operator_note, defaultControlPlaneConfigForm.operatorNote),
  };
}

export function mapConfigFormToApi(form: ControlPlaneConfigForm): ControlPlaneConfigPayload {
  return {
    target_type: form.targetType.trim(),
    source_profile: form.sourceProfile.trim(),
    render_platform: form.renderPlatform.trim(),
    default_workflow_template: form.defaultWorkflowTemplate.trim(),
    concurrency: Math.max(1, Number(form.concurrency) || 1),
    operator_name: form.operatorName.trim() || defaultControlPlaneConfigForm.operatorName,
    review_enabled: form.reviewEnabled,
    draft_auto_render: form.draftAutoRender,
    audit_retention_days: Math.max(0, Number(form.auditRetentionDays) || 0),
    notification_channel: form.notificationChannel.trim(),
    operator_note: form.operatorNote,
  };
}
