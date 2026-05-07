import AddRoundedIcon from '@mui/icons-material/AddRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import FitScreenRoundedIcon from '@mui/icons-material/FitScreenRounded';
import HubRoundedIcon from '@mui/icons-material/HubRounded';
import RadioButtonCheckedRoundedIcon from '@mui/icons-material/RadioButtonCheckedRounded';
import TimelineRoundedIcon from '@mui/icons-material/TimelineRounded';
import Button from '@mui/material/Button';
import Paper from '@mui/material/Paper';
import Chip from '@mui/material/Chip';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { alpha } from '@mui/material/styles';
import StatusChip from '../../../components/StatusChip';

type WorkflowToolbarProps = {
  canDeleteNode: boolean;
  canFitView: boolean;
  canSetEntryNode: boolean;
  focusDescription: string;
  focusLabel: string;
  entryNodeLabel: string | null;
  selectedEdgeLabel: string | null;
  selectedNodeLabel: string | null;
  selectionKind: 'node' | 'edge' | 'idle';
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
  focusDescription,
  focusLabel,
  entryNodeLabel,
  selectedEdgeLabel,
  selectedNodeLabel,
  selectionKind,
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
      <Stack spacing={1.5} flex={1} minWidth={0}>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} alignItems={{ xs: 'flex-start', sm: 'center' }}>
          <StatusChip status="active" label="工作流编辑" />
          <Typography variant="body2" color="text.secondary">
            画布为主操作区，右侧面板跟随当前选择同步更新。
          </Typography>
        </Stack>

        <Stack direction={{ xs: 'column', lg: 'row' }} spacing={1.25} alignItems={{ xs: 'stretch', lg: 'center' }}>
          <Paper
            variant="outlined"
            sx={{
              px: 1.5,
              py: 1.25,
              borderRadius: 3,
              minWidth: { xs: '100%', lg: 280 },
              borderColor: selectionKind === 'edge' ? alpha('#5b3df5', 0.24) : alpha('#0f62fe', 0.22),
              background:
                selectionKind === 'edge'
                  ? 'linear-gradient(135deg, rgba(91, 61, 245, 0.10), rgba(255, 255, 255, 0.98))'
                  : 'linear-gradient(135deg, rgba(15, 98, 254, 0.10), rgba(255, 255, 255, 0.98))',
            }}
          >
            <Stack direction="row" spacing={1.25} alignItems="flex-start">
              {selectionKind === 'edge' ? <TimelineRoundedIcon color="secondary" /> : <HubRoundedIcon color="primary" />}
              <Stack spacing={0.35} minWidth={0}>
                <Typography variant="overline" color="text.secondary">
                  {focusLabel}
                </Typography>
                <Typography variant="subtitle2">{focusDescription}</Typography>
              </Stack>
            </Stack>
          </Paper>

          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} flexWrap="wrap" useFlexGap>
            <Chip size="small" color="default" variant="outlined" label={`节点 ${nodeCount}`} />
            <Chip size="small" color="default" variant="outlined" label={`连线 ${edgeCount}`} />
            <Chip size="small" color="info" variant="outlined" label={`入口节点: ${entryNodeLabel ?? '未设置'}`} />
          </Stack>
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
            color={selectionKind === 'edge' ? 'secondary' : selectionKind === 'node' ? 'primary' : 'default'}
            variant={selectionKind === 'idle' ? 'outlined' : 'filled'}
            label={selectionKind === 'edge' ? '焦点对象: 连线' : selectionKind === 'node' ? '焦点对象: 节点' : '焦点对象: 未选择'}
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
