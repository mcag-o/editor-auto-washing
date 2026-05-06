import { useMemo, useState } from 'react';
import SearchRoundedIcon from '@mui/icons-material/SearchRounded';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
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
import PageCard from '../../components/PageCard';
import PageToolbar from '../../components/PageToolbar';
import StatusChip from '../../components/StatusChip';
import type { AppPage } from '../../layout/AppShell';

type AuditPageProps = {
  onNavigate?: (page: AppPage) => void;
};

type AuditLevel = '全部' | '信息' | '警告' | '错误';

type AuditRow = {
  id: string;
  time: string;
  actor: string;
  module: string;
  action: string;
  level: Exclude<AuditLevel, '全部'>;
  detail: string;
};

const rows: AuditRow[] = [
  { id: 'AUD-801', time: '今天 10:48', actor: '运营值守', module: '流程控制', action: '启动流程', level: '信息', detail: '执行本地启动占位动作，等待后续接入真实控制接口。' },
  { id: 'AUD-802', time: '今天 09:16', actor: '系统', module: '文章导入', action: '上传文件', level: '信息', detail: '新增 3 个待处理文件，当前仍为本地模拟记录。' },
  { id: 'AUD-803', time: '昨天 18:02', actor: '运营管理员', module: '系统配置', action: '调整模板', level: '警告', detail: '默认模板从标准链路切换为高审校链路，尚未写入后端。' },
  { id: 'AUD-804', time: '昨天 15:27', actor: '系统', module: '操作审计', action: '过滤查询', level: '信息', detail: '使用本地关键词过滤查看最近变更。' },
  { id: 'AUD-805', time: '周一 11:05', actor: '值守机器人', module: '流程控制', action: '暂停提醒', level: '错误', detail: '模拟告警项：真实告警服务尚未接入，当前仅展示错误样式。' },
];

export default function AuditPage({ onNavigate }: AuditPageProps) {
  const [levelFilter, setLevelFilter] = useState<AuditLevel>('全部');
  const [keyword, setKeyword] = useState('');

  const filteredRows = useMemo(() => {
    return rows.filter((row) => {
      const matchesLevel = levelFilter === '全部' ? true : row.level === levelFilter;
      const matchesKeyword = keyword.trim()
        ? `${row.id} ${row.actor} ${row.module} ${row.action} ${row.detail}`.toLowerCase().includes(keyword.trim().toLowerCase())
        : true;
      return matchesLevel && matchesKeyword;
    });
  }, [keyword, levelFilter]);

  const levels: AuditLevel[] = ['全部', '信息', '警告', '错误'];

  return (
    <Stack spacing={3}>
      <PageToolbar
        title="操作审计"
        description="使用 Material UI Table 组织本地审计记录、等级过滤与关键词查询，不引入 DataGrid。"
        leading={<StatusChip status="pending" label="本地审计视图" />}
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
                  <Chip
                    key={level}
                    label={level}
                    color={levelFilter === level ? 'primary' : 'default'}
                    variant={levelFilter === level ? 'filled' : 'outlined'}
                    onClick={() => setLevelFilter(level)}
                  />
                ))}
              </Stack>
              <TextField
                size="small"
                placeholder="搜索操作人、模块、动作或详情"
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
              当前结果 {filteredRows.length} 条，过滤与搜索均为本地状态，不请求后端。
            </Typography>
          </Stack>
        }
      />

      <PageCard
        title="审计记录表"
        description="保留操作编号、时间、模块、等级与详情字段，为后续真实审计接口提供稳定布局。"
        action={<StatusChip status="completed" label={`共 ${filteredRows.length} 条`} />}
      >
        <TableContainer>
          <Table sx={{ minWidth: 960 }}>
            <TableHead>
              <TableRow>
                <TableCell>记录编号</TableCell>
                <TableCell>时间</TableCell>
                <TableCell>操作人</TableCell>
                <TableCell>模块</TableCell>
                <TableCell>动作</TableCell>
                <TableCell>等级</TableCell>
                <TableCell>详情</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {filteredRows.map((row) => (
                <TableRow key={row.id} hover>
                  <TableCell>{row.id}</TableCell>
                  <TableCell>{row.time}</TableCell>
                  <TableCell>{row.actor}</TableCell>
                  <TableCell>{row.module}</TableCell>
                  <TableCell>{row.action}</TableCell>
                  <TableCell>
                    <StatusChip
                      status={row.level === '错误' ? 'failed' : row.level === '警告' ? 'pending' : 'completed'}
                      label={row.level}
                    />
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" color="text.secondary">
                      {row.detail}
                    </Typography>
                  </TableCell>
                </TableRow>
              ))}
              {filteredRows.length === 0 ? (
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
    </Stack>
  );
}
