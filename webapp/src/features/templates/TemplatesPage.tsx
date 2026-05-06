import { useMemo, useState } from 'react';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import BadgeRoundedIcon from '@mui/icons-material/BadgeRounded';
import BookmarkAddedRoundedIcon from '@mui/icons-material/BookmarkAddedRounded';
import PreviewRoundedIcon from '@mui/icons-material/PreviewRounded';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import PageToolbar from '../../components/PageToolbar';
import StatusChip from '../../components/StatusChip';
import { buildTemplateDraft, parseTemplateStages, type TemplateFormDraft, type TemplateFormRecord } from '../../lib/mappers/template';
import TemplateEditorDrawer from './components/TemplateEditorDrawer';
import TemplateList from './components/TemplateList';
import TemplatePreview from './components/TemplatePreview';

export type TemplateRecord = TemplateFormRecord;
export type TemplateDraft = TemplateFormDraft;

const mockTemplates: TemplateRecord[] = [
  {
    id: 'template-brand-core',
    name: '品牌改写主模板',
    version: 'v2.1.0',
    enabled: true,
    summary: '适用于品牌稿、产品介绍和专题导语的主模板，强调结构统一、语气稳定和结尾 CTA。',
    prompt:
      '你是一名中文内容编辑，请基于原始素材输出更适合公众号和站内专题页的品牌稿。保持事实准确，标题克制，首段快速交代价值点，并在结尾补充可执行行动建议。',
    stages: [
      { key: 'stage-1', label: '语境校准', note: '识别品牌主体、用户对象和稿件目标，统一称呼与语气。' },
      { key: 'stage-2', label: '主文改写', note: '输出清晰的三段式正文，突出卖点与差异点。' },
      { key: 'stage-3', label: '结尾补强', note: '补充 CTA、适配渠道标签，并检查语义重复。' },
    ],
    updatedAt: '2026-05-06 20:30:00',
    updatedBy: '运营编辑组',
    type: 'prompt',
  },
  {
    id: 'template-social-variant',
    name: '多语气社媒扩写',
    version: 'v1.4.2',
    enabled: false,
    summary: '用于同一素材生成轻快、专业、讨论感三种社媒版本，便于投放前挑选。',
    prompt:
      '请围绕原始内容输出 3 个版本，分别对应轻快分享、专业分析、讨论引导。每个版本保持 120 到 180 字，避免夸张承诺，突出可传播句子。',
    stages: [
      { key: 'stage-1', label: '关键信息提炼', note: '抓取一句核心观点和两个辅助事实。' },
      { key: 'stage-2', label: '语气分支扩写', note: '按三种渠道语气拆分生成文案。' },
      { key: 'stage-3', label: '发布前检查', note: '检查敏感表达、字数和可读性。' },
    ],
    updatedAt: '2026-05-05 09:12:00',
    updatedBy: '内容策略组',
    type: 'stage',
  },
  {
    id: 'template-review-safe',
    name: '审校兜底模板',
    version: 'v1.0.3',
    enabled: true,
    summary: '在渲染前做事实、风格和禁用词复核，适合作为末端兜底模板。',
    prompt:
      '你是质量审校助手。请核对文稿中的事实陈述、品牌称呼、日期数字和不当承诺，列出需要修订的位置，并给出简明替代建议。',
    stages: [
      { key: 'stage-1', label: '事实核查', note: '关注日期、数据、主体名称和结论表述。' },
      { key: 'stage-2', label: '风格统一', note: '统一品牌语气、标点与标题格式。' },
    ],
    updatedAt: '2026-05-04 16:48:00',
    updatedBy: '审核流程模板',
    type: 'review',
  },
];

function createEmptyDraft(templateCount: number): TemplateDraft {
  return {
    name: '新模板',
    version: `v${templateCount + 1}.0.0`,
    enabled: false,
    summary: '请输入模板摘要，说明适用场景、目标渠道和输出风格。',
    prompt: '在这里填写模板提示词，后续会接入真实后端保存。',
    stagesText: '输入理解: 说明第一阶段要做什么。\n输出整理: 说明最终输出要求。',
    type: 'prompt',
  };
}

