import type {
  TemplateDefinition,
  TemplateDefinitionInput,
  TemplateStagePayload,
  TemplateVariablesPayload,
} from '../api/types';

export type TemplateFormStage = {
  key: string;
  label: string;
  note: string;
};

export type TemplateFormRecord = {
  id: string;
  name: string;
  version: string;
  enabled: boolean;
  summary: string;
  prompt: string;
  stages: TemplateFormStage[];
  updatedAt: string;
  updatedBy: string;
  type: string;
};

export type TemplateFormDraft = {
  name: string;
  version: string;
  enabled: boolean;
  summary: string;
  prompt: string;
  stagesText: string;
  type: string;
};

export function mapApiTemplateToForm(template: TemplateDefinition): TemplateFormRecord {
  const prompt = template.content ?? '';
  let summary = template.type;
  let stages: TemplateFormStage[] = [{ key: 'stage-1', label: '模板内容', note: '当前模板未提供分阶段结构。' }];

  try {
    const parsed = JSON.parse(template.variables_json || '{}') as TemplateVariablesPayload;
    summary = parsed.summary || summary;
    if (Array.isArray(parsed.stages) && parsed.stages.length > 0) {
      stages = parsed.stages.map((stage, index) => ({
        key: `stage-${index + 1}`,
        label: stage.label?.trim() || `阶段 ${index + 1}`,
        note: stage.note?.trim() || '未填写说明。',
      }));
    }
  } catch {
    summary = template.type;
  }

  return {
    id: template.id,
    name: template.name,
    version: template.version,
    enabled: template.enabled,
    summary,
    prompt,
    stages,
    updatedAt: template.updated_at,
    updatedBy: template.updated_by || 'system',
    type: template.type,
  };
}

export function mapTemplateFormToApi(
  draft: TemplateFormDraft,
  options: { updatedBy: string },
): TemplateDefinitionInput {
  return {
    name: draft.name.trim() || '未命名模板',
    version: draft.version.trim() || 'v0.0.0',
    enabled: draft.enabled,
    type: draft.type.trim() || 'prompt',
    content: draft.prompt.trim(),
    variables_json: {
      summary: draft.summary.trim(),
      stages: parseTemplateStages(draft.stagesText).map((stage) => ({ label: stage.label, note: stage.note } satisfies TemplateStagePayload)),
    },
    updated_by: options.updatedBy,
  };
}

export function buildTemplateDraft(template: TemplateFormRecord): TemplateFormDraft {
  return {
    name: template.name,
    version: template.version,
    enabled: template.enabled,
    summary: template.summary,
    prompt: template.prompt,
    stagesText: template.stages.map((stage) => `${stage.label}: ${stage.note}`).join('\n'),
    type: template.type,
  };
}

export function parseTemplateStages(stagesText: string): TemplateFormStage[] {
  return stagesText
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line, index) => {
      const separatorIndex = line.indexOf(':');
      if (separatorIndex === -1) {
        return {
          key: `stage-${index + 1}`,
          label: line,
          note: '未填写说明。',
        };
      }

      return {
        key: `stage-${index + 1}`,
        label: line.slice(0, separatorIndex).trim(),
        note: line.slice(separatorIndex + 1).trim() || '未填写说明。',
      };
    });
}
