import ConstructionRoundedIcon from '@mui/icons-material/ConstructionRounded';
import LaunchRoundedIcon from '@mui/icons-material/LaunchRounded';
import Button from '@mui/material/Button';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import PageCard from '../../components/PageCard';
import PageToolbar from '../../components/PageToolbar';
import StatusChip from '../../components/StatusChip';
import type { AppPage } from '../../layout/AppShell';

type PlaceholderPageProps = {
  title: string;
  moduleName: string;
  description: string;
  onNavigate?: (page: AppPage) => void;
};

export default function PlaceholderPage({ title, moduleName, description, onNavigate }: PlaceholderPageProps) {
  return (
    <Stack spacing={3}>
      <PageToolbar
        title={title}
        description={description}
        leading={<StatusChip status="disabled" label="本里程碑未接入" />}
        actions={
          <>
            <Button variant="outlined" onClick={() => onNavigate?.('overview')}>
              返回总览
            </Button>
            <Button variant="contained" startIcon={<LaunchRoundedIcon />} onClick={() => onNavigate?.('articles')}>
              查看现有页面
            </Button>
          </>
        }
      />

      <PageCard
        title={`${moduleName}模块占位`}
        description="该入口已经纳入控制面板信息架构，但真实页面与业务能力会在后续任务中逐步替换。"
        action={<StatusChip status="pending" label="等待后续任务" />}
      >
        <Stack spacing={2} alignItems="flex-start">
          <ConstructionRoundedIcon color="primary" sx={{ fontSize: 32 }} />
          <Typography variant="body1" color="text.secondary">
            当前点击导航后会进入稳定的占位页，而不是停留在原页面，确保外壳体验一致且导航结果可预期。
          </Typography>
          <Typography variant="body2" color="text.secondary">
            后续实现该模块时，只需要将本占位页替换为真实功能页，无需调整导航结构。
          </Typography>
        </Stack>
      </PageCard>
    </Stack>
  );
}
