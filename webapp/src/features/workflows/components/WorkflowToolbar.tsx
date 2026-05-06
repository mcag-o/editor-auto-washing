import AddRoundedIcon from '@mui/icons-material/AddRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import RadioButtonCheckedRoundedIcon from '@mui/icons-material/RadioButtonCheckedRounded';
import Button from '@mui/material/Button';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import StatusChip from '../../../components/StatusChip';

type WorkflowToolbarProps = {
  canDeleteNode: boolean;
  hasSelection: boolean;
  nodeCount: number;
  edgeCount: number;
  onAddNode: () => void;
  onDeleteNode: () => void;
  onSelectEntryNode: () => void;
};

export default function WorkflowToolbar({
  canDeleteNode,
  hasSelection,
  nodeCount,
  edgeCount,
  onAddNode,
  onDeleteNode,
  onSelectEntryNode,
}: WorkflowToolbarProps) {
  return (
    <Stack
      direction={{ xs: 'column', lg: 'row' }}
      spacing={1.5}
      justifyContent="space-between"
      alignItems={{ xs: 'stretch', lg: 'center' }}
      sx={{ p: 1.5, borderRadius: 4, border: '1px solid', borderColor: 'divider', bgcolor: 'background.paper' }}
    >
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} alignItems={{ xs: 'flex-start', sm: 'center' }}>
        <StatusChip status="active" label="工作流编辑" />
        <Typography variant="body2" color="text.secondary">
          当前画布共 {nodeCount} 个节点、{edgeCount} 条连线，保存后会整体覆盖后端中的当前工作流定义。
        </Typography>
      </Stack>

      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1}>
        <Button variant="contained" startIcon={<AddRoundedIcon />} onClick={onAddNode}>
          新增节点
        </Button>
        <Button variant="outlined" startIcon={<RadioButtonCheckedRoundedIcon />} disabled={!hasSelection} onClick={onSelectEntryNode}>
          设为入口节点
        </Button>
        <Button variant="outlined" color="error" startIcon={<DeleteOutlineRoundedIcon />} disabled={!canDeleteNode} onClick={onDeleteNode}>
          删除节点
        </Button>
      </Stack>
    </Stack>
  );
}
