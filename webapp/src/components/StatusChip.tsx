import Chip from '@mui/material/Chip';
import { alpha, useTheme } from '@mui/material/styles';

export type StatusVariant = 'pending' | 'active' | 'disabled' | 'completed' | 'failed';

const statusLabelMap: Record<StatusVariant, string> = {
  pending: '待处理',
  active: '运行中',
  disabled: '已停用',
  completed: '已完成',
  failed: '失败',
};

type StatusChipProps = {
  status: StatusVariant;
  label?: string;
};

export default function StatusChip({ status, label }: StatusChipProps) {
  const theme = useTheme();
  const color = theme.palette.status[status];

  return (
    <Chip
      label={label ?? statusLabelMap[status]}
      size="small"
      sx={{
        color,
        bgcolor: alpha(color, 0.12),
        border: '1px solid',
        borderColor: alpha(color, 0.22),
        '& .MuiChip-label': {
          px: 1.25,
        },
      }}
    />
  );
}
