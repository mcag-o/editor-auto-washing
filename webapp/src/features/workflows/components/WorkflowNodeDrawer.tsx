import Box from '@mui/material/Box';
import Divider from '@mui/material/Divider';
import MenuItem from '@mui/material/MenuItem';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import PageCard from '../../../components/PageCard';

export type WorkflowNodeType = 'input' | 'rewrite' | 'review' | 'render';

export type WorkflowNodeFormValue = {
  label: string;
  type: WorkflowNodeType;
  template: string;
  model: string;
  context: string;
};

type WorkflowNodeDrawerProps = {
  entryNodeLabel: string | null;
  selectedNodeId: string | null;
  value: WorkflowNodeFormValue | null;
  onChange: <K extends keyof WorkflowNodeFormValue>(field: K, nextValue: WorkflowNodeFormValue[K]) => void;
};

const nodeTypeOptions: Array<{ value: WorkflowNodeType; label: string }> = [
  { value: 'input', label: '导入节点' },
  { value: 'rewrite', label: '改写节点' },
  { value: 'review', label: '审核节点' },
  { value: 'render', label: '渲染节点' },
];

export default function WorkflowNodeDrawer({
  entryNodeLabel,
  selectedNodeId,
  value,
  onChange,
}: WorkflowNodeDrawerProps) {
  return (
    <PageCard
      title="节点配置"
      description="右侧面板用于编辑节点配置，保存时会写入工作流定义中的节点配置 JSON。"
      action={
        <Typography variant="caption" color="text.secondary">
          {entryNodeLabel ? `入口节点：${entryNodeLabel}` : '尚未设置入口节点'}
        </Typography>
      }
    >
      {value ? (
        <Stack spacing={2}>
          <Box>
            <Typography variant="subtitle2">当前节点</Typography>
            <Typography variant="body2" color="text.secondary">
              {selectedNodeId}
            </Typography>
          </Box>

          <Divider />

          <TextField label="节点名称" value={value.label} onChange={(event) => onChange('label', event.target.value)} fullWidth />

          <TextField select label="节点类型" value={value.type} onChange={(event) => onChange('type', event.target.value as WorkflowNodeType)} fullWidth>
            {nodeTypeOptions.map((option) => (
              <MenuItem key={option.value} value={option.value}>
                {option.label}
              </MenuItem>
            ))}
          </TextField>

          <TextField
            label="模板标识"
            value={value.template}
            onChange={(event) => onChange('template', event.target.value)}
            placeholder="例如：rewrite.standard"
            fullWidth
          />

          <TextField
            label="模型名称"
            value={value.model}
            onChange={(event) => onChange('model', event.target.value)}
            placeholder="例如：gpt-4.1-mini"
            fullWidth
          />

          <TextField
            label="上下文说明"
            value={value.context}
            onChange={(event) => onChange('context', event.target.value)}
            multiline
            minRows={6}
            placeholder="描述该节点需要的上下文、提示词片段或业务约束。"
            fullWidth
          />
        </Stack>
      ) : (
        <Stack spacing={1}>
          <Typography variant="subtitle2">未选择节点</Typography>
          <Typography variant="body2" color="text.secondary">
            请在中间画布点击一个节点，再在此编辑节点名称、类型、模板、模型和上下文配置。
          </Typography>
        </Stack>
      )}
    </PageCard>
  );
}
