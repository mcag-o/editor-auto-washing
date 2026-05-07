import AddRoundedIcon from '@mui/icons-material/AddRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import FitScreenRoundedIcon from '@mui/icons-material/FitScreenRounded';
import RadioButtonCheckedRoundedIcon from '@mui/icons-material/RadioButtonCheckedRounded';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { alpha } from '@mui/material/styles';
import StatusChip from '../../../components/StatusChip';

type WorkflowToolbarProps = {
  canDeleteNode: boolean;
  canFitView: boolean;
  canSetEntryNode: boolean;
  entryNodeLabel: string | null;
  selectedEdgeLabel: string | null;
  selectedNodeLabel: string | null;
  hasSelection: boolean;
  nodeCount: number;
  edgeCount: number;
  onAddNode: () => void;
  onDeleteNode: () => void;
  onFitView: () => void;
  onSelectEntryNode: () => void;
};

export default function WorkflowToolbar({
  canDeleteNode,
  canFitView,
  canSetEntryNode,
  entryNodeLabel,
  selectedEdgeLabel,
  selectedNodeLabel,
  hasSelection,
  nodeCount,
  edgeCount,
  onAddNode,
  onDeleteNode,
  onFitView,
  onSelectEntryNode,
}: WorkflowToolbarProps) {
  return (
    <Stack
      role="toolbar"
      aria-label="工作流画布工具栏"
      direction={{ xs: 'column', lg: 'row' }}
      spacing={1.5}
      justifyContent="space-between"
      alignItems={{ xs: 'stretch', lg: 'center' }}
      sx={{
        p: 1.75,
        borderRadius: 4,
        border: '1px solid',
        borderColor: alpha('#15304f', 0.12),
        bgcolor: '#ffffff',
        boxShadow: '0 16px 36px rgba(20, 32, 51, 0.08)',
      }}
    >
      <Stack spacing={1.25}>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} alignItems={{ xs: 'flex-start', sm: 'center' }}>
          <StatusChip status="active" label="工作流编辑" />
          <Typography variant="body2" color="text.secondary">
            当前画布共 {nodeCount} 个节点、{edgeCount} 条连线，保存后会整体提交当前工作流定义。
          </Typography>
        </Stack>

        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} flexWrap="wrap" useFlexGap>
          <Chip
            size="small"
            color={selectedNodeLabel ? 'primary' : 'default'}
            variant={selectedNodeLabel ? 'filled' : 'outlined'}
            label={`已选择节点: ${selectedNodeLabel ?? '当前未选择节点'}`}
          />
          <Chip
            size="small"
            color={selectedEdgeLabel ? 'secondary' : 'default'}
            variant={selectedEdgeLabel ? 'filled' : 'outlined'}
            label={`已选择连线: ${selectedEdgeLabel ?? '当前未选择连线'}`}
          />
          <Chip
            size="small"
            color="info"
            variant="outlined"
            label={`入口节点: ${entryNodeLabel ?? '未设置'}`}
          />
        </Stack>
      </Stack>

      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1}>
        <Button variant="contained" startIcon={<AddRoundedIcon />} onClick={onAddNode}>
          新增节点
        </Button>
        <Button variant="outlined" startIcon={<FitScreenRoundedIcon />} disabled={!canFitView} onClick={onFitView}>
          适配视图
        </Button>
        <Button variant="outlined" startIcon={<RadioButtonCheckedRoundedIcon />} disabled={!canSetEntryNode} onClick={onSelectEntryNode}>
          设为入口节点
        </Button>
        <Button variant="outlined" color="error" startIcon={<DeleteOutlineRoundedIcon />} disabled={!canDeleteNode} onClick={onDeleteNode}>
          删除节点
        </Button>
      </Stack>
    </Stack>
  );
}
