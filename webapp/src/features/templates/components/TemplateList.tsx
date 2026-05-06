import EditRoundedIcon from '@mui/icons-material/EditRounded';
import FileCopyRoundedIcon from '@mui/icons-material/FileCopyRounded';
import MoreHorizRoundedIcon from '@mui/icons-material/MoreHorizRounded';
import PowerSettingsNewRoundedIcon from '@mui/icons-material/PowerSettingsNewRounded';
import DeleteOutlineRoundedIcon from '@mui/icons-material/DeleteOutlineRounded';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Menu from '@mui/material/Menu';
import MenuItem from '@mui/material/MenuItem';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableContainer from '@mui/material/TableContainer';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import Typography from '@mui/material/Typography';
import { useState } from 'react';
import StatusChip from '../../../components/StatusChip';
import type { TemplateRecord } from '../TemplatesPage';

type TemplateListProps = {
  items: TemplateRecord[];
  selectedId: string;
  onSelectTemplate: (id: string) => void;
  onEditTemplate: (id: string) => void;
  onToggleEnabled: (id: string) => void;
  onDuplicateTemplate: (id: string) => void;
  onDeleteTemplate: (id: string) => void;
};

export default function TemplateList({
  items,
  selectedId,
  onSelectTemplate,
  onEditTemplate,
  onToggleEnabled,
  onDuplicateTemplate,
  onDeleteTemplate,
}: TemplateListProps) {
  const [menuAnchorEl, setMenuAnchorEl] = useState<HTMLElement | null>(null);
  const [menuTargetId, setMenuTargetId] = useState<string | null>(null);

  const menuOpen = Boolean(menuAnchorEl && menuTargetId);

  const handleOpenMenu = (event: React.MouseEvent<HTMLElement>, templateId: string) => {
    setMenuAnchorEl(event.currentTarget);
    setMenuTargetId(templateId);
  };

  const handleCloseMenu = () => {
    setMenuAnchorEl(null);
    setMenuTargetId(null);
  };

  const handleEdit = () => {
    if (menuTargetId) {
      onEditTemplate(menuTargetId);
    }
    handleCloseMenu();
  };

  const handleToggle = () => {
    if (menuTargetId) {
      onToggleEnabled(menuTargetId);
    }
    handleCloseMenu();
  };

  const handleDuplicate = () => {
    if (menuTargetId) {
      onDuplicateTemplate(menuTargetId);
    }
    handleCloseMenu();
  };

  const handleDelete = () => {
    if (menuTargetId) {
      onDeleteTemplate(menuTargetId);
    }
    handleCloseMenu();
  };

  return (
    <Paper elevation={0} sx={{ p: { xs: 2, md: 2.5 }, border: '1px solid', borderColor: 'divider' }}>
      <Stack spacing={2}>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} justifyContent="space-between" alignItems={{ xs: 'flex-start', sm: 'center' }}>
          <Box>
            <Typography variant="h5">模板列表</Typography>
            <Typography variant="body2" color="text.secondary">
              以表格形式展示模板版本、启用状态、更新时间与真实操作入口。
            </Typography>
          </Box>
          <Chip label={`${items.length} 个模板`} variant="outlined" />
        </Stack>

        <TableContainer>
          <Table size="small" sx={{ minWidth: 920 }}>
            <TableHead>
              <TableRow>
                <TableCell>模板名称</TableCell>
                <TableCell>版本</TableCell>
                <TableCell>启用状态</TableCell>
                <TableCell>摘要</TableCell>
                <TableCell>阶段数</TableCell>
                <TableCell>更新时间</TableCell>
                <TableCell align="right">操作</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {items.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7}>
                    <Stack spacing={0.75} sx={{ py: 4, alignItems: 'center' }}>
                      <Typography variant="subtitle2">当前没有本地模板</Typography>
                      <Typography variant="body2" color="text.secondary">
                        点击右上角“新建模板”即可开始搭建新的提示词或阶段模板。
                      </Typography>
                    </Stack>
                  </TableCell>
                </TableRow>
              ) : null}
              {items.map((item) => {
                const isSelected = item.id === selectedId;

                return (
                  <TableRow
                    key={item.id}
                    hover
                    selected={isSelected}
                    onClick={() => onSelectTemplate(item.id)}
                    sx={{ cursor: 'pointer' }}
                  >
                    <TableCell>
                      <Stack spacing={0.5}>
                        <Typography variant="subtitle2">{item.name}</Typography>
                        <Typography variant="caption" color="text.secondary">
                          由 {item.updatedBy} 更新
                        </Typography>
                      </Stack>
                    </TableCell>
                    <TableCell>
                      <Chip size="small" label={item.version} />
                    </TableCell>
                    <TableCell>
                      <StatusChip status={item.enabled ? 'active' : 'disabled'} />
                    </TableCell>
                    <TableCell sx={{ maxWidth: 320 }}>
                      <Typography variant="body2" color="text.secondary" noWrap>
                        {item.summary}
                      </Typography>
                    </TableCell>
                    <TableCell>{item.stages.length}</TableCell>
                    <TableCell>{item.updatedAt}</TableCell>
                    <TableCell align="right">
                      <Button
                        size="small"
                        variant="text"
                        startIcon={<MoreHorizRoundedIcon />}
                        onClick={(event) => {
                          event.stopPropagation();
                          handleOpenMenu(event, item.id);
                        }}
                      >
                        操作
                      </Button>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </TableContainer>
      </Stack>

      <Menu anchorEl={menuAnchorEl} open={menuOpen} onClose={handleCloseMenu}>
        <MenuItem onClick={handleEdit}>
          <EditRoundedIcon fontSize="small" style={{ marginRight: 8 }} />
          编辑模板
        </MenuItem>
        <MenuItem onClick={handleToggle}>
          <PowerSettingsNewRoundedIcon fontSize="small" style={{ marginRight: 8 }} />
          切换启用状态
        </MenuItem>
        <MenuItem onClick={handleDuplicate}>
          <FileCopyRoundedIcon fontSize="small" style={{ marginRight: 8 }} />
          复制模板
        </MenuItem>
        <MenuItem onClick={handleDelete}>
          <DeleteOutlineRoundedIcon fontSize="small" style={{ marginRight: 8 }} />
          删除模板
        </MenuItem>
      </Menu>
    </Paper>
  );
}
