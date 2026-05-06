import CloseRoundedIcon from '@mui/icons-material/CloseRounded';
import SaveRoundedIcon from '@mui/icons-material/SaveRounded';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Divider from '@mui/material/Divider';
import Drawer from '@mui/material/Drawer';
import FormControlLabel from '@mui/material/FormControlLabel';
import MenuItem from '@mui/material/MenuItem';
import Stack from '@mui/material/Stack';
import Switch from '@mui/material/Switch';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import type { TemplateDraft, TemplateRecord } from '../TemplatesPage';

type TemplateEditorDrawerProps = {
  open: boolean;
  draft: TemplateDraft;
  editingTemplate: TemplateRecord | null;
  saving?: boolean;
  onClose: () => void;
  onChange: <K extends keyof TemplateDraft>(field: K, value: TemplateDraft[K]) => void;
  onSave: () => void;
};

const versionOptions = ['v1.0.0', 'v1.0.3', 'v1.1.0', 'v1.2.0', 'v1.3.0', 'v1.4.2', 'v2.0.0', 'v2.0.1', 'v2.1.0', 'v3.0.0', 'v4.0.0'];

export default function TemplateEditorDrawer({
  open,
  draft,
  editingTemplate,
  saving = false,
  onClose,
  onChange,
  onSave,
}: TemplateEditorDrawerProps) {
  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={onClose}
      PaperProps={{ sx: { width: { xs: '100%', md: 520 }, p: 3 } }}
    >
      <Stack spacing={2.5} sx={{ height: '100%' }}>
        <Stack direction="row" spacing={1.5} alignItems="flex-start" justifyContent="space-between">
          <Box>
            <Typography variant="h5">{editingTemplate ? '编辑模板' : '新建模板'}</Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
              在抽屉中直接编辑提示词、摘要和阶段说明，保存后会写入后端模板记录。
            </Typography>
          </Box>
          <Button variant="text" onClick={onClose} startIcon={<CloseRoundedIcon />}>
            关闭
          </Button>
        </Stack>

        <Divider />

        <Stack spacing={2} sx={{ flex: 1, overflow: 'auto', pr: 0.5 }}>
          <TextField label="模板名称" value={draft.name} onChange={(event) => onChange('name', event.target.value)} fullWidth />

          <TextField
            select
            label="版本"
            value={draft.version}
            onChange={(event) => onChange('version', event.target.value)}
            fullWidth
          >
            {versionOptions.map((option) => (
              <MenuItem key={option} value={option}>
                {option}
              </MenuItem>
            ))}
          </TextField>

          <FormControlLabel
            control={<Switch checked={draft.enabled} onChange={(event) => onChange('enabled', event.target.checked)} />}
            label={draft.enabled ? '启用中' : '未启用'}
          />

          <TextField
            label="模板摘要"
            value={draft.summary}
            onChange={(event) => onChange('summary', event.target.value)}
            multiline
            minRows={3}
            fullWidth
          />

          <TextField
            label="模板类型"
            value={draft.type}
            onChange={(event) => onChange('type', event.target.value)}
            helperText="支持 prompt、rewrite、review、stage 等模板类型字段，并随保存请求一起提交到后端。"
            fullWidth
          />

          <TextField
            label="主提示词"
            value={draft.prompt}
            onChange={(event) => onChange('prompt', event.target.value)}
            multiline
            minRows={5}
            fullWidth
          />

          <TextField
            label="阶段说明"
            value={draft.stagesText}
            onChange={(event) => onChange('stagesText', event.target.value)}
            multiline
            minRows={6}
            helperText="每行一个阶段，格式为“阶段名: 说明”。"
            fullWidth
          />
        </Stack>

        <Divider />

        <Stack direction="row" spacing={1.5} justifyContent="flex-end">
          <Button variant="outlined" onClick={onClose}>
            取消
          </Button>
          <Button variant="contained" startIcon={<SaveRoundedIcon />} onClick={onSave} disabled={saving}>
              {saving ? '保存中...' : '保存模板'}
          </Button>
        </Stack>
      </Stack>
    </Drawer>
  );
}
