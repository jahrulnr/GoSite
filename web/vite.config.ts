import { defineConfig } from 'vite';
import preact from '@preact/preset-vite';

// GoSite frontend build.
// - Outputs to ../internal/delivery/http/frontend/dist so Go can embed it.
// - Dev server proxies /api and /health to the local Go panel (https, self-signed).
export default defineConfig(({ command }) => ({
  // When served behind nginx at /panel/, build assets with absolute base.
  base: process.env.VITE_BASE_PATH || '/',
  plugins: [preact()],
  build: {
    outDir: '../internal/delivery/http/frontend/dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 700,
  },
  server: {
    port: 5173,
    proxy:
      command === 'serve'
        ? {
            '/api': {
              target: process.env.VITE_API_TARGET || 'https://localhost:8080',
              changeOrigin: true,
              secure: false,
            },
            '/health': {
              target: process.env.VITE_API_TARGET || 'https://localhost:8080',
              changeOrigin: true,
              secure: false,
            },
          }
        : undefined,
  },
}));
