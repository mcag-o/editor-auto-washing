import { useMemo, useState } from 'react';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import AutorenewRoundedIcon from '@mui/icons-material/AutorenewRounded';
import BadgeRoundedIcon from '@mui/icons-material/BadgeRounded';
import BookmarkAddedRoundedIcon from '@mui/icons-material/BookmarkAddedRounded';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import PageToolbar from '../../components/PageToolbar';
import StatusChip from '../../components/StatusChip';
import TemplateEditorDrawer from './components/TemplateEditorDrawer';
import TemplateList from './components/TemplateList';
import TemplatePreview from './components/TemplatePreview';

export type TemplateStage = {
  key: string;
  label: string;
  note: string;
};

export type TemplateRecord = {
  id: string;
  name: string;
  version: string;
  enabled: boolean;
  summary: string;
  prompt: string;
  stages: TemplateStage[];
  updatedAt: string;
  updatedBy: string;
};

export type TemplateDraft = {
  name: string;
  version: string;
  enabled: boolean;
  summary: string;
  prompt: string;
  stagesText: string;
};

function createLocalId(prefix: string) {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return `${prefix}-${crypto.randomUUID()}`;
  }

  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

const initialTemplates: TemplateRecord[] = [
  {
    id: 'template-standard',
    name: '标准改写模板',
    version: 'v1.3.0',
    enabled: true,
    summary: '默认启用的中文改写模板，覆盖导入后草稿生成主链路。',
    prompt: '请保留事实信息与语气边界，将文章改写成更适合发布的中文稿件。',
    stages: [
      { key: 'stage-brief', label: '摘要校准', note: '先提炼原文主旨与约束。' },
      { key: 'stage-rewrite', label: '正文改写', note: '围绕平台风格进行重写。' },
      { key: 'stage-check', label: '发布检查', note: '检查敏感表达和语义偏移。' },
    ],
    updatedAt: '今天 21:40',
    updatedBy: '系统',
  },
  {
    id: 'template-review',
    name: '人工复核模板',
    version: 'v2.0.1',
    enabled: false,
    summary: '适用于高风险主题的模板草案，保留更严格的审核说明。',
    prompt: '在输出前加入人工审核提示，并保持措辞保守、可追踪。',
    stages: [
      { key: 'stage-annotate', label: '标注风险', note: '标记需要人工复核的段落。' },
      { key: 'stage-handoff', label: '人工接管', note: '输出给审核人员确认。' },
    ],
    updatedAt: '今天 18:05',
    updatedBy: '运营',
  },
  {
    id: 'template-brief',
    name: '短文卡片模板',
    version: 'v1.0.4',
    enabled: true,
    summary: '用于短内容与标题卡片的轻量化模板。',
    prompt: '压缩内容长度，保留关键信息，并适配卡片式展示。',
    stages: [
      { key: 'stage-condense', label: '内容压缩', note: '删去冗余解释，保留核心事实。' },
      { key: 'stage-card', label: '卡片排版', note: '输出更适合列表阅读的结构。' },
    ],
    updatedAt: '昨天 23:20',
    updatedBy: '编辑',
  },
];

function buildDraft(template: TemplateRecord): TemplateDraft {
  return {
    name: template.name,
    version: template.version,
    enabled: template.enabled,
    summary: template.summary,
    prompt: template.prompt,
    stagesText: template.stages.map((stage) => `${stage.label}: ${stage.note}`).join('\n'),
  };
}

