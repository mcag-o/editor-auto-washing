import AddCircleOutlineRoundedIcon from '@mui/icons-material/AddCircleOutlineRounded';
import AccountTreeRoundedIcon from '@mui/icons-material/AccountTreeRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Divider from '@mui/material/Divider';
import List from '@mui/material/List';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemText from '@mui/material/ListItemText';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import PageCard from '../../../components/PageCard';
import PageState from '../../../components/PageState';

export type WorkflowTemplateSummary = {
  id: string;
  name: string;
  description: string;
  nodeCount: number;
  updatedAt: string;
};

type WorkflowListPanelProps = {
  items: WorkflowTemplateSummary[];
  loading: boolean;
  error: string | null;
  selectedId: string;
  onCreateTemplate: () => void;
  onSelectTemplate: (id: string) => void;
  onDeleteTemplate: () => void;
};

export default function WorkflowListPanel({
  items,
  loading,
  error,
  selectedId,
  onCreateTemplate,
  onSelectTemplate,
  onDeleteTemplate,
}: WorkflowListPanelProps) {
  return (
    <PageCard
      title="模板列表"
      description="左侧展示工作流模板集合与切换入口。"
      action={
        <Button size="small" variant="outlined" startIcon={<AddCircleOutlineRoundedIcon />} onClick={onCreateTemplate}>
          新建模板
        </Button>
      }
    >
      <Stack spacing={1.5}>
        <Box sx={{ p: 1.5, borderRadius: 3, bgcolor: 'background.default', border: '1px solid', borderColor: 'divider' }}>
          <Stack direction="row" spacing={1} alignItems="center">
            <AccountTreeRoundedIcon color="primary" fontSize="small" />
            <Typography variant="subtitle2">工作流模板列表</Typography>
          </Stack>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.75 }}>
            模板列表、保存与删除都直接对应后端工作流定义接口。
          </Typography>
        </Box>

          <Divider />

          <Button size="small" color="error" variant="outlined" startIcon={<DeleteOutlineRoundedIcon />} onClick={onDeleteTemplate} disabled={!selectedId}>
            删除当前模板
          </Button>

          {loading ? (
            <PageState title="正在加载工作流模板" description="正在同步当前工作流定义，请稍候。" tone="loading" />
          ) : error ? (
            <PageState title="工作流模板暂时不可用" description={error} tone="error" />
          ) : items.length === 0 ? (
            <PageState title="暂无工作流模板" description="点击上方“新建模板”开始配置新的流程定义。" tone="empty" />
          ) : (
            <List disablePadding sx={{ display: 'grid', gap: 1 }}>
              {items.map((item) => (
                <ListItemButton
                  key={item.id}
                  selected={item.id === selectedId}
                  onClick={() => onSelectTemplate(item.id)}
                  sx={{
                    borderRadius: 3,
                    border: '1px solid',
                    borderColor: item.id === selectedId ? 'primary.main' : 'divider',
                    alignItems: 'flex-start',
                    px: 1.5,
                    py: 1.25,
                  }}
                >
                  <ListItemText
                    secondaryTypographyProps={{ component: 'div' }}
                    primary={
                      <Stack direction="row" spacing={1} alignItems="center" justifyContent="space-between">
                        <Typography variant="subtitle2">{item.name}</Typography>
                        <Chip size="small" label={`${item.nodeCount} 节点`} />
                      </Stack>
                    }
                    secondary={
                      <Stack spacing={0.75} sx={{ mt: 0.75 }}>
                        <Typography component="span" variant="body2" color="text.secondary" sx={{ display: 'block' }}>
                          {item.description}
                        </Typography>
                        <Typography component="span" variant="caption" color="text.secondary" sx={{ display: 'block' }}>
                          最近更新：{item.updatedAt}
                        </Typography>
                      </Stack>
                    }
                  />
                </ListItemButton>
              ))}
            </List>
          )}
      </Stack>
    </PageCard>
  );
}
