import type { ChangeEvent } from 'react';
import { useMemo, useRef, useState } from 'react';
import ContentPasteRoundedIcon from '@mui/icons-material/ContentPasteRounded';
import DescriptionRoundedIcon from '@mui/icons-material/DescriptionRounded';
import FileUploadRoundedIcon from '@mui/icons-material/FileUploadRounded';
import RestartAltRoundedIcon from '@mui/icons-material/RestartAltRounded';
import SendRoundedIcon from '@mui/icons-material/SendRounded';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import PageCard from '../../components/PageCard';
import PageToolbar from '../../components/PageToolbar';
import StatusChip from '../../components/StatusChip';
import type { AppPage } from '../../layout/AppShell';

const acceptedExtensions = ['txt', 'md', 'json'] as const;

type IntakePageProps = {
  onNavigate?: (page: AppPage) => void;
};

export default function IntakePage({ onNavigate }: IntakePageProps) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const [pasteValue, setPasteValue] = useState('');
  const [error, setError] = useState<string | null>(null);

  const allowedTypesLabel = useMemo(() => acceptedExtensions.map((item) => `.${item}`).join(' / '), []);

  const handleChooseFiles = () => {
    inputRef.current?.click();
  };

  const handleFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? []);
    const invalidFile = files.find((file) => {
      const extension = file.name.split('.').pop()?.toLowerCase();
      return extension ? !acceptedExtensions.includes(extension as (typeof acceptedExtensions)[number]) : true;
    });

    if (invalidFile) {
      setError(`仅支持 ${allowedTypesLabel} 文件，当前文件 ${invalidFile.name} 不可导入。`);
      event.target.value = '';
      return;
    }

    setError(null);
    setSelectedFiles(files);
  };

  const handleReset = () => {
    setSelectedFiles([]);
    setPasteValue('');
    setError(null);
    if (inputRef.current) {
      inputRef.current.value = '';
    }
  };

  return (
    <Stack spacing={3}>
      <PageToolbar
        title="文章导入"
        description="当前仅保留浏览器文件上传与文本粘贴入口，贴合后续默认自动改写主链路。"
        leading={<StatusChip status="active" label="浏览器导入" />}
        actions={
          <>
            <Button variant="outlined" onClick={() => onNavigate?.('overview')}>
              返回总览
            </Button>
            <Button variant="contained" startIcon={<SendRoundedIcon />} onClick={() => onNavigate?.('articles')}>
              查看导入结果
            </Button>
          </>
        }
        filters={
          <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.5} alignItems={{ xs: 'flex-start', md: 'center' }}>
            <StatusChip status="completed" label="支持 .txt / .md / .json" />
            <StatusChip status="disabled" label="不提供 URL 导入" />
            <Typography variant="body2" color="text.secondary">
              本地状态仅模拟待提交内容，不会调用真实后端接口。
            </Typography>
          </Stack>
        }
      />

      <Box
        sx={{
          display: 'grid',
          gap: 3,
          gridTemplateColumns: { xs: '1fr', xl: 'minmax(0, 1.2fr) minmax(340px, 0.8fr)' },
        }}
      >
        <PageCard
          title="文件上传"
          description="适用于批量导入已有原文文件。后续会在提交时接入真实 intake API。"
          action={<StatusChip status="pending" label="等待提交" />}
        >
          <Stack spacing={2}>
            {error ? <Alert severity="error">{error}</Alert> : null}
            <Box
              sx={{
                p: 3,
                borderRadius: 4,
                border: '1px dashed',
                borderColor: 'divider',
                bgcolor: 'background.default',
              }}
            >
              <Stack spacing={1.5} alignItems={{ xs: 'flex-start', md: 'center' }} textAlign={{ xs: 'left', md: 'center' }}>
                <FileUploadRoundedIcon color="primary" sx={{ fontSize: 36 }} />
                <Typography variant="h4">选择文件导入</Typography>
                <Typography variant="body2" color="text.secondary">
                  通过文件选择器导入内容，当前仅接受 {allowedTypesLabel} 文件，不展示 URL 导入表单。
                </Typography>
                <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.25}>
                  <Button variant="contained" onClick={handleChooseFiles}>
                    选择文件
                  </Button>
                  <Button variant="text" color="inherit" startIcon={<RestartAltRoundedIcon />} onClick={handleReset}>
                    清空内容
                  </Button>
                </Stack>
                <input ref={inputRef} type="file" hidden multiple accept=".txt,.md,.json" onChange={handleFileChange} />
              </Stack>
            </Box>
            <Stack spacing={1.25}>
              {selectedFiles.length === 0 ? (
                <Typography variant="body2" color="text.secondary">
                  尚未选择文件。上传后将在此处显示本地待提交清单。
                </Typography>
              ) : (
                selectedFiles.map((file) => (
                  <Stack
                    key={`${file.name}-${file.size}`}
                    direction="row"
                    spacing={1.25}
                    alignItems="center"
                    justifyContent="space-between"
                    sx={{ p: 1.5, borderRadius: 3, border: '1px solid', borderColor: 'divider' }}
                  >
                    <Stack direction="row" spacing={1.25} alignItems="center">
                      <DescriptionRoundedIcon color="primary" fontSize="small" />
                      <Box>
                        <Typography variant="subtitle1">{file.name}</Typography>
                        <Typography variant="body2" color="text.secondary">
                          {(file.size / 1024).toFixed(1)} KB
                        </Typography>
                      </Box>
                    </Stack>
                    <StatusChip status="pending" label="待提交" />
                  </Stack>
                ))
              )}
            </Stack>
          </Stack>
        </PageCard>

        <PageCard
          title="文本粘贴"
          description="适合临时导入单篇原文。后续可直接转入默认改写主链路。"
          action={<StatusChip status={pasteValue.trim() ? 'active' : 'disabled'} label={pasteValue.trim() ? '已填写' : '待输入'} />}
        >
          <Stack spacing={2}>
            <TextField
              multiline
              minRows={14}
              label="粘贴原文内容"
              placeholder="将文章正文、Markdown 或 JSON 内容粘贴到这里"
              value={pasteValue}
              onChange={(event) => setPasteValue(event.target.value)}
            />
            <Stack direction="row" spacing={1.25} alignItems="center" justifyContent="space-between">
              <Stack direction="row" spacing={1} alignItems="center">
                <ContentPasteRoundedIcon color="primary" fontSize="small" />
                <Typography variant="body2" color="text.secondary">
                  当前字数 {pasteValue.trim().length}，仅保留本地输入状态。
                </Typography>
              </Stack>
              <Button variant="outlined" startIcon={<RestartAltRoundedIcon />} onClick={() => setPasteValue('')}>
                清空文本
              </Button>
            </Stack>
          </Stack>
        </PageCard>
      </Box>
    </Stack>
  );
}
