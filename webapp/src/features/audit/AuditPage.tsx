import { useEffect, useMemo, useRef, useState } from 'react';
import SearchRoundedIcon from '@mui/icons-material/SearchRounded';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import CircularProgress from '@mui/material/CircularProgress';
import Divider from '@mui/material/Divider';
import InputAdornment from '@mui/material/InputAdornment';
import Stack from '@mui/material/Stack';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableContainer from '@mui/material/TableContainer';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { ApiError, getAudit, listAudit } from '../../lib/api/client';
import type { AuditLog } from '../../lib/api/types';
import PageCard from '../../components/PageCard';
import PageState from '../../components/PageState';
import PageToolbar from '../../components/PageToolbar';
import StatusChip from '../../components/StatusChip';
import type { AppPage } from '../../layout/AppShell';

type AuditPageProps = {
  onNavigate?: (page: AppPage) => void;
};

type AuditLevel = '全部' | '信息' | '警告' | '错误';

function logLevel(row: AuditLog): Exclude<AuditLevel, '全部'> {
  if (row.result === 'failure') {
    return '错误';
  }
  if (row.result === 'success') {
    return '信息';
  }
  return '警告';
}

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false });
}

function resultLabel(result: string) {
  if (result === 'success') {
    return '成功';
  }
  if (result === 'failure') {
    return '失败';
  }
  return result || '未标注';
}

function metadataSummaryItems(log: AuditLog) {
  return [
    { label: '操作人', value: log.actor || '未记录' },
    { label: '操作动作', value: log.action || '未记录' },
    { label: '目标资源', value: `${log.resource || '未记录'} / ${log.resource_id || '未关联资源'}` },
    { label: '操作结果', value: resultLabel(log.result) },
    { label: '发生时间', value: formatTime(log.created_at) },
  ];
}

