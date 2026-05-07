import { useEffect, useMemo, useState } from 'react';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import BadgeRoundedIcon from '@mui/icons-material/BadgeRounded';
import BookmarkAddedRoundedIcon from '@mui/icons-material/BookmarkAddedRounded';
import PreviewRoundedIcon from '@mui/icons-material/PreviewRounded';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { ApiError, createTemplate, deleteTemplate, listTemplates, updateTemplate } from '../../lib/api/client';
import {
  buildTemplateDraft,
  mapApiTemplateToForm,
  mapTemplateFormToApi,
  type TemplateFormDraft,
  type TemplateFormRecord,
} from '../../lib/mappers/template';
import PageToolbar from '../../components/PageToolbar';
import StatusChip from '../../components/StatusChip';
import TemplateEditorDrawer from './components/TemplateEditorDrawer';
import TemplateList from './components/TemplateList';
import TemplatePreview from './components/TemplatePreview';

export type TemplateRecord = TemplateFormRecord;
export type TemplateDraft = TemplateFormDraft;

function createEmptyDraft(templateCount: number): TemplateDraft {
  return {
    name: '新模板',
    version: `v${templateCount + 1}.0.0`,
    enabled: false,
    summary: '说明适用场景、目标渠道和输出风格。',
    prompt: '输入模板提示词。',
    stagesText: '输入理解: 提取原文重点与限制条件。\n输出整理: 生成符合渠道要求的最终内容。',
    type: 'prompt',
  };
}

function isLocalTemplateId(templateId: string) {
  return templateId.startsWith('template-local-');
}

