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
      data-status={status}
      label={label ?? statusLabelMap[status]}
      size="small"
      variant="outlined"
      sx={{
        color,
        bgcolor: alpha(color, 0.1),
        border: '1px solid',
        borderColor: alpha(color, 0.24),
        height: 28,
        '& .MuiChip-label': {
          px: 1.125,
          lineHeight: 1.1,
        },
      }}
    />
  );
}
