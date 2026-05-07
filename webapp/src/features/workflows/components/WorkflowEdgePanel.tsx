import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import Accordion from '@mui/material/Accordion';
import AccordionDetails from '@mui/material/AccordionDetails';
import AccordionSummary from '@mui/material/AccordionSummary';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import ExpandMoreRoundedIcon from '@mui/icons-material/ExpandMoreRounded';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import Divider from '@mui/material/Divider';
import WorkflowInspectorPanel from './WorkflowInspectorPanel';

export type WorkflowEdgeSummary = {
  id: string;
  sourceLabel: string;
  targetLabel: string;
  condition: string;
  priority: number;
};

type WorkflowEdgePanelProps = {
  selectedEdge: WorkflowEdgeSummary | null;
  onDeleteEdge: () => void;
  onChange: (field: 'condition' | 'priority', value: string | number) => void;
};

export default function WorkflowEdgePanel({ selectedEdge, onDeleteEdge, onChange }: WorkflowEdgePanelProps) {
  return (
    <WorkflowInspectorPanel
      mode={selectedEdge ? 'edge' : 'idle'}
      title={selectedEdge ? '连线检查器' : '工作流检查器'}
      description={selectedEdge ? '右侧面板用于编辑当前分支的来源、目标、条件与优先级配置。' : '右侧检查器会跟随当前画布选择切换节点或连线配置。'}
      action={
        <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
          <Chip size="small" color="secondary" variant="filled" label={selectedEdge ? '正在检查连线' : '等待选择'} />
          <Button
            size="small"
            variant="outlined"
            color="error"
            startIcon={<DeleteOutlineRoundedIcon />}
            disabled={!selectedEdge}
            onClick={onDeleteEdge}
          >
            删除连线
          </Button>
        </Stack>
      }
    >
      {selectedEdge ? (
        <Stack spacing={1.5}>
          <Accordion defaultExpanded disableGutters elevation={0} sx={{ bgcolor: 'transparent', '&::before': { display: 'none' } }}>
            <AccordionSummary expandIcon={<ExpandMoreRoundedIcon />}>条件/分支</AccordionSummary>
            <AccordionDetails>
              <Stack spacing={1.5}>
                <Stack spacing={0.5}>
                  <Typography variant="subtitle2">来源节点</Typography>
                  <Typography variant="body2" color="text.secondary">
                    {selectedEdge.sourceLabel}
                  </Typography>
                </Stack>
                <Divider />
                <Stack spacing={0.5}>
                  <Typography variant="subtitle2">目标节点</Typography>
                  <Typography variant="body2" color="text.secondary">
                    {selectedEdge.targetLabel}
                  </Typography>
                </Stack>
                <Divider />
                <Stack spacing={0.5}>
                  <Typography variant="subtitle2">条件分支</Typography>
                  <TextField label="条件分支" value={selectedEdge.condition} onChange={(event) => onChange('condition', event.target.value)} fullWidth />
                </Stack>
                <Divider />
                <Stack spacing={0.5}>
                  <Typography variant="subtitle2">优先级</Typography>
                  <TextField
                    label="优先级"
                    type="number"
                    value={selectedEdge.priority}
                    onChange={(event) => onChange('priority', Number(event.target.value))}
                    fullWidth
                  />
                </Stack>
                <Typography variant="caption" color="text.secondary">
                  连线 ID：{selectedEdge.id}
                </Typography>
              </Stack>
            </AccordionDetails>
          </Accordion>
        </Stack>
      ) : null}
    </WorkflowInspectorPanel>
  );
}
