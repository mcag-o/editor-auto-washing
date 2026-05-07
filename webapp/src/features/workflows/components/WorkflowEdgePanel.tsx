import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import Accordion from '@mui/material/Accordion';
import AccordionDetails from '@mui/material/AccordionDetails';
import AccordionSummary from '@mui/material/AccordionSummary';
import Button from '@mui/material/Button';
import Divider from '@mui/material/Divider';
import ExpandMoreRoundedIcon from '@mui/icons-material/ExpandMoreRounded';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import PageCard from '../../../components/PageCard';

export type WorkflowEdgeSummary = {
  id: string;
  sourceLabel: string;
  targetLabel: string;
  condition: string;
};

type WorkflowEdgePanelProps = {
  selectedEdge: WorkflowEdgeSummary | null;
  onDeleteEdge: () => void;
};

export default function WorkflowEdgePanel({ selectedEdge, onDeleteEdge }: WorkflowEdgePanelProps) {
  return (
    <PageCard
      title="连线信息"
      description="支持本地断开节点连接，用于后续补充条件分支配置。"
      action={
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
                  <Typography variant="body2" color="text.secondary">
                    {selectedEdge.condition}
                  </Typography>
                </Stack>
                <Typography variant="caption" color="text.secondary">
                  连线 ID：{selectedEdge.id}
                </Typography>
              </Stack>
            </AccordionDetails>
          </Accordion>
        </Stack>
      ) : (
        <Accordion defaultExpanded disableGutters elevation={0} sx={{ bgcolor: 'transparent', '&::before': { display: 'none' } }}>
          <AccordionSummary expandIcon={<ExpandMoreRoundedIcon />}>条件/分支</AccordionSummary>
          <AccordionDetails>
            <Typography variant="body2" color="text.secondary">
              点击画布中的连线后，可在此断开连接。当前未选择任何连线。
            </Typography>
          </AccordionDetails>
        </Accordion>
      )}
    </PageCard>
  );
}
