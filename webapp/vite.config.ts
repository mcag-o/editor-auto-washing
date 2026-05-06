import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  base: '/',
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/test/setup.ts',
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
  },
  build: {
    outDir: '../web/dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        assetFileNames: 'ui/assets/[name]-[hash][extname]',
        chunkFileNames: 'ui/assets/[name]-[hash].js',
        entryFileNames: 'ui/assets/[name]-[hash].js',
      },
    },
  },
});
