import { useEffect, useMemo, useState } from 'react';
import PauseCircleOutlineRoundedIcon from '@mui/icons-material/PauseCircleOutlineRounded';
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded';
import RestartAltRoundedIcon from '@mui/icons-material/RestartAltRounded';
import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import CircularProgress from '@mui/material/CircularProgress';
import LinearProgress from '@mui/material/LinearProgress';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { ApiError, getSystemStatus, listArticles, pauseSystem, resumeSystem, startSystem } from '../../lib/api/client';
import type { SourceDocument, SystemControlState } from '../../lib/api/types';
import MetricCards from '../../components/MetricCards';
import PageCard from '../../components/PageCard';
import PageToolbar from '../../components/PageToolbar';
import StatusChip from '../../components/StatusChip';
import type { AppPage } from '../../layout/AppShell';

type ControlPageProps = {
  onNavigate?: (page: AppPage) => void;
};

export default function ControlPage({ onNavigate }: ControlPageProps) {
  const [systemState, setSystemState] = useState<SystemControlState | null>(null);
  const [articles, setArticles] = useState<SourceDocument[]>([]);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [concurrencyLimit, setConcurrencyLimit] = useState('2');

  const loadData = async () => {
    setLoading(true);
    setError(null);

    try {
      const [state, queue] = await Promise.all([getSystemStatus(), listArticles()]);
      setSystemState(state);
      setArticles(queue);
    } catch (apiError) {
      setError(apiError instanceof ApiError ? apiError.message : '系统状态加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadData();
  }, []);

  const runtimeState = systemState?.state ?? 'stopped';

  const stateSummary = useMemo(() => {
    if (runtimeState === 'running') {
      return {
        chipStatus: 'active' as const,
        chipLabel: '主链路运行中',
        headline: '自动改写主链路正在运行',
        description: '当前状态来自系统控制接口，页面仅提交启动、暂停请求与恢复请求。',
      };
    }

    if (runtimeState === 'paused') {
      return {
        chipStatus: 'pending' as const,
        chipLabel: '主链路已暂停',
        headline: '系统处于暂停观察态',
        description: '当前运行已进入暂停态，满足后端条件时才可恢复进入队列。',
      };
    }

    return {
      chipStatus: 'disabled' as const,
      chipLabel: '主链路未启动',
      headline: '系统等待启动',
        description: '系统当前未运行，可设置并发上限后请求启动主链路。',
    };
  }, [runtimeState]);

  const pendingCount = articles.filter((item) => item.status === 'pending').length;
  const processingCount = articles.filter((item) => item.status === 'processing' || item.status === 'claimed').length;
  const failedCount = articles.filter((item) => item.status === 'failed').length;
  const activeConcurrency = Number(systemState?.metadata?.concurrency_limit ?? 0);

  const metrics = [
    { key: 'runtime', label: '当前状态', value: runtimeState === 'running' ? '运行中' : runtimeState === 'paused' ? '已暂停' : '待启动', hint: '由系统状态接口返回', icon: <PlayArrowRoundedIcon fontSize="small" /> },
    { key: 'queue', label: '待处理任务', value: String(pendingCount), hint: '来源于文章队列', icon: <RestartAltRoundedIcon fontSize="small" /> },
    { key: 'active', label: '处理中任务', value: String(processingCount), hint: '包含 processing / claimed', icon: <Chip size="small" label="任务" /> },
    { key: 'alerts', label: '失败提醒', value: String(failedCount), hint: '来源于失败状态文章数量', icon: <PauseCircleOutlineRoundedIcon fontSize="small" /> },
  ];

  const handleStart = async () => {
    setActionLoading(true);
    setError(null);
    setSuccessMessage(null);

    try {
      const nextState = await startSystem({ concurrency_limit: Math.max(1, Number(concurrencyLimit) || 1) });
      setSystemState(nextState);
      setSuccessMessage('已提交启动请求。');
    } catch (apiError) {
      setError(apiError instanceof ApiError ? apiError.message : '启动流程失败');
    } finally {
      setActionLoading(false);
    }
  };

  const handlePause = async () => {
    setActionLoading(true);
    setError(null);
    setSuccessMessage(null);

    try {
      const nextState = await pauseSystem();
      setSystemState(nextState);
      setSuccessMessage('已提交暂停请求，运行中的任务会协作进入暂停态。');
    } catch (apiError) {
      setError(apiError instanceof ApiError ? apiError.message : '暂停流程失败');
    } finally {
      setActionLoading(false);
    }
  };

  const handleResume = async () => {
    setActionLoading(true);
    setError(null);
    setSuccessMessage(null);

    try {
      const nextState = await resumeSystem();
      setSystemState(nextState);
      setSuccessMessage('已提交恢复请求。');
    } catch (apiError) {
      setError(apiError instanceof ApiError ? apiError.message : '恢复流程失败');
    } finally {
      setActionLoading(false);
    }
  };

  const stages = [
    { key: 'intake', label: '导入接收', progress: articles.length > 0 ? 100 : 10, detail: `当前队列文章数 ${articles.length}。` },
    { key: 'rewrite', label: '自动改写', progress: articles.length > 0 ? Math.min(100, Math.round((processingCount / Math.max(articles.length, 1)) * 100) + (runtimeState === 'running' ? 30 : 0)) : 0, detail: `处理中 ${processingCount} 条，失败 ${failedCount} 条。` },
    { key: 'render', label: '草稿渲染', progress: articles.length > 0 ? Math.round((articles.filter((item) => item.status === 'completed').length / Math.max(articles.length, 1)) * 100) : 0, detail: `已完成 ${articles.filter((item) => item.status === 'completed').length} 条。` },
  ];

  return (
    <Stack spacing={3}>
      <PageToolbar
        title="流程控制"
        description="面向运营值守的控制页，展示系统状态、启动/暂停/恢复请求入口与队列摘要。"
        leading={<StatusChip status={stateSummary.chipStatus} label={stateSummary.chipLabel} />}
        actions={
          <>
            <Button variant="outlined" onClick={() => onNavigate?.('overview')}>
              返回总览
            </Button>
            <Button variant="text" onClick={() => onNavigate?.('config')}>
              查看配置
            </Button>
          </>
        }
        filters={
          <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.25} alignItems={{ xs: 'flex-start', md: 'center' }}>
            <StatusChip status="completed" label="已接入控制 API" />
            <Typography variant="body2" color="text.secondary">
              最近更新：{systemState?.updated_at ? new Date(systemState.updated_at).toLocaleString('zh-CN', { hour12: false }) : '未加载'}
            </Typography>
            <Button size="small" variant="outlined" onClick={() => void loadData()} disabled={loading || actionLoading}>
              刷新状态
            </Button>
          </Stack>
        }
      />

      {error ? <Alert severity="error">{error}</Alert> : null}
      {successMessage ? <Alert severity="success">{successMessage}</Alert> : null}

      <MetricCards items={metrics} />

      <Stack direction={{ xs: 'column', xl: 'row' }} spacing={3} alignItems="stretch">
        <Stack spacing={3} flex={1.2} minWidth={0}>
          <PageCard
            title="系统状态"
            description={stateSummary.description}
            action={loading ? <CircularProgress size={18} /> : <StatusChip status={stateSummary.chipStatus} label={stateSummary.chipLabel} />}
          >
            <Stack spacing={2}>
              <Typography variant="h4">{stateSummary.headline}</Typography>
              <TextField
                label="启动并发上限"
                value={concurrencyLimit}
                onChange={(event) => setConcurrencyLimit(event.target.value)}
                disabled={runtimeState === 'running' || actionLoading}
                helperText={`当前系统记录并发上限：${activeConcurrency || '未设置'}，仅在发起启动请求时提交。`}
              />
              <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.25}>
                <Button variant="contained" startIcon={<PlayArrowRoundedIcon />} disabled={runtimeState === 'running' || actionLoading} onClick={() => void handleStart()}>
                  请求启动
                </Button>
                <Button variant="outlined" color="warning" startIcon={<PauseCircleOutlineRoundedIcon />} disabled={runtimeState !== 'running' || actionLoading} onClick={() => void handlePause()}>
                  请求暂停
                </Button>
                <Button variant="outlined" startIcon={<RestartAltRoundedIcon />} disabled={runtimeState !== 'paused' || actionLoading} onClick={() => void handleResume()}>
                  尝试恢复
                </Button>
              </Stack>
              <Typography variant="body2" color="text.secondary">
                操作记录中的执行人由后端决定，页面仅负责调用现有系统控制接口并展示返回状态。
              </Typography>
            </Stack>
          </PageCard>

          <PageCard
            title="主链路观察"
            description="基于系统状态与文章队列摘要展示导入、改写、草稿渲染三个关键阶段。"
            action={<StatusChip status="pending" label="实时摘要" />}
          >
            <Stack spacing={2}>
              {stages.map((stage) => (
                <Stack key={stage.key} spacing={1}>
                  <Stack direction="row" justifyContent="space-between" alignItems="center">
                    <Typography variant="subtitle1">{stage.label}</Typography>
                    <Typography variant="body2" color="text.secondary">
                      {stage.progress}%
                    </Typography>
                  </Stack>
                  <LinearProgress variant="determinate" value={stage.progress} sx={{ height: 10, borderRadius: 999 }} />
                  <Typography variant="body2" color="text.secondary">
                    {stage.detail}
                  </Typography>
                </Stack>
              ))}
            </Stack>
          </PageCard>
        </Stack>

        <Stack spacing={3} flex={0.9} minWidth={{ xl: 340 }}>
          <PageCard
            title="值守提醒"
            description="基于当前状态保留系统值守的快速观察位。"
            action={<StatusChip status="active" label="持续观察" />}
          >
            <Stack spacing={1.5}>
              {[
                `系统状态：${runtimeState}`,
                `失败文章：${failedCount} 条${failedCount > 0 ? '，建议进入文章队列逐条处理。' : '，当前没有失败积压。'}`,
                `并发上限：${activeConcurrency || Number(concurrencyLimit) || 1}，由系统控制状态 metadata 决定。`,
              ].map((item) => (
                <Stack key={item} spacing={0.5} sx={{ p: 1.75, borderRadius: 3, border: '1px solid', borderColor: 'divider', bgcolor: 'background.default' }}>
                  <Typography variant="subtitle2">控制说明</Typography>
                  <Typography variant="body2" color="text.secondary">
                    {item}
                  </Typography>
                </Stack>
              ))}
            </Stack>
          </PageCard>
        </Stack>
      </Stack>
    </Stack>
  );
}
