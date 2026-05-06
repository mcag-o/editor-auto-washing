import { alpha, createTheme } from '@mui/material/styles';

declare module '@mui/material/styles' {
  interface Palette {
    status: {
      pending: string;
      active: string;
      disabled: string;
      completed: string;
      failed: string;
    };
  }

  interface PaletteOptions {
    status?: {
      pending: string;
      active: string;
      disabled: string;
      completed: string;
      failed: string;
    };
  }
}

const baseTheme = createTheme({
  shape: {
    borderRadius: 16,
  },
  typography: {
    fontFamily: '"Noto Sans SC", "PingFang SC", "Microsoft YaHei", sans-serif',
    h1: {
      fontSize: '2.4rem',
      fontWeight: 700,
      lineHeight: 1.15,
    },
    h2: {
      fontSize: '1.85rem',
      fontWeight: 700,
      lineHeight: 1.2,
    },
    h3: {
      fontSize: '1.5rem',
      fontWeight: 700,
      lineHeight: 1.25,
    },
    h4: {
      fontSize: '1.2rem',
      fontWeight: 700,
      lineHeight: 1.3,
    },
    subtitle1: {
      fontSize: '1rem',
      fontWeight: 600,
    },
    body2: {
      lineHeight: 1.6,
    },
    button: {
      fontWeight: 600,
      textTransform: 'none',
    },
  },
  palette: {
    mode: 'light',
    primary: {
      main: '#0f62fe',
      dark: '#0747b4',
      light: '#5b8dff',
    },
    secondary: {
      main: '#5b3df5',
    },
    success: {
      main: '#16804a',
    },
    warning: {
      main: '#b76e00',
    },
    error: {
      main: '#c62828',
    },
    background: {
      default: '#eef3fb',
      paper: '#ffffff',
    },
    divider: alpha('#15304f', 0.12),
    text: {
      primary: '#142033',
      secondary: '#55637a',
    },
    status: {
      pending: '#9c6b00',
      active: '#0f62fe',
      disabled: '#6b7280',
      completed: '#16804a',
      failed: '#c62828',
    },
  },
});

const theme = createTheme(baseTheme, {
  components: {
    MuiAppBar: {
      styleOverrides: {
        root: {
          backgroundImage:
            'linear-gradient(135deg, rgba(10, 28, 54, 0.96) 0%, rgba(20, 57, 104, 0.94) 100%)',
          boxShadow: 'none',
          borderBottom: `1px solid ${alpha('#ffffff', 0.08)}`,
          backdropFilter: 'blur(18px)',
        },
      },
    },
    MuiCard: {
      styleOverrides: {
        root: {
          borderRadius: 24,
          border: `1px solid ${alpha('#15304f', 0.1)}`,
          boxShadow: '0 24px 60px rgba(20, 32, 51, 0.08)',
        },
      },
    },
    MuiPaper: {
      styleOverrides: {
        rounded: {
          borderRadius: 24,
        },
      },
    },
    MuiButton: {
      defaultProps: {
        disableElevation: true,
      },
      styleOverrides: {
        root: {
          borderRadius: 999,
          paddingInline: 18,
        },
      },
    },
    MuiChip: {
      styleOverrides: {
        root: {
          fontWeight: 600,
          borderRadius: 999,
        },
      },
    },
    MuiDrawer: {
      styleOverrides: {
        paper: {
          backgroundImage:
            'linear-gradient(180deg, rgba(9, 20, 36, 0.98) 0%, rgba(13, 29, 49, 0.98) 100%)',
          color: '#e6edf8',
          borderRight: 'none',
        },
      },
    },
  },
});

export default theme;
