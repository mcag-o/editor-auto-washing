import AutorenewRoundedIcon from '@mui/icons-material/AutorenewRounded';
import CloudDoneRoundedIcon from '@mui/icons-material/CloudDoneRounded';
import FactCheckRoundedIcon from '@mui/icons-material/FactCheckRounded';
import PlayCircleRoundedIcon from '@mui/icons-material/PlayCircleRounded';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { useMemo, useState } from 'react';
import AppShell from '../layout/AppShell';
import ConfirmDialog from '../components/ConfirmDialog';
import MetricCards from '../components/MetricCards';
import PageCard from '../components/PageCard';
import PageToolbar from '../components/PageToolbar';
import StatusChip from '../components/StatusChip';
import { useDashboardSummaryQuery, useHealthQuery } from '../lib/api/hooks';

export default function AppRoutes() {
  const [confirmOpen, setConfirmOpen] = useState(false);
  const healthQuery = useHealthQuery();
  const summaryQuery = useDashboardSummaryQuery();

  const metrics = useMemo(() => {
    if (summaryQuery.data?.metrics.length) {
      return summaryQuery.data.metrics.map((metric) => ({
        key: metric.key,
        label: metric.label,
        value: String(metric.value),
      }));
    }

    return [
      { key: 'queue', label: '待处理文章', value: '--', hint: '等待 API 接入真实汇总值' },
      { key: 'running', label: '运行中的流程', value: '--', hint: '共享壳层预留占位' },
      { key: 'drafts', label: '今日草稿产出', value: '--', hint: '后续页面复用同一卡片样式' },
      { key: 'errors', label: '最近失败任务', value: '--', hint: '统一错误呈现入口' },
    ];
  }, [summaryQuery.data]);

  return (
    <AppShell>
      <Stack spacing={3}>
        <PageToolbar
          title="控制台总览"
          description="本任务仅搭建共享外壳、主题与 API 基础设施，后续功能页将在此结构中接入。"
          leading={<StatusChip status="active" label="共享基础设施" />}
          actions={
            <>
              <Button variant="outlined" onClick={() => setConfirmOpen(true)}>
                打开确认框
              </Button>
              <Button variant="contained">预留主要操作</Button>
            </>
          }
          filters={
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} alignItems="center">
              <StatusChip status="pending" />
              <StatusChip status="completed" />
              <StatusChip status="failed" />
              <Typography variant="body2" color="text.secondary">
                过滤与批量操作区预留给文章列表、配置列表和审计列表复用。
              </Typography>
            </Stack>
          }
        />

        {healthQuery.error ? (
          <Alert severity="warning">健康检查接口尚未接通：{healthQuery.error.message}</Alert>
        ) : null}
        {summaryQuery.error ? (
          <Alert severity="info">汇总接口尚未接通：{summaryQuery.error.message}</Alert>
        ) : null}

        <MetricCards items={metrics} />

        <Box
          sx={{
            display: 'grid',
            gap: 3,
            gridTemplateColumns: { xs: '1fr', xl: 'minmax(0, 1.55fr) minmax(320px, 0.95fr)' },
          }}
        >
          <PageCard
            title="应用外壳"
            description="固定顶部栏、左侧导航、内容容器与统一页面节奏已建立。"
            action={<StatusChip status="completed" />}
          >
            <Stack spacing={1.5}>
              <Typography variant="body2" color="text.secondary">
                新页面只需要专注业务内容，不需要重复处理导航、标题区、间距或基础卡片样式。
              </Typography>
              <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.25}>
                <StatusChip status="active" label="顶部应用栏" />
                <StatusChip status="active" label="侧边导航" />
                <StatusChip status="active" label="内容容器" />
                <StatusChip status="disabled" label="功能页待接入" />
              </Stack>
            </Stack>
          </PageCard>

          <PageCard
            title="API 基础层"
            description="`/api/*` 的 typed client、错误类型与 hooks 已就位。"
            action={<StatusChip status={healthQuery.data ? 'completed' : 'pending'} />}
          >
            <Stack spacing={1.5}>
              <Stack direction="row" spacing={1.25} alignItems="center">
                <CloudDoneRoundedIcon color={healthQuery.data ? 'success' : 'disabled'} />
                <Typography variant="body2" color="text.secondary">
                  健康检查状态：{healthQuery.loading ? '检查中' : healthQuery.data?.status ?? '未连接'}
                </Typography>
              </Stack>
              <Stack direction="row" spacing={1.25} alignItems="center">
                <AutorenewRoundedIcon color={summaryQuery.data ? 'success' : 'disabled'} />
                <Typography variant="body2" color="text.secondary">
                  汇总查询：{summaryQuery.loading ? '加载中' : summaryQuery.data ? '已返回数据' : '等待接口'}
                </Typography>
              </Stack>
            </Stack>
          </PageCard>

          <PageCard
            title="共享组件清单"
            description="状态标签、页面卡片、指标卡片、页面工具栏与确认对话框均可复用。"
            action={<StatusChip status="completed" />}
          >
            <Stack spacing={1.25}>
              <Stack direction="row" spacing={1.25} alignItems="center">
                <FactCheckRoundedIcon color="primary" fontSize="small" />
                <Typography variant="body2" color="text.secondary">
                  适用于文章、配置、审计等后续页面，避免重复实现基础视觉语义。
                </Typography>
              </Stack>
              <Stack direction="row" spacing={1.25} alignItems="center">
                <PlayCircleRoundedIcon color="primary" fontSize="small" />
                <Typography variant="body2" color="text.secondary">
                  当前只展示共享基础设施，不引入具体业务流程或页面逻辑。
                </Typography>
              </Stack>
            </Stack>
          </PageCard>
        </Box>
      </Stack>

      <ConfirmDialog
        open={confirmOpen}
        title="共享确认框示例"
        description="后续删除、停用、重跑等操作可以复用这一对话框，不需要各页面重复维护。"
        confirmText="我知道了"
        onClose={() => setConfirmOpen(false)}
        onConfirm={() => setConfirmOpen(false)}
      />
    </AppShell>
  );
}
