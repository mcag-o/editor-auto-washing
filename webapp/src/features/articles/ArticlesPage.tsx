import { useMemo, useState } from 'react';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import PauseCircleOutlineRoundedIcon from '@mui/icons-material/PauseCircleOutlineRounded';
import ReplayRoundedIcon from '@mui/icons-material/ReplayRounded';
import SearchRoundedIcon from '@mui/icons-material/SearchRounded';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
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
import ConfirmDialog from '../../components/ConfirmDialog';
import PageCard from '../../components/PageCard';
import PageToolbar from '../../components/PageToolbar';
import StatusChip from '../../components/StatusChip';

type ArticleStatus = '未处理' | '处理中' | '已处理';
type ArticleAction = '再处理' | '删除' | '停止';

type ArticleRow = {
  id: string;
  title: string;
  source: string;
  status: ArticleStatus;
  updatedAt: string;
  words: number;
};

type ArticlesPageProps = {
  onNavigate?: (page: 'dashboard' | 'intake' | 'articles') => void;
};

const rows: ArticleRow[] = [
  { id: 'ART-301', title: '品牌周报改写稿', source: '手动上传 .md', status: '未处理', updatedAt: '今天 10:30', words: 1420 },
  { id: 'ART-302', title: '新品活动总结', source: '粘贴文本', status: '处理中', updatedAt: '今天 09:48', words: 2160 },
  { id: 'ART-303', title: '行业观察草稿', source: '手动上传 .txt', status: '已处理', updatedAt: '昨天 18:12', words: 1985 },
  { id: 'ART-304', title: 'FAQ 整理文档', source: '手动上传 .json', status: '处理中', updatedAt: '昨天 16:05', words: 880 },
  { id: 'ART-305', title: '客户案例原文', source: '粘贴文本', status: '未处理', updatedAt: '昨天 11:27', words: 2640 },
  { id: 'ART-306', title: '月度复盘摘要', source: '手动上传 .md', status: '已处理', updatedAt: '周一 14:50', words: 1235 },
];

const pageSizeOptions = [5, 10, 20];

export default function ArticlesPage({ onNavigate }: ArticlesPageProps) {
  const [quickFilter, setQuickFilter] = useState<ArticleStatus | '全部'>('全部');
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(5);
  const [pendingAction, setPendingAction] = useState<{ action: ArticleAction; row: ArticleRow } | null>(null);

  const filteredRows = useMemo(() => {
    return rows.filter((row) => {
      const matchesStatus = quickFilter === '全部' ? true : row.status === quickFilter;
      const matchesKeyword = keyword.trim()
        ? `${row.title} ${row.source} ${row.id}`.toLowerCase().includes(keyword.trim().toLowerCase())
        : true;
      return matchesStatus && matchesKeyword;
    });
  }, [keyword, quickFilter]);

  const visibleRows = useMemo(() => {
    const start = page * rowsPerPage;
    return filteredRows.slice(start, start + rowsPerPage);
  }, [filteredRows, page, rowsPerPage]);

  const openActionDialog = (action: ArticleAction, row: ArticleRow) => {
    setPendingAction({ action, row });
  };

  const closeActionDialog = () => {
    setPendingAction(null);
  };

  const filterActions = ['未处理', '处理中', '已处理'] as const;

  return (
    <>
      <Stack spacing={3}>
        <PageToolbar
          title="文章列表"
          description="使用 Material UI Table 组合工具栏、过滤器与分页壳层，先承接本地 mock 数据。"
          leading={<StatusChip status="active" label="本地队列视图" />}
          actions={
            <>
              <Button variant="outlined" onClick={() => onNavigate?.('dashboard')}>
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
                  {filterActions.map((label) => (
                    <Chip
                      key={label}
                      label={label}
                      color={quickFilter === label ? 'primary' : 'default'}
                      variant={quickFilter === label ? 'filled' : 'outlined'}
                      onClick={() => {
                        setQuickFilter(label);
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
                  InputProps={{ startAdornment: <SearchRoundedIcon fontSize="small" color="action" sx={{ mr: 1 }} /> }}
                  sx={{ minWidth: { md: 280 } }}
                />
              </Stack>
              <Typography variant="body2" color="text.secondary">
                顶部快速过滤固定包含未处理、处理中、已处理，操作区展示再处理、删除、停止按钮。
              </Typography>
            </Stack>
          }
        />

        <PageCard
          title="队列表格"
          description="保留后续 API 接入所需的栏位结构与操作位置。"
          action={<StatusChip status="completed" label={`共 ${filteredRows.length} 条`} />}
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
                  <TableRow key={row.id} hover>
                    <TableCell>{row.id}</TableCell>
                    <TableCell>
                      <Stack spacing={0.5}>
                        <Typography variant="subtitle1">{row.title}</Typography>
                        <Typography variant="body2" color="text.secondary">
                          本地占位数据，等待真实接口接入
                        </Typography>
                      </Stack>
                    </TableCell>
                    <TableCell>{row.source}</TableCell>
                    <TableCell>
                      <StatusChip
                        status={row.status === '未处理' ? 'pending' : row.status === '处理中' ? 'active' : 'completed'}
                        label={row.status}
                      />
                    </TableCell>
                    <TableCell align="right">{row.words.toLocaleString()}</TableCell>
                    <TableCell>{row.updatedAt}</TableCell>
                    <TableCell align="right">
                      <Stack direction="row" spacing={1} justifyContent="flex-end" flexWrap="wrap" useFlexGap>
                        <Button size="small" variant="outlined" startIcon={<ReplayRoundedIcon />} onClick={() => openActionDialog('再处理', row)}>
                          再处理
                        </Button>
                        <Button size="small" variant="outlined" color="warning" startIcon={<PauseCircleOutlineRoundedIcon />} onClick={() => openActionDialog('停止', row)}>
                          停止
                        </Button>
                        <Button size="small" variant="outlined" color="error" startIcon={<DeleteOutlineRoundedIcon />} onClick={() => openActionDialog('删除', row)}>
                          删除
                        </Button>
                      </Stack>
                    </TableCell>
                  </TableRow>
                ))}
                {visibleRows.length === 0 ? (
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
      </Stack>

      <ConfirmDialog
        open={Boolean(pendingAction)}
        title={pendingAction ? `${pendingAction.action}文章` : '操作确认'}
        description={
          pendingAction
            ? `当前仅提供本地交互壳层，将对《${pendingAction.row.title}》执行“${pendingAction.action}”操作。真实接口接入将在后续任务完成。`
            : ''
        }
        confirmText="关闭"
        cancelText="返回"
        confirmColor={pendingAction?.action === '删除' ? 'error' : pendingAction?.action === '停止' ? 'warning' : 'primary'}
        onClose={closeActionDialog}
        onConfirm={closeActionDialog}
      />
    </>
  );
}
