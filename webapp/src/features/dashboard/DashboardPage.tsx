import AutoAwesomeRoundedIcon from '@mui/icons-material/AutoAwesomeRounded';
import ChecklistRoundedIcon from '@mui/icons-material/ChecklistRounded';
import CloudUploadRoundedIcon from '@mui/icons-material/CloudUploadRounded';
import DescriptionRoundedIcon from '@mui/icons-material/DescriptionRounded';
import PlayCircleRoundedIcon from '@mui/icons-material/PlayCircleRounded';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import LinearProgress from '@mui/material/LinearProgress';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import MetricCards from '../../components/MetricCards';
import PageCard from '../../components/PageCard';
import PageToolbar from '../../components/PageToolbar';
import StatusChip from '../../components/StatusChip';
import type { AppPage } from '../../layout/AppShell';

type DashboardPageProps = {
  onNavigate?: (page: AppPage) => void;
};

const metrics = [
  { key: 'pending', label: '待处理文章', value: '18', hint: '等待进入改写流水线', icon: <DescriptionRoundedIcon fontSize="small" /> },
  { key: 'running', label: '处理中任务', value: '6', hint: '含改写、草稿生成与渲染步骤', icon: <PlayCircleRoundedIcon fontSize="small" /> },
  { key: 'completed', label: '今日已处理', value: '42', hint: '本页为本地摘要壳层，数据后续接 API', icon: <ChecklistRoundedIcon fontSize="small" /> },
  { key: 'templates', label: '启用模板', value: '4', hint: '模板配置页后续补齐', icon: <AutoAwesomeRoundedIcon fontSize="small" /> },
];

const pipelines = [
  { key: 'rewrite', label: '标准改写链路', progress: 78, detail: '导入文章后自动改写并生成草稿' },
  { key: 'review', label: '人工复核队列', progress: 32, detail: '处理异常文章与待确认内容' },
  { key: 'render', label: '渲染输出准备', progress: 61, detail: '生成渲染稿并等待后续发布节点' },
];

const alerts = [
  { key: 'upload', title: '导入入口已切换至浏览器', description: '当前仅保留文件上传与文本粘贴，不展示 URL 导入入口。', status: 'active' as const },
  { key: 'queue', title: '文章队列已拆分状态视图', description: '文章列表支持按未处理、处理中、已处理快速过滤。', status: 'pending' as const },
  { key: 'api', title: '业务 API 暂未接入', description: '本任务仅提供页面结构与本地交互壳层，真实请求延后到 Task 7。', status: 'disabled' as const },
];

export default function DashboardPage({ onNavigate }: DashboardPageProps) {
  return (
    <Stack spacing={3}>
      <PageToolbar
        title="控制台总览"
        description="聚合查看当前处理节奏、待办压力与页面入口，保持运营视角的中文工作台体验。"
        leading={<StatusChip status="active" label="本地摘要视图" />}
        actions={
          <>
            <Button variant="outlined" onClick={() => onNavigate?.('articles')}>
              查看文章队列
            </Button>
            <Button variant="contained" startIcon={<CloudUploadRoundedIcon />} onClick={() => onNavigate?.('intake')}>
              导入文章
            </Button>
          </>
        }
      />

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
          description="用于承接后续真实统计与节点详情，本任务先提供结构化摘要卡片。"
          action={<StatusChip status="completed" label="摘要就绪" />}
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
          description="保留常驻提醒区，后续可替换为真实告警、失败任务与系统通知。"
          action={<StatusChip status="pending" label="需持续关注" />}
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
