import { useMemo, useState } from 'react';
import PauseCircleOutlineRoundedIcon from '@mui/icons-material/PauseCircleOutlineRounded';
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded';
import RestartAltRoundedIcon from '@mui/icons-material/RestartAltRounded';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import LinearProgress from '@mui/material/LinearProgress';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import MetricCards from '../../components/MetricCards';
import PageCard from '../../components/PageCard';
import PageToolbar from '../../components/PageToolbar';
import StatusChip from '../../components/StatusChip';
import type { AppPage } from '../../layout/AppShell';

type RuntimeState = 'idle' | 'running' | 'paused';

type ControlPageProps = {
  onNavigate?: (page: AppPage) => void;
};

const stages = [
  { key: 'intake', label: '导入接收', progress: 100, detail: '浏览器上传与粘贴入口已就绪，等待统一启动。' },
  { key: 'rewrite', label: '自动改写', progress: 72, detail: '后续接入真实编排状态后展示批次、模板与失败明细。' },
  { key: 'render', label: '草稿渲染', progress: 48, detail: '当前仅保留结果阶段位置，不触发真实任务。' },
];

const pendingItems = [
  '当前为本地交互壳层，启动/暂停/恢复只切换页面状态。',
  '后续可在此接入任务调度器、模板选择与系统级告警。',
  '页面已预留运行状态、摘要指标与链路观察区。',
];

export default function ControlPage({ onNavigate }: ControlPageProps) {
  const [runtimeState, setRuntimeState] = useState<RuntimeState>('idle');
  const [lastAction, setLastAction] = useState('尚未执行控制动作');

  const stateSummary = useMemo(() => {
    if (runtimeState === 'running') {
      return {
        chipStatus: 'active' as const,
        chipLabel: '主链路运行中',
        headline: '自动改写主链路已进入运行态',
        description: '当前页面仅模拟运行控制，后续会替换为真实任务编排状态。',
      };
    }

    if (runtimeState === 'paused') {
      return {
        chipStatus: 'pending' as const,
        chipLabel: '主链路已暂停',
        headline: '系统处于暂停观察态',
        description: '页面保留恢复入口与链路摘要，用于后续接入真实暂停逻辑。',
      };
    }

    return {
      chipStatus: 'disabled' as const,
      chipLabel: '主链路未启动',
      headline: '系统等待启动',
      description: '本任务仅实现控制页结构与按钮占位，不触发后端操作。',
    };
  }, [runtimeState]);

  const metrics = [
    { key: 'runtime', label: '当前状态', value: runtimeState === 'running' ? '运行中' : runtimeState === 'paused' ? '已暂停' : '待启动', hint: '按钮状态为本地模拟', icon: <PlayArrowRoundedIcon fontSize="small" /> },
    { key: 'queue', label: '待处理任务', value: '18', hint: '沿用当前前端占位统计', icon: <RestartAltRoundedIcon fontSize="small" /> },
    { key: 'template', label: '活动模板', value: '4', hint: '后续接入真实模板绑定', icon: <Chip size="small" label="模板" /> },
    { key: 'alerts', label: '观察提醒', value: runtimeState === 'paused' ? '2' : '1', hint: '仅展示本地提醒摘要', icon: <PauseCircleOutlineRoundedIcon fontSize="small" /> },
  ];

  return (
    <Stack spacing={3}>
      <PageToolbar
        title="流程控制"
        description="面向运营值守的工作流控制页，先提供系统状态、启动/暂停/恢复按钮与链路摘要壳层。"
        leading={<StatusChip status={stateSummary.chipStatus} label={stateSummary.chipLabel} />}
        actions={
          <>
            <Button variant="outlined" onClick={() => onNavigate?.('dashboard')}>
              返回总览
            </Button>
            <Button variant="text" onClick={() => onNavigate?.('settings')}>
              查看配置
            </Button>
          </>
        }
        filters={
          <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.25} alignItems={{ xs: 'flex-start', md: 'center' }}>
            <StatusChip status="completed" label="页面结构已就绪" />
            <StatusChip status="disabled" label="未接入真实编排 API" />
            <Typography variant="body2" color="text.secondary">
              最近动作：{lastAction}
            </Typography>
          </Stack>
        }
      />

      <MetricCards items={metrics} />

      <Stack direction={{ xs: 'column', xl: 'row' }} spacing={3} alignItems="stretch">
        <Stack spacing={3} flex={1.2} minWidth={0}>
          <PageCard
            title="系统状态"
            description={stateSummary.description}
            action={<StatusChip status={stateSummary.chipStatus} label={stateSummary.chipLabel} />}
          >
            <Stack spacing={2}>
              <Typography variant="h4">{stateSummary.headline}</Typography>
              <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.25}>
                <Button
                  variant="contained"
                  startIcon={<PlayArrowRoundedIcon />}
                  disabled={runtimeState === 'running'}
                  onClick={() => {
                    setRuntimeState('running');
                    setLastAction('已执行“启动流程”本地占位动作');
                  }}
                >
                  启动流程
                </Button>
                <Button
                  variant="outlined"
                  color="warning"
                  startIcon={<PauseCircleOutlineRoundedIcon />}
                  disabled={runtimeState !== 'running'}
                  onClick={() => {
                    setRuntimeState('paused');
                    setLastAction('已执行“暂停流程”本地占位动作');
                  }}
                >
                  暂停流程
                </Button>
                <Button
                  variant="outlined"
                  startIcon={<RestartAltRoundedIcon />}
                  disabled={runtimeState !== 'paused'}
                  onClick={() => {
                    setRuntimeState('running');
                    setLastAction('已执行“恢复流程”本地占位动作');
                  }}
                >
                  恢复流程
                </Button>
              </Stack>
              <Typography variant="body2" color="text.secondary">
                后续接入后，这里将承接真实系统状态、操作反馈与权限控制。
              </Typography>
            </Stack>
          </PageCard>

          <PageCard
            title="主链路观察"
            description="通过本地进度条展示导入、改写、草稿渲染三个关键阶段，方便后续替换为真实运行指标。"
            action={<StatusChip status="pending" label="本地摘要" />}
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
            description="保留后续系统告警、失败批次与人工干预入口的位置。"
            action={<StatusChip status="active" label="持续观察" />}
          >
            <Stack spacing={1.5}>
              {pendingItems.map((item) => (
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
