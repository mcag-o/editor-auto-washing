import { useEffect, useMemo, useState } from 'react';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import PauseCircleOutlineRoundedIcon from '@mui/icons-material/PauseCircleOutlineRounded';
import PlayArrowRoundedIcon from '@mui/icons-material/PlayArrowRounded';
import ReplayRoundedIcon from '@mui/icons-material/ReplayRounded';
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
import TablePagination from '@mui/material/TablePagination';
import TableRow from '@mui/material/TableRow';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import { ApiError, deleteArticle, getArticleStages, listArticles, resumeArticle, retryArticle, stopArticle } from '../../lib/api/client';
import type { ArticleStagesResponse, SourceDocument } from '../../lib/api/types';
import ConfirmDialog from '../../components/ConfirmDialog';
import PageCard from '../../components/PageCard';
import PageToolbar from '../../components/PageToolbar';
import StatusChip from '../../components/StatusChip';
import type { AppPage } from '../../layout/AppShell';

type ArticlesPageProps = {
  onNavigate?: (page: AppPage) => void;
};

type ArticleAction = 'retry' | 'stop' | 'resume' | 'delete';

const pageSizeOptions = [5, 10, 20];

function statusToLabel(status: string) {
  switch (status) {
    case 'pending':
      return '未处理';
    case 'processing':
    case 'claimed':
      return '处理中';
    case 'paused':
      return '已暂停';
    case 'completed':
      return '已处理';
    case 'failed':
      return '失败';
    default:
      return status || '未知';
  }
}

function statusToChip(status: string) {
  if (status === 'completed') {
    return 'completed' as const;
  }
  if (status === 'failed') {
    return 'failed' as const;
  }
  if (status === 'processing' || status === 'claimed') {
    return 'active' as const;
  }
  if (status === 'paused') {
    return 'disabled' as const;
  }
  return 'pending' as const;
}

