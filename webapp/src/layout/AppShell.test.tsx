import { CssBaseline, ThemeProvider } from '@mui/material';
import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import AppShell from './AppShell';
import theme from '../theme/theme';

function installMatchMedia(width: number) {
  Object.defineProperty(window, 'innerWidth', {
    configurable: true,
    writable: true,
    value: width,
  });

  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: (query: string) => {
      const minWidth = Number(/min-width:\s*(\d+(?:\.\d+)?)px/.exec(query)?.[1] ?? Number.NEGATIVE_INFINITY);
      const maxWidth = Number(/max-width:\s*(\d+(?:\.\d+)?)px/.exec(query)?.[1] ?? Number.POSITIVE_INFINITY);
      const matches = width >= minWidth && width <= maxWidth;

      return {
        matches,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      };
    },
  });
}

function renderAppShell() {
  return render(
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <AppShell currentPage="workflows" onNavigate={vi.fn()}>
        <div>shell content</div>
      </AppShell>
    </ThemeProvider>,
  );
}

describe('AppShell responsive layout', () => {
  beforeEach(() => {
    installMatchMedia(1440);
  });

  it('keeps mobile navigation and primary action reachable on narrow screens', () => {
    installMatchMedia(480);

    renderAppShell();

    expect(screen.getByRole('button', { name: 'open navigation' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '导入' })).toBeInTheDocument();
  });
});
