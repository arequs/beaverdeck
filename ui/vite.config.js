import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';

const uiDir = path.dirname(fileURLToPath(import.meta.url));
const distDir = path.resolve(uiDir, '../cmd/server/web/dist');

export default defineConfig({
  base: './',
  publicDir: path.resolve(uiDir, 'public'),
  build: {
    outDir: distDir,
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) {
            return;
          }
          if (id.includes('/@xterm/') || id.includes('/xterm')) {
            return 'xterm';
          }
          if (id.includes('/react/') || id.includes('/react-dom/')) {
            return 'react-vendor';
          }
          return 'vendor';
        }
      }
    }
  }
});