function createLocalId() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return `template-local-${crypto.randomUUID()}`;
  }

  return `template-local-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

function draftToRecord(draft: TemplateDraft, existing?: TemplateRecord): TemplateRecord {
  return {
    id: existing?.id ?? createLocalId(),
    name: draft.name.trim() || '未命名模板',
    version: draft.version.trim() || 'v0.0.0',
    enabled: draft.enabled,
    summary: draft.summary.trim() || '未填写模板摘要。',
    prompt: draft.prompt.trim() || '未填写主提示词。',
    stages: parseTemplateStages(draft.stagesText),
    updatedAt: new Date().toLocaleString('zh-CN', { hour12: false }),
    updatedBy: '本地交互演示',
    type: draft.type.trim() || 'prompt',
  };
}

export default function TemplatesPage() {
  const [templates, setTemplates] = useState<TemplateRecord[]>(mockTemplates);
  const [selectedId, setSelectedId] = useState<string>(mockTemplates[0]?.id ?? '');
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<TemplateDraft>(() => createEmptyDraft(mockTemplates.length));
  const [bannerMessage, setBannerMessage] = useState<string | null>('当前页面仅演示本地模板交互，不会请求后端接口。');

  const selectedTemplate = useMemo(
    () => templates.find((item) => item.id === selectedId) ?? templates[0] ?? null,
    [selectedId, templates],
  );

  const selectedDraftTarget = editingId ? templates.find((item) => item.id === editingId) ?? null : null;

  const resetEditorState = (templateCount: number) => {
    setEditorOpen(false);
    setEditingId(null);
    setDraft(createEmptyDraft(templateCount));
  };

  const handleOpenCreate = () => {
    setEditingId(null);
    setDraft(createEmptyDraft(templates.length));
    setEditorOpen(true);
  };

  const handleOpenEdit = (templateId: string) => {
    const template = templates.find((item) => item.id === templateId);
    if (!template) {
      return;
    }

    setSelectedId(templateId);
    setEditingId(templateId);
    setDraft(buildTemplateDraft(template));
    setEditorOpen(true);
  };

  const handleCloseEditor = () => {
    resetEditorState(templates.length);
  };

  const handleSaveTemplate = () => {
    const existing = editingId ? templates.find((item) => item.id === editingId) ?? undefined : undefined;
    const nextTemplate = draftToRecord(draft, existing);

    setTemplates((currentTemplates) => {
      if (editingId) {
        return currentTemplates.map((template) => (template.id === editingId ? nextTemplate : template));
      }

      return [nextTemplate, ...currentTemplates];
    });
    setSelectedId(nextTemplate.id);
    setBannerMessage(editingId ? '已在本地更新模板草稿。' : '已在本地创建模板草稿。');
    resetEditorState(editingId ? templates.length : templates.length + 1);
  };

  const handleToggleEnabled = (templateId: string) => {
    setTemplates((currentTemplates) =>
      currentTemplates.map((item) => {
        if (item.id !== templateId) {
          return item;
        }

        return {
          ...item,
          enabled: !item.enabled,
          updatedAt: new Date().toLocaleString('zh-CN', { hour12: false }),
          updatedBy: '本地交互演示',
        };
      }),
    );
    setSelectedId(templateId);
    setBannerMessage('已在本地切换模板启用状态。');
  };

  const handleDuplicateTemplate = (templateId: string) => {
    const template = templates.find((item) => item.id === templateId);
    if (!template) {
      return;
    }

    setEditingId(null);
    setDraft({
      ...buildTemplateDraft(template),
      name: `${template.name} 副本`,
      enabled: false,
      version: template.version.startsWith('v') ? template.version.replace(/^v/, 'v') : template.version,
    });
    setEditorOpen(true);
    setBannerMessage('已将模板内容复制到本地编辑抽屉。');
  };

  const handleDeleteTemplate = (templateId: string) => {
    const remaining = templates.filter((template) => template.id !== templateId);
    setTemplates(remaining);

    if (selectedId === templateId) {
      setSelectedId(remaining[0]?.id ?? '');
    }

    if (editingId === templateId) {
      resetEditorState(remaining.length);
    }

    setBannerMessage('已从本地列表移除模板。');
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
        description="使用 Material UI 表格、抽屉和预览面板搭建模板管理页外壳，当前仅保留本地交互。"
        actions={
          <>
            <StatusChip status={selectedTemplate?.enabled ? 'active' : 'disabled'} label={selectedTemplate?.enabled ? '已启用' : '已停用'} />
            <Button variant="contained" startIcon={<AddRoundedIcon />} onClick={handleOpenCreate}>
              新建模板
            </Button>
          </>
        }
        filters={
          <Stack direction={{ xs: 'column', lg: 'row' }} spacing={2} alignItems={{ xs: 'flex-start', lg: 'center' }}>
            <Stack direction="row" spacing={1} alignItems="center">
              <PreviewRoundedIcon color="primary" fontSize="small" />
              <Typography variant="body2" color="text.secondary">
                右侧面板实时展示主提示词、阶段结构、版本与启用状态。
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

      {bannerMessage ? <Alert severity="info">{bannerMessage}</Alert> : null}

      <Box
        sx={{
          display: 'grid',
          gap: 2.5,
          gridTemplateColumns: {
            xs: '1fr',
            xl: 'minmax(0, 1.65fr) minmax(340px, 0.95fr)',
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
            onDeleteTemplate={handleDeleteTemplate}
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
