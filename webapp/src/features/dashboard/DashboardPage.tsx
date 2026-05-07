import { useEffect, useMemo, useState } from 'react';
import AutoAwesomeRoundedIcon from '@mui/icons-material/AutoAwesomeRounded';
import ChecklistRoundedIcon from '@mui/icons-material/ChecklistRounded';
import CloudUploadRoundedIcon from '@mui/icons-material/CloudUploadRounded';
import DescriptionRoundedIcon from '@mui/icons-material/DescriptionRounded';
import PlayCircleRoundedIcon from '@mui/icons-material/PlayCircleRounded';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import CircularProgress from '@mui/material/CircularProgress';
import LinearProgress from '@mui/material/LinearProgress';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import MetricCards from '../../components/MetricCards';
import PageCard from '../../components/PageCard';
import PageToolbar from '../../components/PageToolbar';
import StatusChip from '../../components/StatusChip';
import { ApiError, getSystemStatus, listArticles, listAudit, listTemplates } from '../../lib/api/client';
import type { AuditLog, SourceDocument, SystemControlState, TemplateDefinition } from '../../lib/api/types';
import type { AppPage } from '../../layout/AppShell';

type DashboardPageProps = {
  onNavigate?: (page: AppPage) => void;
};

function countByStatus(articles: SourceDocument[], ...statuses: string[]) {
  return articles.filter((item) => statuses.includes(item.status)).length;
}

