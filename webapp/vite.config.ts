import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  base: '/',
  plugins: [react()],
  build: {
    outDir: 'dist',
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
