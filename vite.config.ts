import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';

export default defineConfig(({ command }) => ({
  root: fileURLToPath(new URL('./frontend', import.meta.url)),
  base: command === 'build' ? '/static/app/' : '/',
  plugins: [react()],
  build: {
    outDir: fileURLToPath(new URL('./static/app', import.meta.url)),
    emptyOutDir: true,
  },
  server: {
    host: '127.0.0.1',
    proxy: {
      '/api': 'http://127.0.0.1:8080',
      '/static': 'http://127.0.0.1:8080',
      '/admin': 'http://127.0.0.1:8080',
    },
  },
}));
