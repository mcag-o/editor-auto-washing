import { useEffect, useMemo, useState } from 'react';
import AddRoundedIcon from '@mui/icons-material/AddRounded';
import AutorenewRoundedIcon from '@mui/icons-material/AutorenewRounded';
import BadgeRoundedIcon from '@mui/icons-material/BadgeRounded';
import BookmarkAddedRoundedIcon from '@mui/icons-material/BookmarkAddedRounded';
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

export type TemplateStage = {
  key: string;
  label: string;
  note: string;
};

export type TemplateRecord = TemplateFormRecord;
export type TemplateDraft = TemplateFormDraft;

function createEmptyDraft(templateCount: number): TemplateDraft {
  return {
    name: '新模板',
    version: `v${templateCount + 1}.0.0`,
    enabled: false,
    summary: '请输入模板摘要，说明它适用的场景。',
    prompt: '在这里填写模板提示词。',
    stagesText: '起始阶段: 描述第一步的职责。\n输出阶段: 描述最终输出要求。',
    type: 'prompt',
  };
}

function decorateTemplate(template: TemplateFormRecord): TemplateRecord {
  return {
    ...template,
    updatedAt: template.updatedAt ? new Date(template.updatedAt).toLocaleString('zh-CN', { hour12: false }) : '未记录',
  };
}

export default function TemplatesPage() {
  const [templates, setTemplates] = useState<TemplateRecord[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<TemplateDraft>(() => createEmptyDraft(0));
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const loadTemplates = async () => {
    setLoading(true);
    setError(null);

    try {
      const items = await listTemplates();
      const mapped = items.map(mapApiTemplateToForm).map(decorateTemplate);
      setTemplates(mapped);
      setSelectedId((current) => current || mapped[0]?.id || '');
    } catch (apiError) {
      setError(apiError instanceof ApiError ? apiError.message : '模板列表加载失败');
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
    setSaving(true);
    setError(null);
    setSuccessMessage(null);

    const payload = mapTemplateFormToApi(draft, { updatedBy: 'react-webapp' });

    try {
      const saved = editingId ? await updateTemplate(editingId, payload) : await createTemplate(payload);
      const nextTemplate = decorateTemplate(mapApiTemplateToForm(saved));
      setTemplates((currentTemplates) => {
        if (editingId) {
          return currentTemplates.map((template) => (template.id === editingId ? nextTemplate : template));
        }
        return [nextTemplate, ...currentTemplates];
      });
      setSelectedId(nextTemplate.id);
      setSuccessMessage(editingId ? '模板已更新。' : '模板已创建。');
      resetEditorState(editingId ? templates.length : templates.length + 1);
    } catch (apiError) {
      setError(apiError instanceof ApiError ? apiError.message : '模板保存失败');
    } finally {
      setSaving(false);
    }
  };

  const handleToggleEnabled = async (templateId: string) => {
    const template = templates.find((item) => item.id === templateId);
    if (!template) {
      return;
    }

    setError(null);
    setSuccessMessage(null);

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
      const nextTemplate = decorateTemplate(mapApiTemplateToForm(saved));
      setTemplates((currentTemplates) => currentTemplates.map((item) => (item.id === templateId ? nextTemplate : item)));
      setSelectedId(templateId);
      setSuccessMessage(nextTemplate.enabled ? '模板已启用。' : '模板已停用。');
    } catch (apiError) {
      setError(apiError instanceof ApiError ? apiError.message : '模板状态更新失败');
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
  };

  const handleDeleteTemplate = async (templateId: string) => {
    setError(null);
    setSuccessMessage(null);

    try {
      await deleteTemplate(templateId);
      setTemplates((currentTemplates) => currentTemplates.filter((template) => template.id !== templateId));
      if (selectedId === templateId) {
        const fallback = templates.find((item) => item.id !== templateId)?.id ?? '';
        setSelectedId(fallback);
      }
      setSuccessMessage('模板已删除。');
    } catch (apiError) {
      setError(apiError instanceof ApiError ? apiError.message : '模板删除失败');
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
        description="保留现有 Drawer 编辑体验，并接入真实模板列表、创建、更新与删除接口。"
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
                当前模板列表已接入后端接口。
              </Typography>
            </Stack>
            <Stack direction="row" spacing={1} alignItems="center">
              <BadgeRoundedIcon color="primary" fontSize="small" />
              <Typography variant="body2" color="text.secondary">
                共 {templates.length} 个模板，{templates.filter((template) => template.enabled).length} 个已启用。
              </Typography>
            </Stack>
            <Button size="small" variant="outlined" onClick={() => void loadTemplates()} disabled={loading}>
              刷新列表
            </Button>
          </Stack>
        }
      />

      {error ? <Alert severity="error">{error}</Alert> : null}
      {successMessage ? <Alert severity="success">{successMessage}</Alert> : null}

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
        saving={saving}
        onClose={handleCloseEditor}
        onChange={handleChangeDraft}
        onSave={() => {
          void handleSaveTemplate();
        }}
      />
    </Stack>
  );
}
