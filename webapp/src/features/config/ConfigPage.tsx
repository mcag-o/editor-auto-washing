import { useState } from 'react';
import SaveRoundedIcon from '@mui/icons-material/SaveRounded';
import SettingsSuggestRoundedIcon from '@mui/icons-material/SettingsSuggestRounded';
import Button from '@mui/material/Button';
import FormControlLabel from '@mui/material/FormControlLabel';
import MenuItem from '@mui/material/MenuItem';
import Stack from '@mui/material/Stack';
import Switch from '@mui/material/Switch';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
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

const initialState: ConfigState = {
  workspaceName: '主运营空间',
  defaultTemplate: '标准改写链路',
  concurrentJobs: '3',
  reviewEnabled: true,
  draftAutoRender: true,
  auditRetentionDays: '90',
  notificationChannel: '站内提醒',
  operatorNote: '当前页面仅提供 DB 配置编辑壳层，保存动作不会写入后端。',
};

export default function ConfigPage({ onNavigate }: ConfigPageProps) {
  const [config, setConfig] = useState<ConfigState>(initialState);
  const [saveMessage, setSaveMessage] = useState('尚未提交配置变更');

  const handleFieldChange = <K extends keyof ConfigState>(key: K, value: ConfigState[K]) => {
    setConfig((current) => ({ ...current, [key]: value }));
    setSaveMessage('检测到本地未保存修改');
  };

  return (
    <Stack spacing={3}>
      <PageToolbar
        title="系统配置"
        description="展示 DB 驱动配置的可编辑外壳，覆盖工作流、审核与保留策略等核心设置分区。"
        leading={<StatusChip status="pending" label="本地编辑壳层" />}
        actions={
          <>
            <Button variant="outlined" onClick={() => onNavigate?.('workflows')}>
              返回流程控制
            </Button>
            <Button
              variant="contained"
              startIcon={<SaveRoundedIcon />}
              onClick={() => setSaveMessage('已触发本地保存占位动作，后续任务再接入真实写入 API')}
            >
              保存配置
            </Button>
          </>
        }
        filters={
          <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.25} alignItems={{ xs: 'flex-start', md: 'center' }}>
            <StatusChip status="disabled" label="未写入后端" />
            <Typography variant="body2" color="text.secondary">
              配置状态：{saveMessage}
            </Typography>
          </Stack>
        }
      />

      <Stack direction={{ xs: 'column', xl: 'row' }} spacing={3} alignItems="stretch">
        <Stack spacing={3} flex={1.1} minWidth={0}>
          <PageCard
            title="工作流默认设置"
            description="设置默认模板、并发数与草稿结果策略，后续将与数据库配置实体对接。"
            action={<StatusChip status="active" label="可编辑" />}
          >
            <Stack spacing={2}>
              <TextField
                label="工作空间名称"
                value={config.workspaceName}
                onChange={(event) => handleFieldChange('workspaceName', event.target.value)}
              />
              <TextField
                select
                label="默认流程模板"
                value={config.defaultTemplate}
                onChange={(event) => handleFieldChange('defaultTemplate', event.target.value)}
              >
                <MenuItem value="标准改写链路">标准改写链路</MenuItem>
                <MenuItem value="高审校链路">高审校链路</MenuItem>
                <MenuItem value="快速渲染链路">快速渲染链路</MenuItem>
              </TextField>
              <TextField
                label="并发任务上限"
                value={config.concurrentJobs}
                onChange={(event) => handleFieldChange('concurrentJobs', event.target.value)}
              />
              <FormControlLabel
                control={
                  <Switch
                    checked={config.draftAutoRender}
                    onChange={(event) => handleFieldChange('draftAutoRender', event.target.checked)}
                  />
                }
                label="草稿完成后自动进入渲染结果"
              />
            </Stack>
          </PageCard>

          <PageCard
            title="审核与审计策略"
            description="用于承接审核开关、审计保留天数与通知策略的配置编辑入口。"
            action={<StatusChip status="pending" label="等待落库" />}
          >
            <Stack spacing={2}>
              <FormControlLabel
                control={
                  <Switch
                    checked={config.reviewEnabled}
                    onChange={(event) => handleFieldChange('reviewEnabled', event.target.checked)}
                  />
                }
                label="启用人工复核环节"
              />
              <TextField
                label="审计记录保留天数"
                value={config.auditRetentionDays}
                onChange={(event) => handleFieldChange('auditRetentionDays', event.target.value)}
              />
              <TextField
                select
                label="默认通知渠道"
                value={config.notificationChannel}
                onChange={(event) => handleFieldChange('notificationChannel', event.target.value)}
              >
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
            description="当前仅实现可编辑页面结构，后续再接真实加载、校验与保存反馈。"
            action={<StatusChip status="completed" label="结构已预留" />}
          >
            <Stack spacing={2}>
              <Stack direction="row" spacing={1.25} alignItems="center">
                <SettingsSuggestRoundedIcon color="primary" />
                <Typography variant="subtitle1">DB 配置编辑壳层</Typography>
              </Stack>
              <Typography variant="body2" color="text.secondary">
                当前页面面向未来的数据库配置模型设计，只保留输入控件、分区结构与本地提示文案。
              </Typography>
              <TextField
                multiline
                minRows={8}
                label="运维备注"
                value={config.operatorNote}
                onChange={(event) => handleFieldChange('operatorNote', event.target.value)}
              />
            </Stack>
          </PageCard>
        </Stack>
      </Stack>
    </Stack>
  );
}
