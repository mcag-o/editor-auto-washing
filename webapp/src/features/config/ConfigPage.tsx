import { useEffect, useMemo, useState } from 'react';
import SaveRoundedIcon from '@mui/icons-material/SaveRounded';
import SettingsSuggestRoundedIcon from '@mui/icons-material/SettingsSuggestRounded';
import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import FormControlLabel from '@mui/material/FormControlLabel';
import MenuItem from '@mui/material/MenuItem';
import Stack from '@mui/material/Stack';
import Switch from '@mui/material/Switch';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { ApiError, getConfig, listTemplates, listWorkflows, updateConfig } from '../../lib/api/client';
import type { JsonObject, TemplateDefinition, WorkflowDefinition } from '../../lib/api/types';
import PageCard from '../../components/PageCard';
import PageToolbar from '../../components/PageToolbar';
import StatusChip from '../../components/StatusChip';
import type { AppPage } from '../../layout/AppShell';

type ConfigPageProps = {
  onNavigate?: (page: AppPage) => void;
};

type ConfigState = {
  workspaceName: string;
  defaultTemplate: string;
  concurrentJobs: string;
  reviewEnabled: boolean;
  draftAutoRender: boolean;
  auditRetentionDays: string;
  notificationChannel: string;
  operatorNote: string;
};

function toStringValue(value: unknown, fallback = '') {
  return typeof value === 'string' ? value : fallback;
}

function toBooleanValue(value: unknown, fallback = false) {
  return typeof value === 'boolean' ? value : fallback;
}

