import type { PropsWithChildren } from 'react';
import { useState } from 'react';
import AppsRoundedIcon from '@mui/icons-material/AppsRounded';
import AutoAwesomeRoundedIcon from '@mui/icons-material/AutoAwesomeRounded';
import DescriptionRoundedIcon from '@mui/icons-material/DescriptionRounded';
import HistoryRoundedIcon from '@mui/icons-material/HistoryRounded';
import MenuRoundedIcon from '@mui/icons-material/MenuRounded';
import SettingsRoundedIcon from '@mui/icons-material/SettingsRounded';
import AppBar from '@mui/material/AppBar';
import Avatar from '@mui/material/Avatar';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Container from '@mui/material/Container';
import Divider from '@mui/material/Divider';
import Drawer from '@mui/material/Drawer';
import IconButton from '@mui/material/IconButton';
import List from '@mui/material/List';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import Stack from '@mui/material/Stack';
import Toolbar from '@mui/material/Toolbar';
import Typography from '@mui/material/Typography';
import { alpha } from '@mui/material/styles';
import useMediaQuery from '@mui/material/useMediaQuery';
import { useTheme } from '@mui/material/styles';

const drawerWidth = 248;

export type AppPage = 'overview' | 'intake' | 'articles' | 'control' | 'workflows' | 'templates' | 'config' | 'audit';

const navigationItems = [
  { key: 'overview' as const, label: '控制台总览', icon: <AppsRoundedIcon fontSize="small" /> },
  { key: 'intake' as const, label: '文章导入', icon: <AutoAwesomeRoundedIcon fontSize="small" /> },
  { key: 'articles' as const, label: '文章队列', icon: <DescriptionRoundedIcon fontSize="small" /> },
  { key: 'control' as const, label: '流程控制', icon: <AutoAwesomeRoundedIcon fontSize="small" /> },
  { key: 'workflows' as const, label: '工作流编辑', icon: <AutoAwesomeRoundedIcon fontSize="small" /> },
  { key: 'templates' as const, label: '模板管理', icon: <AutoAwesomeRoundedIcon fontSize="small" /> },
  { key: 'audit', label: '操作审计', icon: <HistoryRoundedIcon fontSize="small" /> },
  { key: 'config' as const, label: '配置管理', icon: <SettingsRoundedIcon fontSize="small" /> },
];

type AppShellProps = PropsWithChildren<{
  currentPage: AppPage;
  onNavigate: (page: AppPage) => void;
}>;

export default function AppShell({ children, currentPage, onNavigate }: AppShellProps) {
  const theme = useTheme();
  const isDesktop = useMediaQuery(theme.breakpoints.up('md'));
  const [mobileNavOpen, setMobileNavOpen] = useState(false);

  const handleOpenMobileNav = () => {
    setMobileNavOpen(true);
  };

  const handleCloseMobileNav = () => {
    setMobileNavOpen(false);
  };

  const drawerContent = (
    <>
      <Toolbar sx={{ minHeight: 72 }} />
      <Box sx={{ px: 2, py: 2.5 }}>
        <Stack
          spacing={1}
          sx={{
            p: 2,
            borderRadius: 4,
            background: 'linear-gradient(135deg, rgba(91, 61, 245, 0.26), rgba(15, 98, 254, 0.12))',
            border: `1px solid ${alpha('#ffffff', 0.08)}`,
          }}
        >
          <Typography variant="overline" sx={{ color: alpha('#ffffff', 0.64) }}>
            运营工作台
          </Typography>
          <Typography variant="h4" sx={{ color: '#ffffff' }}>
            Content Hub
          </Typography>
          <Typography variant="body2" sx={{ color: alpha('#ffffff', 0.7) }}>
            统一处理文章导入、流程控制、模板配置与审计查看。
          </Typography>
        </Stack>
      </Box>
      <Divider sx={{ borderColor: alpha('#ffffff', 0.08) }} />
      <List sx={{ px: 1.5, py: 2 }}>
        {navigationItems.map((item) => (
          <ListItemButton
            key={item.key}
            selected={item.key === currentPage}
            onClick={() => {
              if (
                item.key === 'overview' ||
                item.key === 'intake' ||
                item.key === 'articles' ||
                item.key === 'control' ||
                item.key === 'workflows' ||
                item.key === 'templates' ||
                item.key === 'audit' ||
                item.key === 'config'
              ) {
                onNavigate(item.key);
              }
              handleCloseMobileNav();
            }}
            sx={{
              mb: 0.75,
              borderRadius: 3,
              color: item.key === currentPage ? '#ffffff' : alpha('#ffffff', 0.74),
              bgcolor: item.key === currentPage ? alpha('#ffffff', 0.1) : 'transparent',
              '&.Mui-selected': {
                bgcolor: alpha('#ffffff', 0.1),
              },
              '&.Mui-selected:hover, &:hover': {
                bgcolor: alpha('#ffffff', 0.14),
              },
            }}
          >
            <ListItemIcon sx={{ color: 'inherit', minWidth: 36 }}>{item.icon}</ListItemIcon>
            <ListItemText primary={item.label} />
          </ListItemButton>
        ))}
      </List>
    </>
  );

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: 'background.default' }}>
      <AppBar position="fixed">
        <Toolbar sx={{ minHeight: 72, px: { xs: 2, md: 3 } }}>
          <Stack direction="row" spacing={2} alignItems="center" sx={{ flexGrow: 1 }}>
            {!isDesktop ? (
              <IconButton color="inherit" edge="start" onClick={handleOpenMobileNav} sx={{ ml: -1 }}>
                <MenuRoundedIcon />
              </IconButton>
            ) : null}
            <Avatar
              variant="rounded"
              sx={{
                width: 44,
                height: 44,
                bgcolor: alpha('#ffffff', 0.12),
                color: '#ffffff',
              }}
            >
              <AutoAwesomeRoundedIcon />
            </Avatar>
            <Box>
              <Typography variant="subtitle1" sx={{ color: '#ffffff' }}>
                Content Hub 控制台
              </Typography>
              <Typography variant="body2" sx={{ color: alpha('#ffffff', 0.72) }}>
                自动改写、草稿生成与运营管理
              </Typography>
            </Box>
          </Stack>
          <Stack direction="row" spacing={1.25} alignItems="center">
              <Button variant="contained" color="secondary" onClick={() => onNavigate('intake')}>
                新建导入
              </Button>
            </Stack>
          </Toolbar>
      </AppBar>

      <Drawer
        variant={isDesktop ? 'permanent' : 'temporary'}
        open={isDesktop ? true : mobileNavOpen}
        onClose={handleCloseMobileNav}
        ModalProps={{ keepMounted: true }}
        sx={{
          width: drawerWidth,
          flexShrink: 0,
          '& .MuiDrawer-paper': {
            width: drawerWidth,
            boxSizing: 'border-box',
            borderRight: `1px solid ${alpha('#ffffff', 0.08)}`,
          },
        }}
      >
        {drawerContent}
      </Drawer>

      <Box
        component="main"
        sx={{
          ml: { md: `${drawerWidth}px` },
          width: { md: `calc(100% - ${drawerWidth}px)` },
          pt: '88px',
          pb: 5,
        }}
      >
        <Container maxWidth="xl" sx={{ px: { xs: 2, md: 3 } }}>
          {children}
        </Container>
      </Box>
    </Box>
  );
}
