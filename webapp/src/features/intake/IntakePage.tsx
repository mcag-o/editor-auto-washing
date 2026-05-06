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
import { ApiError, pasteIntake, uploadIntake } from '../../lib/api/client';
import type { SourceDocument } from '../../lib/api/types';
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
  const [uploadedItems, setUploadedItems] = useState<SourceDocument[]>([]);
  const [pasteValue, setPasteValue] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [submittingPaste, setSubmittingPaste] = useState(false);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

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
      setSuccessMessage(null);
      event.target.value = '';
      return;
    }

    setError(null);
    setSuccessMessage(null);
    setSelectedFiles(files);
  };

  const handleReset = () => {
    setSelectedFiles([]);
    setPasteValue('');
    setError(null);
    setSuccessMessage(null);
    if (inputRef.current) {
      inputRef.current.value = '';
    }
  };

  const handleUpload = async () => {
    if (selectedFiles.length === 0) {
      setError('请先选择至少一个文件。');
      return;
    }

    setUploading(true);
    setError(null);
    setSuccessMessage(null);

    try {
      const results = await Promise.all(selectedFiles.map((file) => uploadIntake(file)));
      setUploadedItems((current) => [...results, ...current]);
      setSuccessMessage(`已成功导入 ${results.length} 个文件。`);
      setSelectedFiles([]);
      if (inputRef.current) {
        inputRef.current.value = '';
      }
    } catch (apiError) {
      setError(apiError instanceof ApiError ? apiError.message : '文件上传失败');
    } finally {
      setUploading(false);
    }
  };

  const handlePasteSubmit = async () => {
    const trimmed = pasteValue.trim();
    if (!trimmed) {
      setError('请先粘贴原文内容。');
      return;
    }

    setSubmittingPaste(true);
    setError(null);
    setSuccessMessage(null);

    try {
      const firstLine = trimmed.split('\n').map((line) => line.trim()).find(Boolean) ?? '浏览器粘贴导入';
      const item = await pasteIntake({ title: firstLine.slice(0, 80), body: pasteValue });
      setUploadedItems((current) => [item, ...current]);
      setSuccessMessage(`已成功导入文章《${item.title || '未命名文章'}》。`);
      setPasteValue('');
    } catch (apiError) {
      setError(apiError instanceof ApiError ? apiError.message : '粘贴导入失败');
    } finally {
      setSubmittingPaste(false);
    }
  };

  return (
    <Stack spacing={3}>
      <PageToolbar
        title="文章导入"
        description="当前仅保留浏览器文件上传与文本粘贴入口，导入后写入文章队列并进入后续处理链路。"
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
            <StatusChip status="disabled" label="仅支持浏览器上传与粘贴" />
            <Typography variant="body2" color="text.secondary">
              当前页面已接入真实导入接口，成功后会新增文章记录。
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
          description="适用于批量导入已有原文文件，提交后直接调用后端上传接口。"
          action={<StatusChip status={uploading ? 'active' : selectedFiles.length > 0 ? 'pending' : 'disabled'} label={uploading ? '上传中' : selectedFiles.length > 0 ? '待提交' : '待选择'} />}
        >
          <Stack spacing={2}>
            {error ? <Alert severity="error">{error}</Alert> : null}
            {successMessage ? <Alert severity="success">{successMessage}</Alert> : null}
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
                  通过文件选择器导入内容，当前仅接受 {allowedTypesLabel} 文件。
                </Typography>
                <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.25}>
                  <Button variant="contained" onClick={handleChooseFiles} disabled={uploading}>
                    选择文件
                  </Button>
                  <Button variant="contained" onClick={handleUpload} disabled={uploading || selectedFiles.length === 0}>
                    提交上传
                  </Button>
                  <Button variant="text" color="inherit" startIcon={<RestartAltRoundedIcon />} onClick={handleReset} disabled={uploading}>
                    清空内容
                  </Button>
                </Stack>
                <input ref={inputRef} type="file" hidden multiple accept=".txt,.md,.json" onChange={handleFileChange} />
              </Stack>
            </Box>
            <Stack spacing={1.25}>
              {selectedFiles.length === 0 ? (
                <Typography variant="body2" color="text.secondary">
                  尚未选择文件。上传后将在此处显示待提交清单。
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
          description="适合临时导入单篇原文，提交后直接调用后端粘贴接口。"
          action={<StatusChip status={submittingPaste ? 'active' : pasteValue.trim() ? 'pending' : 'disabled'} label={submittingPaste ? '提交中' : pasteValue.trim() ? '待提交' : '待输入'} />}
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
                  当前字数 {pasteValue.trim().length}。
                </Typography>
              </Stack>
              <Stack direction="row" spacing={1.25}>
                <Button variant="outlined" onClick={handlePasteSubmit} disabled={submittingPaste || !pasteValue.trim()}>
                  提交粘贴
                </Button>
                <Button variant="outlined" startIcon={<RestartAltRoundedIcon />} onClick={() => setPasteValue('')} disabled={submittingPaste}>
                  清空文本
                </Button>
              </Stack>
            </Stack>
          </Stack>
        </PageCard>
      </Box>

      <PageCard
        title="最近导入结果"
        description="展示当前页面本次会话内新写入的文章记录，便于快速跳转队列。"
        action={<StatusChip status="completed" label={`已导入 ${uploadedItems.length} 条`} />}
      >
        <Stack spacing={1.25}>
          {uploadedItems.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              当前会话还没有新的导入结果。
            </Typography>
          ) : (
            uploadedItems.map((item) => (
              <Stack
                key={item.id}
                direction={{ xs: 'column', md: 'row' }}
                spacing={1.25}
                alignItems={{ xs: 'flex-start', md: 'center' }}
                justifyContent="space-between"
                sx={{ p: 1.5, borderRadius: 3, border: '1px solid', borderColor: 'divider' }}
              >
                <Box>
                  <Typography variant="subtitle1">{item.title || item.original_filename || item.id}</Typography>
                  <Typography variant="body2" color="text.secondary">
                    {item.source_type} / {item.file_type || 'text'} / {item.id}
                  </Typography>
                </Box>
                <StatusChip status={item.status === 'completed' ? 'completed' : item.status === 'failed' ? 'failed' : 'pending'} label={item.status} />
              </Stack>
            ))
          )}
        </Stack>
      </PageCard>
    </Stack>
  );
}
