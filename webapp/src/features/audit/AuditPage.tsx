import { useEffect, useMemo, useState } from 'react';
import SearchRoundedIcon from '@mui/icons-material/SearchRounded';
import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import CircularProgress from '@mui/material/CircularProgress';
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

export default function AuditPage({ onNavigate }: AuditPageProps) {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [levelFilter, setLevelFilter] = useState<AuditLevel>('全部');
  const [keyword, setKeyword] = useState('');
  const [listError, setListError] = useState<string | null>(null);
  const [detailError, setDetailError] = useState<string | null>(null);

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
    setDetailLoading(true);
    setDetailError(null);

    try {
      const detail = await getAudit(log.id);
      setSelectedLog(detail);
    } catch (apiError) {
      setDetailError(apiError instanceof ApiError ? apiError.message : '审计详情加载失败');
    } finally {
      setDetailLoading(false);
    }
  };

  return (
    <Stack spacing={3}>
      <PageToolbar
        title="操作审计"
        description="使用 Material UI Table 组织真实审计记录、等级过滤与关键词查询，不引入 DataGrid。"
        leading={<StatusChip status="active" label="后端审计视图" />}
        actions={
          <>
            <Button variant="outlined" onClick={() => onNavigate?.('overview')}>
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
          title="审计记录表"
          description="保留操作编号、时间、资源、等级与详情字段，并支持点击查看详情。"
          action={loading ? <CircularProgress size={18} /> : <StatusChip status="completed" label={`共 ${filteredRows.length} 条`} />}
        >
          <TableContainer>
            <Table sx={{ minWidth: 960 }}>
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
                    <TableRow key={row.id} hover selected={selectedLog?.id === row.id} onClick={() => void handleSelectLog(row)} sx={{ cursor: 'pointer' }}>
                      <TableCell>{row.id}</TableCell>
                      <TableCell>{formatTime(row.created_at)}</TableCell>
                      <TableCell>{row.actor}</TableCell>
                      <TableCell>{row.resource}</TableCell>
                      <TableCell>{row.action}</TableCell>
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
                {!loading && filteredRows.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={7}>
                      <Typography variant="body2" color="text.secondary" sx={{ py: 4, textAlign: 'center' }}>
                        当前过滤条件下没有匹配的审计记录。
                      </Typography>
                    </TableCell>
                  </TableRow>
                ) : null}
              </TableBody>
            </Table>
          </TableContainer>
        </PageCard>

        <PageCard
          title="审计详情"
          description="显示选中记录的 message、资源 ID 与 metadata。"
          action={detailLoading ? <CircularProgress size={18} /> : <StatusChip status={selectedLog ? 'active' : 'disabled'} label={selectedLog ? '已加载' : '未选择'} />}
        >
          {detailError ? <Alert severity="error">{detailError}</Alert> : null}
          {selectedLog ? (
            <Stack spacing={1.5}>
              <Typography variant="subtitle1">{selectedLog.action}</Typography>
              <Typography variant="body2" color="text.secondary">
                资源：{selectedLog.resource} / {selectedLog.resource_id || '未关联资源'}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                结果：{selectedLog.result}
              </Typography>
              <Typography variant="body2">{selectedLog.message || '无补充说明'}</Typography>
              <TextField multiline minRows={10} label="Metadata" value={JSON.stringify(selectedLog.metadata ?? {}, null, 2)} InputProps={{ readOnly: true }} />
            </Stack>
          ) : (
            <Typography variant="body2" color="text.secondary">
              点击左侧记录查看详情。
            </Typography>
          )}
        </PageCard>
      </Stack>
    </Stack>
  );
}