export default function DashboardPage({ onNavigate }: DashboardPageProps) {
  const [systemState, setSystemState] = useState<SystemControlState | null>(null);
  const [articles, setArticles] = useState<SourceDocument[]>([]);
  const [templates, setTemplates] = useState<TemplateDefinition[]>([]);
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();

    Promise.all([
      getSystemStatus({ signal: controller.signal }),
      listArticles({ signal: controller.signal }),
      listTemplates({ signal: controller.signal }),
      listAudit({ signal: controller.signal }),
    ])
      .then(([state, articleItems, templateItems, auditItems]) => {
        if (controller.signal.aborted) {
          return;
        }
        setSystemState(state);
        setArticles(articleItems);
        setTemplates(templateItems);
        setAuditLogs(auditItems);
      })
      .catch((apiError: unknown) => {
        if (controller.signal.aborted) {
          return;
        }
        setError(apiError instanceof ApiError ? apiError.message : '总览数据加载失败');
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      });

    return () => controller.abort();
  }, []);

  const pendingCount = countByStatus(articles, 'pending');
  const runningCount = countByStatus(articles, 'processing', 'claimed');
  const completedCount = countByStatus(articles, 'completed');
  const failedCount = countByStatus(articles, 'failed');
  const enabledTemplates = templates.filter((item) => item.enabled).length;
  const runtimeLabel = systemState?.state === 'running' ? '运行中' : systemState?.state === 'paused' ? '已暂停' : '待启动';

  const metrics = [
    { key: 'pending', label: '待处理文章', value: String(pendingCount), hint: '来源于文章队列中的 pending 状态。', icon: <DescriptionRoundedIcon fontSize="small" /> },
    { key: 'running', label: '处理中任务', value: String(runningCount), hint: `当前系统状态：${runtimeLabel}。`, icon: <PlayCircleRoundedIcon fontSize="small" /> },
    { key: 'completed', label: '已完成文章', value: String(completedCount), hint: '来源于已完成状态文章数量。', icon: <ChecklistRoundedIcon fontSize="small" /> },
    { key: 'templates', label: '启用模板', value: String(enabledTemplates), hint: `${enabledTemplates} 个已启用`, icon: <AutoAwesomeRoundedIcon fontSize="small" /> },
  ];

  const pipelines = useMemo(() => {
    const total = Math.max(articles.length, 1);
    return [
      { key: 'rewrite', label: '标准改写链路', progress: articles.length === 0 ? 0 : Math.round(((runningCount + completedCount) / total) * 100), detail: `运行中 ${runningCount} 条，已完成 ${completedCount} 条。` },
      { key: 'review', label: '人工复核队列', progress: articles.length === 0 ? 0 : Math.round((failedCount / total) * 100), detail: `失败文章 ${failedCount} 条，建议优先进入文章队列处理。` },
      { key: 'render', label: '渲染输出准备', progress: articles.length === 0 ? 0 : Math.round((completedCount / total) * 100), detail: `已完成 ${completedCount} 条，可进入后续草稿/渲染观察。` },
    ];
  }, [articles.length, completedCount, failedCount, runningCount]);

  const alerts = useMemo(() => {
    const latestAudit = auditLogs[0];
    return [
      {
        key: 'runtime',
        title: `系统当前状态：${runtimeLabel}`,
        description: systemState?.updated_at ? `最近一次状态更新时间：${new Date(systemState.updated_at).toLocaleString('zh-CN', { hour12: false })}。` : '系统状态尚未返回更新时间。',
        status: systemState?.state === 'running' ? ('active' as const) : systemState?.state === 'paused' ? ('pending' as const) : ('disabled' as const),
      },
      {
        key: 'queue',
        title: `失败文章 ${failedCount} 条，建议优先进入文章队列处理。`,
        description: `当前待处理 ${pendingCount} 条，处理中 ${runningCount} 条。`,
        status: failedCount > 0 ? ('failed' as const) : ('completed' as const),
      },
      {
        key: 'audit',
        title: latestAudit ? `最近审计：${latestAudit.action}` : '暂无审计记录',
        description: latestAudit?.message || '当前审计 API 未返回最新记录。',
        status: latestAudit?.result === 'failure' ? ('failed' as const) : latestAudit ? ('completed' as const) : ('disabled' as const),
      },
    ];
  }, [auditLogs, failedCount, pendingCount, runningCount, runtimeLabel, systemState?.state, systemState?.updated_at]);

  return (
    <Stack spacing={3}>
      <PageToolbar
        title="控制台总览"
        description="汇总系统状态、文章队列、模板与审计摘要。"
        leading={<StatusChip status="active" label="运营总览" />}
        actions={
          <>
            <Button color="inherit" variant="text" onClick={() => onNavigate?.('articles')}>
              查看文章队列
            </Button>
            <Button variant="contained" startIcon={<CloudUploadRoundedIcon />} onClick={() => onNavigate?.('intake')}>
              导入文章
            </Button>
          </>
        }
      />

      {error ? <Alert severity="error">{error}</Alert> : null}

      <MetricCards items={metrics} />

      <Box
        sx={{
          display: 'grid',
          gap: 3,
          gridTemplateColumns: { xs: '1fr', xl: 'minmax(0, 1.45fr) minmax(320px, 0.95fr)' },
        }}
      >
        <PageCard
          title="处理链路概览"
          description="基于当前已加载数据汇总导入、改写、复核与输出进度。"
          action={loading ? <CircularProgress size={18} /> : <StatusChip status="completed" label="摘要就绪" />}
        >
          <Stack spacing={2}>
            {pipelines.map((pipeline) => (
              <Stack key={pipeline.key} spacing={1}>
                <Stack direction="row" justifyContent="space-between" alignItems="center">
                  <Typography variant="subtitle1">{pipeline.label}</Typography>
                  <Typography variant="body2" color="text.secondary">
                    {pipeline.progress}%
                  </Typography>
                </Stack>
                <LinearProgress variant="determinate" value={pipeline.progress} sx={{ height: 10, borderRadius: 999 }} />
                <Typography variant="body2" color="text.secondary">
                  {pipeline.detail}
                </Typography>
              </Stack>
            ))}
          </Stack>
        </PageCard>

        <PageCard
          title="运营提醒"
          description="基于最近一次加载结果汇总系统状态、失败积压与审计记录。"
          action={<StatusChip status={failedCount > 0 ? 'failed' : 'pending'} label="需持续关注" />}
        >
          <Stack spacing={1.5}>
            {alerts.map((item) => (
              <Stack
                key={item.key}
                spacing={0.75}
                sx={{ p: 1.75, borderRadius: 3, border: '1px solid', borderColor: 'divider', bgcolor: 'background.default' }}
              >
                <Stack direction="row" spacing={1} alignItems="center" justifyContent="space-between">
                  <Typography variant="subtitle1">{item.title}</Typography>
                  <StatusChip status={item.status} />
                </Stack>
                <Typography variant="body2" color="text.secondary">
                  {item.description}
                </Typography>
              </Stack>
            ))}
          </Stack>
        </PageCard>
      </Box>
    </Stack>
  );
}
