import ContentCopyRoundedIcon from '@mui/icons-material/ContentCopyRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import RadioButtonCheckedRoundedIcon from '@mui/icons-material/RadioButtonCheckedRounded';
import Button from '@mui/material/Button';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { alpha } from '@mui/material/styles';

type WorkflowCanvasQuickActionsProps = {
  canDelete: boolean;
  canDuplicate: boolean;
  canSetEntryNode: boolean;
  focusLabel: string;
  onDelete: () => void;
  onDuplicate: () => void;
  onSetEntryNode: () => void;
  selectionKind: 'node' | 'edge' | 'idle';
};

export default function WorkflowCanvasQuickActions({
  canDelete,
  canDuplicate,
  canSetEntryNode,
  focusLabel,
  onDelete,
  onDuplicate,
  onSetEntryNode,
  selectionKind,
}: WorkflowCanvasQuickActionsProps) {
  if (selectionKind === 'idle') {
    return null;
  }

  return (
    <Paper
      data-testid="workflow-canvas-quick-actions"
      elevation={0}
      sx={{
        p: 1,
        borderRadius: 4,
        bgcolor: alpha('#ffffff', 0.94),
        border: `1px solid ${alpha(selectionKind === 'edge' ? '#5b3df5' : '#0f62fe', 0.16)}`,
        boxShadow: '0 16px 36px rgba(20, 32, 51, 0.12)',
        backdropFilter: 'blur(12px)',
      }}
    >
      <Stack spacing={1}>
        <Typography variant="caption" color="text.secondary">
          画布快捷操作: {focusLabel}
        </Typography>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1}>
          {selectionKind === 'node' ? (
            <>
              <Button
                size="small"
                variant="contained"
                startIcon={<RadioButtonCheckedRoundedIcon />}
                disabled={!canSetEntryNode}
                onClick={onSetEntryNode}
              >
                设为入口节点
              </Button>
              <Button
                size="small"
                variant="outlined"
                startIcon={<ContentCopyRoundedIcon />}
                disabled={!canDuplicate}
                onClick={onDuplicate}
              >
                复制节点
              </Button>
            </>
          ) : null}
          <Button
            size="small"
            variant="outlined"
            color="error"
            startIcon={<DeleteOutlineRoundedIcon />}
            disabled={!canDelete}
            onClick={onDelete}
          >
            {selectionKind === 'edge' ? '删除连线' : '删除节点'}
          </Button>
        </Stack>
      </Stack>
    </Paper>
  );
}