export default function ConfigPage({ onNavigate }: ConfigPageProps) {
  const [config, setConfig] = useState<ConfigState>({
    workspaceName: '主运营空间',
    defaultTemplate: '',
    concurrentJobs: '2',
    reviewEnabled: true,
    draftAutoRender: true,
    auditRetentionDays: '90',
    notificationChannel: '站内提醒',
    operatorNote: '',
  });
  const [workflowOptions, setWorkflowOptions] = useState<WorkflowDefinition[]>([]);
  const [templateOptions, setTemplateOptions] = useState<TemplateDefinition[]>([]);
  const [saveMessage, setSaveMessage] = useState('尚未提交配置变更');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();

    const loadData = async () => {
      setLoading(true);
      setError(null);

      try {
        const [payload, workflows, templates] = await Promise.all([
          getConfig({ signal: controller.signal }),
          listWorkflows({ signal: controller.signal }),
          listTemplates({ signal: controller.signal }),
        ]);

        if (controller.signal.aborted) {
          return;
        }

        setWorkflowOptions(workflows);
        setTemplateOptions(templates);
        setConfig({
          workspaceName: toStringValue(payload.workspace_name, '主运营空间'),
          defaultTemplate: toStringValue(payload.default_template, workflows[0]?.id ?? ''),
          concurrentJobs: String(payload.concurrent_jobs ?? payload.concurrency_limit ?? 2),
          reviewEnabled: toBooleanValue(payload.review_enabled, true),
          draftAutoRender: toBooleanValue(payload.draft_auto_render, true),
          auditRetentionDays: String(payload.audit_retention_days ?? 90),
          notificationChannel: toStringValue(payload.notification_channel, '站内提醒'),
          operatorNote: toStringValue(payload.operator_note, ''),
        });
        setSaveMessage('已从真实配置接口加载当前设置');
      } catch (apiError) {
        if (controller.signal.aborted) {
          return;
        }
        setError(apiError instanceof ApiError ? apiError.message : '配置加载失败');
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      }
    };

    void loadData();
    return () => controller.abort();
  }, []);

  const handleFieldChange = <K extends keyof ConfigState>(key: K, value: ConfigState[K]) => {
    setConfig((current) => ({ ...current, [key]: value }));
    setSaveMessage('检测到未保存修改');
  };

  const payload = useMemo<JsonObject>(
    () => ({
      workspace_name: config.workspaceName,
      default_template: config.defaultTemplate,
      default_prompt_template: templateOptions.find((item) => item.id === config.defaultTemplate)?.id ?? '',
      concurrent_jobs: Number(config.concurrentJobs) || 1,
      review_enabled: config.reviewEnabled,
      draft_auto_render: config.draftAutoRender,
      audit_retention_days: Number(config.auditRetentionDays) || 0,
      notification_channel: config.notificationChannel,
      operator_note: config.operatorNote,
    }),
    [config, templateOptions],
  );

  const handleSave = async () => {
    setSaving(true);
    setError(null);

    try {
      await updateConfig(payload);
      setSaveMessage('配置已写入后端');
    } catch (apiError) {
      setError(apiError instanceof ApiError ? apiError.message : '配置保存失败');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Stack spacing={3}>
      <PageToolbar
        title="系统配置"
        description="展示 DB 驱动配置的可编辑外壳，现已接入配置读取和保存接口。"
        leading={<StatusChip status={loading ? 'pending' : 'active'} label={loading ? '加载中' : '已连接后端'} />}
        actions={
          <>
            <Button variant="outlined" onClick={() => onNavigate?.('control')}>
              返回流程控制
            </Button>
            <Button variant="contained" startIcon={<SaveRoundedIcon />} onClick={() => void handleSave()} disabled={saving || loading}>
              保存配置
            </Button>
          </>
        }
        filters={
          <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.25} alignItems={{ xs: 'flex-start', md: 'center' }}>
            <StatusChip status="completed" label="已写入后端" />
            <Typography variant="body2" color="text.secondary">
              配置状态：{saveMessage}
            </Typography>
          </Stack>
        }
      />

      {error ? <Alert severity="error">{error}</Alert> : null}

      <Stack direction={{ xs: 'column', xl: 'row' }} spacing={3} alignItems="stretch">
        <Stack spacing={3} flex={1.1} minWidth={0}>
          <PageCard
            title="工作流默认设置"
            description="设置默认模板、并发数与草稿结果策略，并将结果写回配置存储。"
            action={<StatusChip status="active" label="可编辑" />}
          >
            <Stack spacing={2}>
              <TextField label="工作空间名称" value={config.workspaceName} onChange={(event) => handleFieldChange('workspaceName', event.target.value)} disabled={loading} />
              <TextField select label="默认流程模板" value={config.defaultTemplate} onChange={(event) => handleFieldChange('defaultTemplate', event.target.value)} disabled={loading}>
                {workflowOptions.map((option) => (
                  <MenuItem key={option.id} value={option.id}>
                    {option.name}
                  </MenuItem>
                ))}
              </TextField>
              <TextField label="并发任务上限" value={config.concurrentJobs} onChange={(event) => handleFieldChange('concurrentJobs', event.target.value)} disabled={loading} />
              <FormControlLabel control={<Switch checked={config.draftAutoRender} onChange={(event) => handleFieldChange('draftAutoRender', event.target.checked)} disabled={loading} />} label="草稿完成后自动进入渲染结果" />
            </Stack>
          </PageCard>

          <PageCard
            title="审核与审计策略"
            description="用于承接审核开关、审计保留天数与通知策略的配置编辑入口。"
            action={<StatusChip status="pending" label="DB 配置项" />}
          >
            <Stack spacing={2}>
              <FormControlLabel control={<Switch checked={config.reviewEnabled} onChange={(event) => handleFieldChange('reviewEnabled', event.target.checked)} disabled={loading} />} label="启用人工复核环节" />
              <TextField label="审计记录保留天数" value={config.auditRetentionDays} onChange={(event) => handleFieldChange('auditRetentionDays', event.target.value)} disabled={loading} />
              <TextField select label="默认通知渠道" value={config.notificationChannel} onChange={(event) => handleFieldChange('notificationChannel', event.target.value)} disabled={loading}>
                <MenuItem value="站内提醒">站内提醒</MenuItem>
                <MenuItem value="邮件摘要">邮件摘要</MenuItem>
                <MenuItem value="值守群通知">值守群通知</MenuItem>
              </TextField>
            </Stack>
          </PageCard>
        </Stack>

        <Stack spacing={3} flex={0.9} minWidth={{ xl: 360 }}>
          <PageCard
            title="配置说明"
            description="当前仍保持 phase-one 编辑体验，但加载与保存都使用真实配置接口。"
            action={<StatusChip status="completed" label="结构已预留" />}
          >
            <Stack spacing={2}>
              <Stack direction="row" spacing={1.25} alignItems="center">
                <SettingsSuggestRoundedIcon color="primary" />
                <Typography variant="subtitle1">DB 配置编辑入口</Typography>
              </Stack>
              <Typography variant="body2" color="text.secondary">
                当前页面面向数据库配置模型，控件布局保持不变，但会读取 /api/config 并将变更写回后端。
              </Typography>
              <TextField multiline minRows={8} label="运维备注" value={config.operatorNote} onChange={(event) => handleFieldChange('operatorNote', event.target.value)} disabled={loading} />
            </Stack>
          </PageCard>
        </Stack>
      </Stack>
    </Stack>
  );
}
