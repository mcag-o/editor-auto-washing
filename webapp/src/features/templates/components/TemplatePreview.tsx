import AutoAwesomeRoundedIcon from '@mui/icons-material/AutoAwesomeRounded';
import DataObjectRoundedIcon from '@mui/icons-material/DataObjectRounded';
import LayersRoundedIcon from '@mui/icons-material/LayersRounded';
import Box from '@mui/material/Box';
import Chip from '@mui/material/Chip';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import type { TemplateRecord } from '../TemplatesPage';
import StatusChip from '../../../components/StatusChip';

type TemplatePreviewProps = {
  template: TemplateRecord | null;
};

export default function TemplatePreview({ template }: TemplatePreviewProps) {
  return (
    <Paper elevation={0} sx={{ p: { xs: 2, md: 2.5 }, border: '1px solid', borderColor: 'divider' }}>
      {template ? (
        <Stack spacing={2.25}>
          <Stack direction="row" spacing={1.5} justifyContent="space-between" alignItems="flex-start">
            <Box>
              <Typography variant="h5">内容预览</Typography>
              <Typography variant="body2" color="text.secondary">
                以阅读面板方式查看模板主提示词、阶段说明与启用状态。
              </Typography>
            </Box>
            <StatusChip status={template.enabled ? 'active' : 'disabled'} />
          </Stack>

          <Box sx={{ p: 2, borderRadius: 3, bgcolor: 'background.default', border: '1px solid', borderColor: 'divider' }}>
            <Stack spacing={0.75}>
              <Typography variant="overline" color="text.secondary">
                当前选中模板
              </Typography>
              <Typography variant="h6">{template.name}</Typography>
              <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                <Chip size="small" label={template.version} />
                <Chip size="small" variant="outlined" label={`更新于 ${template.updatedAt}`} />
                <Chip size="small" variant="outlined" label={`更新人 ${template.updatedBy}`} />
              </Stack>
            </Stack>
          </Box>

          <Stack spacing={1.5}>
            <Stack direction="row" spacing={1} alignItems="center">
              <AutoAwesomeRoundedIcon fontSize="small" color="primary" />
              <Typography variant="subtitle2">主提示词</Typography>
            </Stack>
            <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: 'pre-wrap', lineHeight: 1.8 }}>
              {template.prompt}
            </Typography>
          </Stack>

          <Stack spacing={1.5}>
            <Stack direction="row" spacing={1} alignItems="center">
              <LayersRoundedIcon fontSize="small" color="primary" />
              <Typography variant="subtitle2">阶段列表</Typography>
            </Stack>
            <Stack spacing={1}>
              {template.stages.map((stage, index) => (
                <Box key={stage.key} sx={{ p: 1.5, borderRadius: 2.5, bgcolor: 'background.default', border: '1px solid', borderColor: 'divider' }}>
                  <Stack direction="row" spacing={1.25} alignItems="center">
                    <Chip size="small" label={`阶段 ${index + 1}`} />
                    <Typography variant="subtitle2">{stage.label}</Typography>
                  </Stack>
                  <Typography variant="body2" color="text.secondary" sx={{ mt: 0.75 }}>
                    {stage.note}
                  </Typography>
                </Box>
              ))}
            </Stack>
          </Stack>

          <Stack spacing={1.5}>
            <Stack direction="row" spacing={1} alignItems="center">
              <DataObjectRoundedIcon fontSize="small" color="primary" />
              <Typography variant="subtitle2">模板摘要</Typography>
            </Stack>
            <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: 'pre-wrap', lineHeight: 1.8 }}>
              {template.summary}
            </Typography>
          </Stack>
        </Stack>
      ) : (
        <Stack spacing={1.25}>
          <Typography variant="h5">内容预览</Typography>
          <Typography variant="body2" color="text.secondary">
            请选择一个模板查看右侧预览内容。
          </Typography>
        </Stack>
      )}
    </Paper>
  );
}