function draftToStages(stagesText: string): TemplateStage[] {
  return stagesText
    .split('\n')
    .map((line, index) => line.trim())
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

export default function TemplatesPage() {
  const [templates, setTemplates] = useState<TemplateRecord[]>(initialTemplates);
  const [selectedId, setSelectedId] = useState(initialTemplates[0].id);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<TemplateDraft>(() => buildDraft(initialTemplates[0]));

  const selectedTemplate = useMemo(
    () => templates.find((item) => item.id === selectedId) ?? templates[0] ?? null,
    [selectedId, templates],
  );

  const selectedDraftTarget = editingId ? templates.find((item) => item.id === editingId) ?? null : null;

  const handleOpenCreate = () => {
    setEditingId(null);
    setDraft({
      name: '新模板',
      version: `v${templates.length + 1}.0.0`,
      enabled: false,
      summary: '请输入模板摘要，说明它适用的场景。',
      prompt: '在这里填写模板提示词。',
      stagesText: '起始阶段: 描述第一步的职责。\n输出阶段: 描述最终输出要求。',
    });
    setEditorOpen(true);
  };

  const handleOpenEdit = (templateId: string) => {
    const template = templates.find((item) => item.id === templateId);
    if (!template) {
      return;
    }

    setSelectedId(templateId);
    setEditingId(templateId);
    setDraft(buildDraft(template));
    setEditorOpen(true);
  };

  const handleCloseEditor = () => {
    setEditorOpen(false);
  };

  const handleSaveTemplate = () => {
    const nextTemplate: TemplateRecord = {
      id: editingId ?? createLocalId('template-local'),
      name: draft.name.trim() || '未命名模板',
      version: draft.version.trim() || 'v0.0.0',
      enabled: draft.enabled,
      summary: draft.summary.trim(),
      prompt: draft.prompt.trim(),
      stages: draftToStages(draft.stagesText),
      updatedAt: '刚刚',
      updatedBy: '本地编辑',
    };

    setTemplates((currentTemplates) => {
      if (editingId) {
        return currentTemplates.map((template) => (template.id === editingId ? nextTemplate : template));
      }

      return [nextTemplate, ...currentTemplates];
    });
    setSelectedId(nextTemplate.id);
    setEditorOpen(false);
    setEditingId(nextTemplate.id);
  };

  const handleToggleEnabled = (templateId: string) => {
    setTemplates((currentTemplates) =>
      currentTemplates.map((template) =>
        template.id === templateId
          ? {
              ...template,
              enabled: !template.enabled,
              updatedAt: '刚刚',
              updatedBy: '本地切换',
            }
          : template,
      ),
    );
    setSelectedId(templateId);
  };

  const handleDuplicateTemplate = (templateId: string) => {
    const template = templates.find((item) => item.id === templateId);
    if (!template) {
      return;
    }

    const duplicated: TemplateRecord = {
      ...template,
      id: createLocalId('template-copy'),
      name: `${template.name} 副本`,
      version: template.version,
      enabled: false,
      updatedAt: '刚刚',
      updatedBy: '本地复制',
    };

    setTemplates((currentTemplates) => [duplicated, ...currentTemplates]);
    setSelectedId(duplicated.id);
  };

  const handleSelectTemplate = (templateId: string) => {
    setSelectedId(templateId);
  };

  const handleChangeDraft = <K extends keyof TemplateDraft>(field: K, value: TemplateDraft[K]) => {
    setDraft((currentDraft) => ({
      ...currentDraft,
      [field]: value,
    }));
  };

  return (
    <Stack spacing={3}>
      <PageToolbar
        leading={
          <Stack direction="row" spacing={1} alignItems="center">
            <BookmarkAddedRoundedIcon color="primary" fontSize="small" />
            <Typography variant="overline" color="text.secondary">
              模板管理
            </Typography>
          </Stack>
        }
        title="模板管理"
        description="使用 Material UI Table 组织模板列表、版本与启用状态，并通过本地 Drawer 预览和编辑模板内容。"
        actions={
          <>
            <StatusChip status={selectedTemplate?.enabled ? 'active' : 'disabled'} label={selectedTemplate?.enabled ? '已启用' : '已停用'} />
            <Button variant="contained" startIcon={<AddRoundedIcon />} onClick={handleOpenCreate}>
              新建模板
            </Button>
          </>
        }
        filters={
          <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} alignItems={{ xs: 'flex-start', md: 'center' }}>
            <Stack direction="row" spacing={1} alignItems="center">
              <AutorenewRoundedIcon color="primary" fontSize="small" />
              <Typography variant="body2" color="text.secondary">
                当前仅维护本地草稿和选择状态，尚未接入后端 API。
              </Typography>
            </Stack>
            <Stack direction="row" spacing={1} alignItems="center">
              <BadgeRoundedIcon color="primary" fontSize="small" />
              <Typography variant="body2" color="text.secondary">
                共 {templates.length} 个模板，{templates.filter((template) => template.enabled).length} 个已启用。
              </Typography>
            </Stack>
          </Stack>
        }
      />

      <Box
        sx={{
          display: 'grid',
          gap: 2.5,
          gridTemplateColumns: {
            xs: '1fr',
            lg: 'minmax(0, 1.6fr) minmax(320px, 1fr)',
          },
          alignItems: 'start',
        }}
      >
        <Box>
          <TemplateList
            items={templates}
            selectedId={selectedId}
            onSelectTemplate={handleSelectTemplate}
            onEditTemplate={handleOpenEdit}
            onToggleEnabled={handleToggleEnabled}
            onDuplicateTemplate={handleDuplicateTemplate}
          />
        </Box>
        <Box>
          <TemplatePreview template={selectedTemplate} />
        </Box>
      </Box>

      <TemplateEditorDrawer
        open={editorOpen}
        draft={draft}
        editingTemplate={selectedDraftTarget}
        onClose={handleCloseEditor}
        onChange={handleChangeDraft}
        onSave={handleSaveTemplate}
      />
    </Stack>
  );
}