function createLocalId() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return `template-local-${crypto.randomUUID()}`;
  }

  return `template-local-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

export default function TemplatesPage() {
  const [templates, setTemplates] = useState<TemplateRecord[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<TemplateDraft>(createEmptyDraft(0));
  const [bannerMessage, setBannerMessage] = useState<string | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const loadTemplates = async () => {
    setLoading(true);
    setLoadError(null);

    try {
      const items = await listTemplates();
      const mapped = items.map(mapApiTemplateToForm);
      setTemplates(mapped);
      setSelectedId((current) => (mapped.some((item) => item.id === current) ? current : mapped[0]?.id ?? ''));
      setBannerMessage(`已加载 ${mapped.length} 个模板。`);
    } catch (apiError) {
      setLoadError(apiError instanceof ApiError ? apiError.message : '模板列表加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadTemplates();
  }, []);

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

  const handleSaveTemplate = async () => {
    const existing = editingId ? templates.find((item) => item.id === editingId) ?? undefined : undefined;
    const targetId = existing?.id ?? createLocalId();
    const payload = mapTemplateFormToApi(draft, { updatedBy: 'react-webapp' });

    setSaving(true);
    setActionError(null);

    try {
      const saved = !editingId || isLocalTemplateId(targetId)
        ? await createTemplate({ ...payload, id: undefined })
        : await updateTemplate(targetId, payload);
      const nextTemplate = mapApiTemplateToForm(saved);

      setTemplates((currentTemplates) => {
        if (!editingId || isLocalTemplateId(targetId)) {
          return [nextTemplate, ...currentTemplates.filter((item) => item.id !== targetId)];
        }

        return currentTemplates.map((template) => (template.id === targetId ? nextTemplate : template));
      });
      setSelectedId(nextTemplate.id);
      setBannerMessage(!editingId || isLocalTemplateId(targetId) ? '模板已创建。' : '模板已保存。');
      resetEditorState(editingId ? templates.length : templates.length + 1);
    } catch (apiError) {
      setActionError(apiError instanceof ApiError ? apiError.message : '模板保存失败');
    } finally {
      setSaving(false);
    }
  };

  const handleToggleEnabled = async (templateId: string) => {
    const template = templates.find((item) => item.id === templateId);
    if (!template) {
      return;
    }

    setSaving(true);
    setActionError(null);

    try {
      const saved = await updateTemplate(
        templateId,
        mapTemplateFormToApi(
          {
            ...buildTemplateDraft(template),
            enabled: !template.enabled,
          },
          { updatedBy: 'react-webapp' },
        ),
      );
      const nextTemplate = mapApiTemplateToForm(saved);
      setTemplates((currentTemplates) =>
        currentTemplates.map((item) => (item.id === templateId ? nextTemplate : item)),
      );
      setSelectedId(templateId);
      setBannerMessage('模板启用状态已更新。');
    } catch (apiError) {
      setActionError(apiError instanceof ApiError ? apiError.message : '模板状态更新失败');
    } finally {
      setSaving(false);
    }
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
    });
    setEditorOpen(true);
    setBannerMessage('已将模板内容复制到编辑抽屉。');
  };

  const handleDeleteTemplate = async (templateId: string) => {
    const remaining = templates.filter((template) => template.id !== templateId);

    if (isLocalTemplateId(templateId)) {
      setTemplates(remaining);
      if (selectedId === templateId) {
        setSelectedId(remaining[0]?.id ?? '');
      }
      if (editingId === templateId) {
        resetEditorState(remaining.length);
      }
      setBannerMessage('已从列表移除本地模板。');
      return;
    }

    setSaving(true);
    setActionError(null);

    try {
      await deleteTemplate(templateId);
      setTemplates(remaining);
      if (selectedId === templateId) {
        setSelectedId(remaining[0]?.id ?? '');
      }
      if (editingId === templateId) {
        resetEditorState(remaining.length);
      }
      setBannerMessage('模板已删除。');
    } catch (apiError) {
      setActionError(apiError instanceof ApiError ? apiError.message : '模板删除失败');
    } finally {
      setSaving(false);
    }
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

  const enabledCount = templates.filter((template) => template.enabled).length;

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
        description="通过后端模板接口管理模板记录，并在页面内完成列表、预览、编辑、启停与删除操作。"
        actions={
          <>
            <StatusChip status={selectedTemplate?.enabled ? 'active' : 'disabled'} label={selectedTemplate?.enabled ? '已启用' : '已停用'} />
            <Button variant="contained" startIcon={<AddRoundedIcon />} onClick={handleOpenCreate} disabled={saving}>
              新建模板
            </Button>
          </>
        }
        filters={
          <Stack direction={{ xs: 'column', lg: 'row' }} spacing={1.5} alignItems={{ xs: 'flex-start', lg: 'center' }}>
            <Stack direction="row" spacing={1} alignItems="center">
              <PreviewRoundedIcon color="primary" fontSize="small" />
              <Typography variant="body2" color="text.secondary">
                右侧面板显示当前模板的提示词、阶段结构、版本与启用状态。
              </Typography>
            </Stack>
            <Stack direction="row" spacing={1} alignItems="center">
              <BadgeRoundedIcon color="primary" fontSize="small" />
              <Typography variant="body2" color="text.secondary">
                共 {templates.length} 个模板，{enabledCount} 个已启用。
              </Typography>
            </Stack>
          </Stack>
        }
      />

      {actionError ? <Alert severity="error">{actionError}</Alert> : null}
      {bannerMessage ? <Alert severity={loading ? 'info' : 'success'}>{bannerMessage}</Alert> : null}

      <Box
        sx={{
          display: 'grid',
          gap: 2,
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
            loading={loading}
            error={loadError}
            selectedId={selectedId}
            onSelectTemplate={handleSelectTemplate}
            onEditTemplate={handleOpenEdit}
            onToggleEnabled={(templateId) => void handleToggleEnabled(templateId)}
            onDuplicateTemplate={handleDuplicateTemplate}
            onDeleteTemplate={(templateId) => void handleDeleteTemplate(templateId)}
          />
        </Box>
        <Box>
          <TemplatePreview template={selectedTemplate} loading={loading} />
        </Box>
      </Box>

      <TemplateEditorDrawer
        open={editorOpen}
        draft={draft}
        editingTemplate={selectedDraftTarget}
        saving={saving}
        onClose={handleCloseEditor}
        onChange={handleChangeDraft}
        onSave={() => void handleSaveTemplate()}
      />
    </Stack>
  );
}