function formatTime(value: string | null) {
  if (!value) {
    return '未记录';
  }

  return new Date(value).toLocaleString('zh-CN', {
    hour12: false,
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function articleWordCount(item: SourceDocument) {
  return (item.body || '').trim().length;
}

function canRetry(status: string) {
  return status === 'failed';
}

function canStop(status: string) {
  return status === 'processing';
}

function canResume(status: string) {
  return status === 'paused';
}

function canDelete(status: string) {
  return ['pending', 'paused', 'completed'].includes(status);
}

export default function ArticlesPage({ onNavigate }: ArticlesPageProps) {
  const [articles, setArticles] = useState<SourceDocument[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [quickFilter, setQuickFilter] = useState<'全部' | 'pending' | 'processing' | 'completed' | 'paused' | 'failed'>('全部');
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(5);
  const [pendingAction, setPendingAction] = useState<{ action: ArticleAction; row: SourceDocument } | null>(null);
  const [actionLoading, setActionLoading] = useState(false);
  const [selectedArticleId, setSelectedArticleId] = useState<string | null>(null);
  const [stageDetail, setStageDetail] = useState<ArticleStagesResponse | null>(null);
  const [stagesLoading, setStagesLoading] = useState(false);

  const loadArticles = async () => {
    setLoading(true);
    setError(null);

    try {
      const items = await listArticles();
      setArticles(items);
    } catch (apiError) {
      setError(apiError instanceof ApiError ? apiError.message : '文章列表加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadArticles();
  }, []);

  useEffect(() => {
    if (!selectedArticleId) {
      setStageDetail(null);
      return;
    }

    const controller = new AbortController();
    setStagesLoading(true);

    getArticleStages(selectedArticleId, { signal: controller.signal })
      .then((payload) => setStageDetail(payload))
      .catch((apiError: unknown) => {
        if (controller.signal.aborted) {
          return;
        }
        setError(apiError instanceof ApiError ? apiError.message : '阶段信息加载失败');
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setStagesLoading(false);
        }
      });

    return () => controller.abort();
  }, [selectedArticleId]);

  const filteredRows = useMemo(() => {
    return articles.filter((row) => {
      const normalizedStatus = row.status === 'claimed' ? 'processing' : row.status;
      const matchesStatus = quickFilter === '全部' ? true : normalizedStatus === quickFilter;
      const matchesKeyword = keyword.trim()
        ? `${row.title} ${row.original_filename} ${row.id}`.toLowerCase().includes(keyword.trim().toLowerCase())
        : true;
      return matchesStatus && matchesKeyword;
    });
  }, [articles, keyword, quickFilter]);

  const visibleRows = useMemo(() => {
    const start = page * rowsPerPage;
    return filteredRows.slice(start, start + rowsPerPage);
  }, [filteredRows, page, rowsPerPage]);

  const openActionDialog = (action: ArticleAction, row: SourceDocument) => {
    setPendingAction({ action, row });
  };

  const closeActionDialog = () => {
    setPendingAction(null);
  };

  const handleConfirmAction = async () => {
    if (!pendingAction) {
      return;
    }

    setActionLoading(true);
    setError(null);
    setSuccessMessage(null);

    try {
      const { action, row } = pendingAction;
      if (action === 'retry') {
        const response = await retryArticle(row.id);
        setArticles((current) => current.map((item) => (item.id === row.id ? response.article : item)));
        setSuccessMessage(response.message);
      } else if (action === 'stop') {
        const response = await stopArticle(row.id);
        setArticles((current) => current.map((item) => (item.id === row.id ? response.article : item)));
        setSuccessMessage(response.message);
      } else if (action === 'resume') {
        const response = await resumeArticle(row.id);
        setArticles((current) => current.map((item) => (item.id === row.id ? response.article : item)));
        setSuccessMessage(response.message);
      } else {
        await deleteArticle(row.id);
        setArticles((current) => current.filter((item) => item.id !== row.id));
        if (selectedArticleId === row.id) {
          setSelectedArticleId(null);
          setStageDetail(null);
        }
        setSuccessMessage('文章已删除。');
      }
      closeActionDialog();
    } catch (apiError) {
      setError(apiError instanceof ApiError ? apiError.message : '文章操作失败');
    } finally {
      setActionLoading(false);
    }
  };

  const filterActions = [
    { key: 'pending', label: '未处理' },
    { key: 'processing', label: '处理中' },
    { key: 'paused', label: '已暂停' },
    { key: 'completed', label: '已处理' },
    { key: 'failed', label: '失败' },
  ] as const;

  return (
    <>
      <Stack spacing={3}>
        <PageToolbar
          title="文章列表"
          description="使用现有文章列表、阶段查询与队列控制接口承接真实文章队列。"
          leading={<StatusChip status="active" label="实时队列视图" />}
          actions={
            <>
              <Button variant="outlined" onClick={() => onNavigate?.('overview')}>
                返回总览
              </Button>
              <Button variant="contained" onClick={() => onNavigate?.('intake')}>
                新增导入
              </Button>
            </>
          }
          filters={
            <Stack spacing={1.5}>
              <Stack direction={{ xs: 'column', md: 'row' }} spacing={1.25} alignItems={{ xs: 'stretch', md: 'center' }}>
                <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                  <Chip
                    label="全部"
                    color={quickFilter === '全部' ? 'primary' : 'default'}
                    variant={quickFilter === '全部' ? 'filled' : 'outlined'}
                    onClick={() => {
                      setQuickFilter('全部');
                      setPage(0);
                    }}
                  />
                  {filterActions.map((item) => (
                    <Chip
                      key={item.key}
                      label={item.label}
                      color={quickFilter === item.key ? 'primary' : 'default'}
                      variant={quickFilter === item.key ? 'filled' : 'outlined'}
                      onClick={() => {
                        setQuickFilter(item.key);
                        setPage(0);
                      }}
                    />
                  ))}
                </Stack>
                <TextField
                  size="small"
                  placeholder="搜索标题、来源或编号"
                  value={keyword}
                  onChange={(event) => {
                    setKeyword(event.target.value);
                    setPage(0);
                  }}
                  InputProps={{
                    startAdornment: (
                      <InputAdornment position="start">
                        <SearchRoundedIcon fontSize="small" color="action" />
                      </InputAdornment>
                    ),
                  }}
                  sx={{ minWidth: { md: 280 } }}
                />
                <Button variant="outlined" onClick={() => void loadArticles()} disabled={loading}>
                  刷新列表
                </Button>
              </Stack>
              <Typography variant="body2" color="text.secondary">
                顶部快速过滤固定包含未处理、处理中、已暂停、已处理、失败，操作区按后端允许状态发起请求。
              </Typography>
            </Stack>
          }
        />

        {error ? <Alert severity="error">{error}</Alert> : null}
        {successMessage ? <Alert severity="success">{successMessage}</Alert> : null}

        <PageCard
          title="队列表格"
          description="展示真实文章队列，并在右侧保留阶段查看入口。"
          action={loading ? <CircularProgress size={18} /> : <StatusChip status="completed" label={`共 ${filteredRows.length} 条`} />}
        >
          <TableContainer>
            <Table sx={{ minWidth: 880 }}>
              <TableHead>
                <TableRow>
                  <TableCell>文章编号</TableCell>
                  <TableCell>标题</TableCell>
                  <TableCell>导入来源</TableCell>
                  <TableCell>状态</TableCell>
                  <TableCell align="right">字数</TableCell>
                  <TableCell>更新时间</TableCell>
                  <TableCell align="right">操作</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {visibleRows.map((row) => (
                  <TableRow key={row.id} hover selected={selectedArticleId === row.id} onClick={() => setSelectedArticleId(row.id)} sx={{ cursor: 'pointer' }}>
                    <TableCell>{row.id}</TableCell>
                    <TableCell>
                      <Stack spacing={0.5}>
                        <Typography variant="subtitle1">{row.title || row.original_filename || row.id}</Typography>
                        <Typography variant="body2" color="text.secondary">
                          {row.error_summary || '点击行可查看阶段详情'}
                        </Typography>
                      </Stack>
                    </TableCell>
                    <TableCell>{row.source_type || row.file_type || '未知'}</TableCell>
                    <TableCell>
                      <StatusChip status={statusToChip(row.status)} label={statusToLabel(row.status)} />
                    </TableCell>
                    <TableCell align="right">{articleWordCount(row).toLocaleString()}</TableCell>
                    <TableCell>{formatTime(row.completed_at ?? row.processing_started_at ?? row.imported_at)}</TableCell>
                    <TableCell align="right">
                      <Stack direction="row" spacing={1} justifyContent="flex-end" flexWrap="wrap" useFlexGap>
                        <Button size="small" variant="outlined" startIcon={<ReplayRoundedIcon />} disabled={!canRetry(row.status)} onClick={(event) => {
                          event.stopPropagation();
                          openActionDialog('retry', row);
                        }}>
                          重新入队
                        </Button>
                        <Button size="small" variant="outlined" color="warning" startIcon={<PauseCircleOutlineRoundedIcon />} disabled={!canStop(row.status)} onClick={(event) => {
                          event.stopPropagation();
                          openActionDialog('stop', row);
                        }}>
                          请求暂停
                        </Button>
                        <Button size="small" variant="outlined" startIcon={<PlayArrowRoundedIcon />} disabled={!canResume(row.status)} onClick={(event) => {
                          event.stopPropagation();
                          openActionDialog('resume', row);
                        }}>
                          尝试恢复
                        </Button>
                        <Button size="small" variant="outlined" color="error" startIcon={<DeleteOutlineRoundedIcon />} disabled={!canDelete(row.status)} onClick={(event) => {
                          event.stopPropagation();
                          openActionDialog('delete', row);
                        }}>
                           删除记录
                        </Button>
                      </Stack>
                    </TableCell>
                  </TableRow>
                ))}
                {!loading && visibleRows.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={7}>
                      <Typography variant="body2" color="text.secondary" sx={{ py: 4, textAlign: 'center' }}>
                        当前过滤条件下没有匹配文章。
                      </Typography>
                    </TableCell>
                  </TableRow>
                ) : null}
              </TableBody>
            </Table>
          </TableContainer>
          <TablePagination
            component="div"
            count={filteredRows.length}
            page={page}
            onPageChange={(_, nextPage) => setPage(nextPage)}
            rowsPerPage={rowsPerPage}
            onRowsPerPageChange={(event) => {
              setRowsPerPage(Number(event.target.value));
              setPage(0);
            }}
            rowsPerPageOptions={pageSizeOptions}
            labelRowsPerPage="每页条数"
          />
        </PageCard>

        <PageCard
          title="阶段详情"
          description="按选中文章展示当前运行批次与阶段记录。"
          action={stagesLoading ? <CircularProgress size={18} /> : <StatusChip status={stageDetail ? 'active' : 'disabled'} label={stageDetail ? '已加载' : '未选择'} />}
        >
          <Stack spacing={1.5}>
            {!selectedArticleId ? (
              <Typography variant="body2" color="text.secondary">
                请先选择一篇文章查看阶段详情。
              </Typography>
            ) : stagesLoading ? (
              <Typography variant="body2" color="text.secondary">正在加载阶段信息...</Typography>
            ) : stageDetail ? (
              <>
                <Typography variant="subtitle1">
                  当前运行：{stageDetail.run ? `${stageDetail.run.status} / ${stageDetail.run.current_stage || '未进入阶段'}` : '尚未创建改写运行'}
                </Typography>
                {stageDetail.stages.length === 0 ? (
                  <Typography variant="body2" color="text.secondary">
                    当前文章还没有阶段记录。
                  </Typography>
                ) : (
                  stageDetail.stages.map((stage) => (
                    <Stack key={stage.id} spacing={0.5} sx={{ p: 1.5, borderRadius: 3, border: '1px solid', borderColor: 'divider' }}>
                      <Stack direction={{ xs: 'column', md: 'row' }} justifyContent="space-between" spacing={1}>
                        <Typography variant="subtitle2">{stage.stage_name}</Typography>
                        <StatusChip status={statusToChip(stage.status)} label={stage.status} />
                      </Stack>
                      <Typography variant="body2" color="text.secondary">
                        类型：{stage.stage_type || '未标注'} / 尝试次数：{stage.attempt}
                      </Typography>
                      {stage.error_summary ? (
                        <Typography variant="body2" color="error.main">
                          {stage.error_summary}
                        </Typography>
                      ) : null}
                    </Stack>
                  ))
                )}
              </>
            ) : (
              <Typography variant="body2" color="text.secondary">当前文章阶段信息不可用。</Typography>
            )}
          </Stack>
        </PageCard>
      </Stack>

      <ConfirmDialog
        open={Boolean(pendingAction)}
        title={pendingAction ? `确认${pendingAction.action === 'retry' ? '重新入队' : pendingAction.action === 'stop' ? '请求暂停' : pendingAction.action === 'resume' ? '尝试恢复' : '删除记录'}文章` : '操作确认'}
        description={
          pendingAction
            ? pendingAction.action === 'retry'
              ? `将把《${pendingAction.row.title || pendingAction.row.original_filename || pendingAction.row.id}》重新放回待处理队列。`
              : pendingAction.action === 'stop'
                ? `将为《${pendingAction.row.title || pendingAction.row.original_filename || pendingAction.row.id}》提交协作暂停请求，不会强制中断正在执行的处理。`
                : pendingAction.action === 'resume'
                  ? `将尝试恢复《${pendingAction.row.title || pendingAction.row.original_filename || pendingAction.row.id}》。仅当后端确认状态安全时才会重新入队。`
                  : `将删除《${pendingAction.row.title || pendingAction.row.original_filename || pendingAction.row.id}》及允许清理的相关运行记录。`
            : ''
        }
        confirmText={actionLoading ? '处理中...' : '确认执行'}
        cancelText="返回"
        confirmColor={pendingAction?.action === 'delete' ? 'error' : pendingAction?.action === 'stop' ? 'warning' : 'primary'}
        onClose={closeActionDialog}
        onConfirm={() => {
          void handleConfirmAction();
        }}
      />
    </>
  );
}