export default function AuditPage({ onNavigate }: AuditPageProps) {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [selectedLogID, setSelectedLogID] = useState<string | null>(null);
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [levelFilter, setLevelFilter] = useState<AuditLevel>('全部');
  const [keyword, setKeyword] = useState('');
  const [listError, setListError] = useState<string | null>(null);
  const [detailError, setDetailError] = useState<string | null>(null);
  const detailRequestSequence = useRef(0);

  useEffect(() => {
    const controller = new AbortController();

    listAudit({ signal: controller.signal })
      .then((items) => setLogs(items))
      .catch((apiError: unknown) => {
        if (controller.signal.aborted) {
          return;
        }
        setListError(apiError instanceof ApiError ? apiError.message : '审计记录加载失败');
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      });

    return () => controller.abort();
  }, []);

  const filteredRows = useMemo(() => {
    return logs.filter((row) => {
      const level = logLevel(row);
      const matchesLevel = levelFilter === '全部' ? true : level === levelFilter;
      const matchesKeyword = keyword.trim()
        ? `${row.id} ${row.actor} ${row.action} ${row.message} ${row.resource}`.toLowerCase().includes(keyword.trim().toLowerCase())
        : true;
      return matchesLevel && matchesKeyword;
    });
  }, [keyword, levelFilter, logs]);

  const levels: AuditLevel[] = ['全部', '信息', '警告', '错误'];

  const handleSelectLog = async (log: AuditLog) => {
    const requestSequence = detailRequestSequence.current + 1;
    detailRequestSequence.current = requestSequence;
    setSelectedLogID(log.id);
    setSelectedLog(null);
    setDetailLoading(true);
    setDetailError(null);

    try {
      const detail = await getAudit(log.id);
      if (detailRequestSequence.current !== requestSequence) {
        return;
      }
      setSelectedLog(detail);
    } catch (apiError) {
      if (detailRequestSequence.current !== requestSequence) {
        return;
      }
      setDetailError(apiError instanceof ApiError ? apiError.message : '审计详情加载失败');
    } finally {
      if (detailRequestSequence.current === requestSequence) {
        setDetailLoading(false);
      }
    }
  };

  useEffect(() => {
    if (loading) {
      return;
    }

    if (filteredRows.length === 0) {
      detailRequestSequence.current += 1;
      setSelectedLogID(null);
      setSelectedLog(null);
      setDetailLoading(false);
      setDetailError(null);
      return;
    }

    const selectedVisible = selectedLogID ? filteredRows.some((row) => row.id === selectedLogID) : false;
    if (!selectedVisible) {
      void handleSelectLog(filteredRows[0]);
    }
  }, [filteredRows, loading, selectedLogID]);

  return (
    <Stack spacing={3}>
      <PageToolbar
        title="操作审计"
        description="使用 Material UI Table 组织真实审计记录、等级过滤与关键词查询，不引入 DataGrid。"
        leading={<StatusChip status="active" label="后端审计视图" />}
        actions={
          <>
            <Button color="inherit" variant="text" onClick={() => onNavigate?.('overview')}>
              返回总览
            </Button>
            <Button variant="contained" onClick={() => onNavigate?.('config')}>
              查看配置
            </Button>
          </>
        }
        filters={
          <Stack spacing={1.5}>
            <Stack direction={{ xs: 'column', lg: 'row' }} spacing={1.25} alignItems={{ xs: 'stretch', lg: 'center' }}>
              <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                {levels.map((level) => (
                  <Chip key={level} label={level} color={levelFilter === level ? 'primary' : 'default'} variant={levelFilter === level ? 'filled' : 'outlined'} onClick={() => setLevelFilter(level)} />
                ))}
              </Stack>
              <TextField
                size="small"
                placeholder="搜索操作人、资源、动作或详情"
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                InputProps={{
                  startAdornment: (
                    <InputAdornment position="start">
                      <SearchRoundedIcon fontSize="small" color="action" />
                    </InputAdornment>
                  ),
                }}
                sx={{ minWidth: { lg: 320 } }}
              />
            </Stack>
            <Typography variant="body2" color="text.secondary">
              当前结果 {filteredRows.length} 条，列表与详情都来自现有审计 API。
            </Typography>
          </Stack>
        }
      />

      {listError ? <Alert severity="error">{listError}</Alert> : null}

      <Stack direction={{ xs: 'column', xl: 'row' }} spacing={3} alignItems="stretch">
        <PageCard
          testId="audit-list-card"
          title="审计记录表"
          description="保留操作编号、时间、目标资源、等级与摘要，并支持快速查看详情。"
          action={loading ? <CircularProgress size={18} /> : <StatusChip status="completed" label={`共 ${filteredRows.length} 条`} />}
        >
          {loading ? (
            <PageState title="正在加载审计记录" description="正在同步最新的操作审计数据，请稍候。" tone="loading" />
          ) : listError ? (
            <PageState title="审计记录暂时不可用" description={listError} tone="error" />
          ) : filteredRows.length === 0 ? (
            <PageState title="暂无审计记录" description="当前筛选条件下没有可显示的审计记录。" tone="empty" />
          ) : (
            <TableContainer>
              <Table size="small" sx={{ minWidth: 960 }}>
                <TableHead>
                  <TableRow>
                    <TableCell>记录编号</TableCell>
                    <TableCell>时间</TableCell>
                    <TableCell>操作人</TableCell>
                    <TableCell>资源</TableCell>
                    <TableCell>动作</TableCell>
                    <TableCell>等级</TableCell>
                    <TableCell>详情</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {filteredRows.map((row) => {
                    const level = logLevel(row);
                    return (
                     <TableRow
                         key={row.id}
                         hover
                         selected={selectedLogID === row.id}
                        onClick={() => void handleSelectLog(row)}
                        sx={{ cursor: 'pointer', '& .MuiTableCell-root': { py: 1.25, verticalAlign: 'top' } }}
                      >
                         <TableCell>{row.id}</TableCell>
                         <TableCell>{formatTime(row.created_at)}</TableCell>
                         <TableCell>
                           <Stack spacing={0.25}>
                             <Typography variant="body2" fontWeight={600}>
                               {row.actor}
                             </Typography>
                             <Typography variant="caption" color="text.secondary">
                               执行动作：{row.action}
                             </Typography>
                           </Stack>
                         </TableCell>
                         <TableCell>
                           <Stack spacing={0.25}>
                             <Typography variant="body2" fontWeight={600}>
                               {row.resource}
                             </Typography>
                             <Typography variant="caption" color="text.secondary">
                               {row.resource_id || '未关联资源'}
                             </Typography>
                           </Stack>
                         </TableCell>
                         <TableCell>
                           <Typography variant="body2">{row.action}</Typography>
                         </TableCell>
                        <TableCell>
                          <StatusChip status={level === '错误' ? 'failed' : level === '警告' ? 'pending' : 'completed'} label={level} />
                        </TableCell>
                        <TableCell>
                          <Typography variant="body2" color="text.secondary">
                            {row.message}
                          </Typography>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </PageCard>

        <PageCard
          testId="audit-detail-card"
          title="审计详情"
          description="突出操作人、动作、目标资源、结果与 metadata，便于快速判读。"
          action={detailLoading ? <CircularProgress size={18} /> : <StatusChip status={selectedLogID ? (selectedLog ? 'active' : 'failed') : 'disabled'} label={selectedLogID ? (selectedLog ? '已加载' : '加载失败') : '未选择'} />}
        >
          {detailLoading && selectedLogID ? (
            <PageState title="正在加载审计详情" description="正在同步当前选中记录的详细信息。" tone="loading" />
          ) : selectedLog ? (
            <Stack spacing={2}>
              <Stack spacing={1}>
                <Typography variant="subtitle1">{selectedLog.message || selectedLog.action}</Typography>
                <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} flexWrap="wrap" useFlexGap>
                  <StatusChip
                    status={selectedLog.result === 'failure' ? 'failed' : selectedLog.result === 'success' ? 'completed' : 'pending'}
                    label={`结果：${resultLabel(selectedLog.result)}`}
                  />
                  <StatusChip status="active" label={`动作：${selectedLog.action || '未记录'}`} />
                </Stack>
              </Stack>

              <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.25} flexWrap="wrap" useFlexGap>
                {metadataSummaryItems(selectedLog).map((item) => (
                  <Box
                    key={item.label}
                    sx={{
                      flex: { xs: '1 1 100%', md: '1 1 calc(50% - 10px)' },
                      p: 1.5,
                      borderRadius: 3,
                      border: '1px solid',
                      borderColor: 'divider',
                      bgcolor: 'background.default',
                    }}
                  >
                    <Typography variant="caption" color="text.secondary">
                      {item.label}
                    </Typography>
                    <Typography variant="body2" fontWeight={600} sx={{ mt: 0.5 }}>
                      {item.value}
                    </Typography>
                  </Box>
                ))}
              </Stack>

              <Divider />

              <Typography variant="body2">{selectedLog.message || '无补充说明'}</Typography>
              <TextField multiline minRows={10} label="Metadata" value={JSON.stringify(selectedLog.metadata ?? {}, null, 2)} InputProps={{ readOnly: true }} />
            </Stack>
          ) : detailError ? (
            <PageState title="审计详情暂时不可用" description={detailError} tone="error" />
          ) : selectedLogID ? (
            <PageState title="审计详情暂时不可用" description={detailError || '当前记录详情暂时不可用，请重新选择或稍后重试。'} tone="error" />
          ) : (
            <PageState title="未选择审计记录" description="请选择一条记录查看审计详情。" tone="empty" />
          )}
        </PageCard>
      </Stack>
    </Stack>
  );
}
