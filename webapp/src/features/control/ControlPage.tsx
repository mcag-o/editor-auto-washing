import { useEffect, useMemo, useState } from 'react';
import PauseCircleOutlineRoundedIcon from '@mui/icons-material/PauseCircleOutlineRounded';
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded';
import RestartAltRoundedIcon from '@mui/icons-material/RestartAltRounded';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import CircularProgress from '@mui/material/CircularProgress';
import Divider from '@mui/material/Divider';
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

  const runtimeState = systemState?.state ?? null;

  const stateSummary = useMemo(() => {
    if (loading) {
      return {
        chipStatus: 'pending' as const,
        chipLabel: '状态加载中',
        headline: '正在加载主链路状态',
        description: '正在同步系统控制状态与队列摘要，请稍候。',
      };
    }

    if (error) {
      return {
        chipStatus: 'failed' as const,
        chipLabel: '状态加载失败',
        headline: '主链路状态暂时不可用',
        description: '系统状态加载失败，请刷新后重试。',
      };
    }

    if (runtimeState === 'running') {
      return {
        chipStatus: 'active' as const,
        chipLabel: '主链路运行中',
        headline: '自动改写主链路正在运行',
        description: '当前状态来自系统控制接口，页面用于提交控制请求并查看最近一次返回结果。',
      };
    }

    if (runtimeState === 'paused') {
      return {
        chipStatus: 'pending' as const,
        chipLabel: '主链路已暂停',
        headline: '主链路已暂停',
        description: '系统当前处于暂停状态，可继续查看队列摘要并按需恢复。',
      };
    }

    return {
      chipStatus: 'disabled' as const,
      chipLabel: '主链路未启动',
      headline: '主链路未启动',
      description: '系统当前未运行，可设置并发上限后提交启动请求。',
    };
  }, [error, loading, runtimeState]);

  const pendingCount = articles.filter((item) => item.status === 'pending').length;
  const processingCount = articles.filter((item) => item.status === 'processing' || item.status === 'claimed').length;
  const failedCount = articles.filter((item) => item.status === 'failed').length;
  const activeConcurrency = systemState?.metadata?.concurrency_limit == null ? null : Number(systemState.metadata.concurrency_limit);

  const runtimeMetricValue = loading
    ? '加载中'
    : error
      ? '加载失败'
      : runtimeState === 'running'
        ? '运行中'
        : runtimeState === 'paused'
          ? '已暂停'
          : '待启动';

  const pendingMetricValue = loading ? '加载中' : error ? '加载失败' : String(pendingCount);
  const processingMetricValue = loading ? '加载中' : error ? '加载失败' : String(processingCount);
  const failedMetricValue = loading ? '加载中' : error ? '加载失败' : String(failedCount);

  const metrics = [
    { key: 'runtime', label: '当前状态', value: runtimeMetricValue, hint: '由系统状态接口返回', icon: <PlayArrowRoundedIcon fontSize="small" /> },
    { key: 'queue', label: '待处理任务', value: pendingMetricValue, hint: '来源于文章队列', icon: <RestartAltRoundedIcon fontSize="small" /> },
    { key: 'active', label: '处理中任务', value: processingMetricValue, hint: '包含 processing / claimed', icon: <Chip size="small" label="任务" /> },
    { key: 'alerts', label: '失败提醒', value: failedMetricValue, hint: '来源于失败状态文章数量', icon: <PauseCircleOutlineRoundedIcon fontSize="small" /> },
  ];

  const completedCount = articles.filter((item) => item.status === 'completed').length;

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
    {
      key: 'intake',
      label: '导入接收',
      progress: loading || error ? 0 : articles.length > 0 ? 100 : 10,
      detail: loading ? '正在加载队列摘要。' : error ? '队列摘要暂时不可用。' : `当前队列文章数 ${articles.length}。`,
    },
    {
      key: 'rewrite',
      label: '自动改写',
      progress: loading || error ? 0 : articles.length > 0 ? Math.min(100, Math.round((processingCount / Math.max(articles.length, 1)) * 100) + (runtimeState === 'running' ? 30 : 0)) : 0,
      detail: loading ? '正在加载处理进度。' : error ? '处理进度暂时不可用。' : `处理中 ${processingCount} 条，失败 ${failedCount} 条。`,
    },
    {
      key: 'render',
      label: '草稿渲染',
      progress: loading || error ? 0 : articles.length > 0 ? Math.round((articles.filter((item) => item.status === 'completed').length / Math.max(articles.length, 1)) * 100) : 0,
      detail: loading ? '正在加载完成进度。' : error ? '完成进度暂时不可用。' : `已完成 ${articles.filter((item) => item.status === 'completed').length} 条。`,
    },
  ];

  return (
    <Stack spacing={3}>
      <PageToolbar
        title="流程控制"
        description="管理主链路运行状态，并查看系统状态与队列摘要。"
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
              状态更新时间：{loading ? '加载中' : error ? '加载失败' : systemState?.updated_at ? new Date(systemState.updated_at).toLocaleString('zh-CN', { hour12: false }) : '未记录'}
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
            title="运行状态与控制"
            description={stateSummary.description}
            action={loading ? <CircularProgress size={18} /> : <StatusChip status={stateSummary.chipStatus} label={stateSummary.chipLabel} />}
          >
            <Stack spacing={2}>
              <Typography variant="h4">{stateSummary.headline}</Typography>

              <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.25} flexWrap="wrap" useFlexGap>
                {[
                  { label: '当前状态', value: loading ? '加载中' : error ? '加载失败' : runtimeState === 'running' ? '运行中' : runtimeState === 'paused' ? '已暂停' : '未启动' },
                  { label: '最近操作人', value: loading ? '加载中' : error ? '加载失败' : systemState?.updated_by || '未记录' },
                  { label: '状态原因', value: loading ? '加载中' : error ? '加载失败' : systemState?.reason || '未提供' },
                ].map((item) => (
                  <Box
                    key={item.label}
                    sx={{
                      flex: { xs: '1 1 100%', md: '1 1 calc(33.33% - 10px)' },
                      p: 1.5,
                      borderRadius: 3,
                      border: '1px solid',
                      borderColor: 'divider',
                      bgcolor: 'background.default',
                    }}
                  >
                    <Typography variant="caption" color="text.secondary">
                      {item.label}
                    </Typography>
                    <Typography variant="body2" fontWeight={600} sx={{ mt: 0.5 }}>
                      {item.value}
                    </Typography>
                  </Box>
                ))}
              </Stack>

              <Divider />

              <Typography variant="subtitle1">控制动作</Typography>
              <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.25}>
                 <Button variant="contained" startIcon={<PlayArrowRoundedIcon />} disabled={loading || Boolean(error) || runtimeState === 'running' || runtimeState === 'paused' || actionLoading} onClick={() => void handleStart()}>
                   启动主链路
                 </Button>
                 <Button variant="outlined" color="warning" startIcon={<PauseCircleOutlineRoundedIcon />} disabled={loading || Boolean(error) || runtimeState !== 'running' || actionLoading} onClick={() => void handlePause()}>
                   提交暂停请求
                 </Button>
                 <Button variant="outlined" startIcon={<RestartAltRoundedIcon />} disabled={loading || Boolean(error) || runtimeState !== 'paused' || actionLoading} onClick={() => void handleResume()}>
                   恢复已暂停主链路
                 </Button>
              </Stack>

              <Stack spacing={1}>
                {[
                  '启动会按当前并发上限拉起主链路，仅对未启动状态生效。',
                  '暂停会提交协作暂停请求，不会强制中断已在执行中的任务。',
                  '恢复只对已暂停状态生效，会继续处理当前待处理队列。',
                ].map((item) => (
                  <Typography key={item} variant="body2" color="text.secondary">
                    {item}
                  </Typography>
                ))}
              </Stack>
            </Stack>
          </PageCard>

          <PageCard
            title="队列与并发"
            description="把并发上限、队列压力与完成进度放在同一视图中，便于判断是否需要启动、暂停或恢复。"
            action={<StatusChip status="pending" label="需手动刷新" />}
          >
            <Stack spacing={2}>
              <TextField
                label="启动并发上限"
                value={concurrencyLimit}
                onChange={(event) => setConcurrencyLimit(event.target.value)}
                disabled={runtimeState === 'running' || runtimeState === 'paused' || actionLoading}
                 helperText={`当前系统记录并发上限：${loading ? '加载中' : error ? '加载失败' : activeConcurrency ?? '未设置'}，只有点击“启动主链路”时才会提交。`}
               />

              <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.25} flexWrap="wrap" useFlexGap>
                {[
                   { label: '待处理队列', value: loading ? '加载中' : error ? '加载失败' : `${pendingCount} 条` },
                   { label: '处理中', value: loading ? '加载中' : error ? '加载失败' : `${processingCount} 条` },
                   { label: '已完成', value: loading ? '加载中' : error ? '加载失败' : `${completedCount} 条` },
                 ].map((item) => (
                  <Box
                    key={item.label}
                    sx={{
                      flex: { xs: '1 1 100%', md: '1 1 calc(33.33% - 10px)' },
                      p: 1.5,
                      borderRadius: 3,
                      border: '1px solid',
                      borderColor: 'divider',
                      bgcolor: 'background.default',
                    }}
                  >
                    <Typography variant="caption" color="text.secondary">
                      {item.label}
                    </Typography>
                    <Typography variant="body2" fontWeight={600} sx={{ mt: 0.5 }}>
                      {item.value}
                    </Typography>
                  </Box>
                ))}
              </Stack>

              <Divider />

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
            description="整理当前状态、失败积压与并发设置，便于值守处理。"
            action={<StatusChip status="active" label="持续观察" />}
          >
            <Stack spacing={1.5}>
              {[
                 loading
                   ? '系统状态与失败积压正在加载。'
                   : error
                     ? '系统状态暂时不可用，请刷新后重试。'
                     : `系统状态：${runtimeState}`,
                 loading
                   ? '失败积压正在加载。'
                   : error
                     ? '失败文章摘要暂时不可用。'
                     : `失败文章：${failedCount} 条${failedCount > 0 ? '，建议进入文章队列逐条处理。' : '，当前没有失败积压。'}`,
                 loading
                   ? '并发上限正在加载。'
                   : error
                     ? '并发上限暂时不可用。'
                     : `并发上限：${activeConcurrency ?? (Number(concurrencyLimit) || 1)}，由系统控制状态 metadata 决定。`,
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
