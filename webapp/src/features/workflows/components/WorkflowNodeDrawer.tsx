import Box from '@mui/material/Box';
import Divider from '@mui/material/Divider';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import PageCard from '../../../components/PageCard';

export const commonWorkflowNodeTypes = ['input', 'rewrite', 'review', 'render'] as const;

export type WorkflowNodeType = (typeof commonWorkflowNodeTypes)[number];

export type WorkflowNodeFormValue = {
  label: string;
  type: WorkflowNodeType;
  rawType: string;
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

const nodeTypeLabels: Record<WorkflowNodeType, string> = {
  input: '导入节点',
  rewrite: '改写节点',
  review: '审核节点',
  render: '渲染节点',
};

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

          <TextField
            label="节点类型"
            value={value.rawType}
            onChange={(event) => onChange('rawType', event.target.value)}
            helperText={commonWorkflowNodeTypes.includes(value.rawType as WorkflowNodeType) ? '常用类型可直接输入或从浏览器自动补全中选择。' : '保留当前后端类型值，保存时会原样写入节点 config_json。'}
            placeholder="例如：rewrite、review、moderate"
            inputProps={{ list: 'workflow-node-type-options' }}
            fullWidth
          />
          <datalist id="workflow-node-type-options">
            {commonWorkflowNodeTypes.map((type) => (
              <option key={type} value={type}>
                {nodeTypeLabels[type]}
              </option>
            ))}
          </datalist>

          <TextField
            label="模板标识"
            value={value.template}
            onChange={(event) => onChange('template', event.target.value)}
            helperText="写入节点 config_json.template，供后端在执行时按约定解析。"
            placeholder="例如：rewrite.standard"
            fullWidth
          />

          <TextField
            label="模型名称"
            value={value.model}
            onChange={(event) => onChange('model', event.target.value)}
            helperText="写入节点 config_json.model；是否生效由后端节点实现决定。"
            placeholder="例如：gpt-4.1-mini"
            fullWidth
          />

          <TextField
            label="上下文说明"
            value={value.context}
            onChange={(event) => onChange('context', event.target.value)}
            multiline
            minRows={6}
            helperText="写入节点 config_json.context，用于保存节点上下文说明。"
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
