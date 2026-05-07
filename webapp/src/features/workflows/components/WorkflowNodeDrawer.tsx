import Box from '@mui/material/Box';
import Divider from '@mui/material/Divider';
import Stack from '@mui/material/Stack';
import Tab from '@mui/material/Tab';
import Tabs from '@mui/material/Tabs';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { useEffect, useId, useState } from 'react';
import PageCard from '../../../components/PageCard';
import PageState from '../../../components/PageState';

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
  loading: boolean;
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
  loading,
  entryNodeLabel,
  selectedNodeId,
  value,
  onChange,
}: WorkflowNodeDrawerProps) {
  const [activeTab, setActiveTab] = useState('basic');
  const tabsId = useId();

  useEffect(() => {
    setActiveTab('basic');
  }, [selectedNodeId]);

  const tabPanelId = (tab: string) => `${tabsId}-${tab}-panel`;
  const tabId = (tab: string) => `${tabsId}-${tab}-tab`;

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
      {loading ? (
        <PageState title="正在加载节点配置" description="工作流定义加载完成后即可编辑节点参数。" tone="loading" />
      ) : value ? (
        <Stack spacing={2}>
          <Tabs
            value={activeTab}
            onChange={(_event, nextValue: string) => setActiveTab(nextValue)}
            variant="scrollable"
            scrollButtons="auto"
            aria-label="节点配置分组"
          >
            <Tab label="基础信息" value="basic" id={tabId('basic')} aria-controls={tabPanelId('basic')} />
            <Tab label="模板绑定" value="template" id={tabId('template')} aria-controls={tabPanelId('template')} />
            <Tab label="模型参数" value="model" id={tabId('model')} aria-controls={tabPanelId('model')} />
            <Tab label="上下文" value="context" id={tabId('context')} aria-controls={tabPanelId('context')} />
          </Tabs>

          <Divider />

          <Box role="tabpanel" hidden={activeTab !== 'basic'} id={tabPanelId('basic')} aria-labelledby={tabId('basic')}>
            {activeTab === 'basic' ? (
              <Stack spacing={2}>
                <Box>
                  <Typography variant="subtitle2">当前节点</Typography>
                  <Typography variant="body2" color="text.secondary">
                    {selectedNodeId}
                  </Typography>
                </Box>

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
              </Stack>
            ) : null}
          </Box>

          <Box role="tabpanel" hidden={activeTab !== 'template'} id={tabPanelId('template')} aria-labelledby={tabId('template')}>
            {activeTab === 'template' ? (
              <Stack spacing={2}>
                <Typography variant="subtitle2">模板绑定</Typography>
                <Typography variant="body2" color="text.secondary">
                  配置执行该节点时要绑定的模板标识。
                </Typography>
                <TextField
                  label="模板标识"
                  value={value.template}
                  onChange={(event) => onChange('template', event.target.value)}
                  helperText="写入节点 config_json.template，供后端在执行时按约定解析。"
                  placeholder="例如：rewrite.standard"
                  fullWidth
                />
              </Stack>
            ) : null}
          </Box>

          <Box role="tabpanel" hidden={activeTab !== 'model'} id={tabPanelId('model')} aria-labelledby={tabId('model')}>
            {activeTab === 'model' ? (
              <Stack spacing={2}>
                <Typography variant="subtitle2">模型参数</Typography>
                <Typography variant="body2" color="text.secondary">
                  为节点记录模型名称等执行侧参数。
                </Typography>
                <TextField
                  label="模型名称"
                  value={value.model}
                  onChange={(event) => onChange('model', event.target.value)}
                  helperText="写入节点 config_json.model；是否生效由后端节点实现决定。"
                  placeholder="例如：gpt-4.1-mini"
                  fullWidth
                />
              </Stack>
            ) : null}
          </Box>

          <Box role="tabpanel" hidden={activeTab !== 'context'} id={tabPanelId('context')} aria-labelledby={tabId('context')}>
            {activeTab === 'context' ? (
              <Stack spacing={2}>
                <Typography variant="subtitle2">上下文</Typography>
                <Typography variant="body2" color="text.secondary">
                  保存节点上下文说明、提示词片段或业务约束。
                </Typography>
                <TextField
                  label="上下文说明"
                  value={value.context}
                  onChange={(event) => onChange('context', event.target.value)}
                  multiline
                  minRows={8}
                  helperText="写入节点 config_json.context，用于保存节点上下文说明。"
                  placeholder="描述该节点需要的上下文、提示词片段或业务约束。"
                  fullWidth
                />
              </Stack>
            ) : null}
          </Box>
        </Stack>
      ) : (
        <PageState title="未选择节点" description="请在画布中选择一个节点后再编辑节点配置。" tone="empty" />
      )}
    </PageCard>
  );
}
