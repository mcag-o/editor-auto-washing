import SearchRoundedIcon from '@mui/icons-material/SearchRounded';
import Button from '@mui/material/Button';
import InputAdornment from '@mui/material/InputAdornment';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { useMemo, useState } from 'react';
import { alpha } from '@mui/material/styles';
import { commonWorkflowNodeTypes, type WorkflowNodeType } from './WorkflowNodeDrawer';

const nodeTypeLabels: Record<WorkflowNodeType, string> = {
  input: '导入节点',
  rewrite: '改写节点',
  review: '审核节点',
  render: '渲染节点',
};

type WorkflowNodeCreateMenuProps = {
  mode: 'create' | 'append';
  onClose: () => void;
  onSelectType: (type: WorkflowNodeType) => void;
};

export default function WorkflowNodeCreateMenu({ mode, onClose, onSelectType }: WorkflowNodeCreateMenuProps) {
  const [query, setQuery] = useState('');

  const filteredTypes = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    if (!normalizedQuery) {
      return commonWorkflowNodeTypes;
    }

    return commonWorkflowNodeTypes.filter((type) => {
      const label = nodeTypeLabels[type].toLowerCase();
      return type.includes(normalizedQuery) || label.includes(normalizedQuery);
    });
  }, [query]);

  return (
    <Paper
      elevation={0}
      sx={{
        width: 320,
        p: 1.75,
        borderRadius: 4,
        border: `1px solid ${alpha('#15304f', 0.12)}`,
        bgcolor: alpha('#ffffff', 0.97),
        backdropFilter: 'blur(18px)',
        boxShadow: '0 24px 60px rgba(20, 32, 51, 0.18)',
      }}
    >
      <Stack spacing={1.5}>
        <Stack spacing={0.5}>
          <Typography variant="subtitle2">{mode === 'append' ? '为当前节点追加下游节点' : '在空白画布创建节点'}</Typography>
          <Typography variant="body2" color="text.secondary">
            先搜索节点类型，再创建到当前工作流画布。
          </Typography>
        </Stack>

        <TextField
          label="搜索节点类型"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          size="small"
          autoFocus
          fullWidth
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <SearchRoundedIcon fontSize="small" />
              </InputAdornment>
            ),
          }}
        />

        <Stack spacing={1}>
          {filteredTypes.length > 0 ? (
            filteredTypes.map((type) => (
              <Button key={type} variant="outlined" color="inherit" onClick={() => onSelectType(type)}>
                {nodeTypeLabels[type]}
              </Button>
            ))
          ) : (
            <Typography variant="body2" color="text.secondary">
              未找到匹配的节点类型。
            </Typography>
          )}
        </Stack>

        <Button variant="text" color="inherit" onClick={onClose}>
          关闭
        </Button>
      </Stack>
    </Paper>
  );
}
